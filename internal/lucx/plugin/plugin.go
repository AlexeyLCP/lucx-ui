// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

const maxPackageSize = 200 << 20

var pluginID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func validateID(id string) error {
	if !pluginID.MatchString(id) {
		return errors.New("invalid plugin ID")
	}
	return nil
}

type Manifest struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Version       string              `json:"version"`
	APIVersion    int                 `json:"apiVersion"`
	Type          string              `json:"type"`
	Entrypoint    string              `json:"entrypoint"`
	Architectures []string            `json:"architectures"`
	Permissions   map[string][]string `json:"permissions,omitempty"`
}

type Status struct {
	Manifest   Manifest `json:"manifest"`
	Installed  bool     `json:"installed"`
	Running    bool     `json:"running"`
	PID        int      `json:"pid,omitempty"`
	BinaryPath string   `json:"binaryPath"`
	DataPath   string   `json:"dataPath"`
	LogPath    string   `json:"logPath"`
	LastError  string   `json:"lastError,omitempty"`
}

type managed struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
	log       *os.File
	lastError string
}

var manager = struct {
	sync.Mutex
	procs map[string]*managed
}{procs: map[string]*managed{}}

func root() string                  { return filepath.Join(config.GetBinFolderPath(), "plugins") }
func dir(id string) string          { return filepath.Join(root(), id) }
func manifestPath(id string) string { return filepath.Join(dir(id), "manifest.json") }
func dataPath(id string) string     { return filepath.Join(dir(id), "data") }
func logPath(id string) string      { return filepath.Join(dir(id), "plugin.log") }

func validateManifest(m Manifest) error {
	if err := validateID(m.ID); err != nil {
		return err
	}
	if m.ID == "" || m.Name == "" || m.Version == "" || m.APIVersion != 1 || m.Type != "sidecar" || m.Entrypoint == "" {
		return errors.New("plugin manifest requires id, name, version, apiVersion=1, type=sidecar and entrypoint")
	}
	if filepath.IsAbs(m.Entrypoint) || strings.Contains(filepath.ToSlash(m.Entrypoint), "..") {
		return errors.New("plugin entrypoint must stay inside the plugin directory")
	}
	okArch := false
	for _, arch := range m.Architectures {
		if arch == runtime.GOARCH {
			okArch = true
		}
	}
	if len(m.Architectures) > 0 && !okArch {
		return fmt.Errorf("plugin does not support %s", runtime.GOARCH)
	}
	return nil
}

func Load(id string) (Manifest, error) {
	var m Manifest
	if err := validateID(id); err != nil {
		return m, err
	}
	b, err := os.ReadFile(manifestPath(id))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.ID != id {
		return m, errors.New("plugin manifest ID does not match directory")
	}
	return m, validateManifest(m)
}

func InstallPackage(r io.Reader, size int64) (Manifest, error) {
	if size <= 0 || size > maxPackageSize {
		return Manifest{}, errors.New("plugin package must be between 1 byte and 200 MB")
	}
	tmp, err := os.CreateTemp("", "lucx-plugin-*.zip")
	if err != nil {
		return Manifest{}, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = io.CopyN(tmp, r, size+1); err != nil && !errors.Is(err, io.EOF) {
		tmp.Close()
		return Manifest{}, err
	}
	if err := tmp.Close(); err != nil {
		return Manifest{}, err
	}
	z, err := zip.OpenReader(name)
	if err != nil {
		return Manifest{}, errors.New("plugin package must be a zip archive")
	}
	defer z.Close()
	var m Manifest
	for _, f := range z.File {
		if f.Name == "manifest.json" {
			rc, e := f.Open()
			if e != nil {
				return m, e
			}
			e = json.NewDecoder(rc).Decode(&m)
			rc.Close()
			if e != nil {
				return m, e
			}
			break
		}
	}
	if err := validateManifest(m); err != nil {
		return m, err
	}
	if err := os.MkdirAll(dir(m.ID), 0o700); err != nil {
		return m, err
	}
	for _, f := range z.File {
		target := filepath.Join(dir(m.ID), filepath.FromSlash(f.Name))
		rel, _ := filepath.Rel(dir(m.ID), target)
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
			return m, errors.New("plugin archive contains an unsafe path")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return m, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return m, err
		}
		src, e := f.Open()
		if e != nil {
			return m, e
		}
		dst, e := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
		if e == nil {
			_, e = io.Copy(dst, src)
			dst.Close()
		}
		src.Close()
		if e != nil {
			return m, e
		}
	}
	info, err := os.Stat(filepath.Join(dir(m.ID), m.Entrypoint))
	if err != nil || info.IsDir() {
		return m, errors.New("plugin entrypoint is missing")
	}
	if info.Mode().Perm()&0o111 == 0 {
		return m, errors.New("plugin entrypoint is not executable")
	}
	return m, nil
}

func List() ([]Status, error) {
	entries, err := os.ReadDir(root())
	if os.IsNotExist(err) {
		return []Status{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if s, err := Get(e.Name()); err == nil {
			out = append(out, s)
		}
	}
	return out, nil
}

func Get(id string) (Status, error) {
	m, err := Load(id)
	if err != nil {
		return Status{}, err
	}
	manager.Lock()
	p := manager.procs[id]
	manager.Unlock()
	s := Status{Manifest: m, Installed: true, BinaryPath: filepath.Join(dir(id), m.Entrypoint), DataPath: dataPath(id), LogPath: logPath(id)}
	if p != nil {
		p.mu.Lock()
		s.Running = p.cmd != nil && p.cmd.Process != nil
		if s.Running {
			s.PID = p.cmd.Process.Pid
		}
		s.LastError = p.lastError
		p.mu.Unlock()
	}
	return s, nil
}

func Start(id string) error {
	m, err := Load(id)
	if err != nil {
		return err
	}
	bin := filepath.Join(dir(id), m.Entrypoint)
	if _, err := os.Stat(bin); err != nil {
		return err
	}
	if err := os.MkdirAll(dataPath(id), 0o700); err != nil {
		return err
	}
	manager.Lock()
	p := manager.procs[id]
	if p == nil {
		p = &managed{}
		manager.procs[id] = p
	}
	manager.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil {
		return nil
	}
	log, err := os.OpenFile(logPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), bin)
	cmd.Dir = dir(id)
	cmd.Env = append(os.Environ(), "LUCX_PLUGIN_ID="+m.ID, "LUCX_PLUGIN_VERSION="+m.Version, "LUCX_PLUGIN_DATA="+dataPath(id))
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		log.Close()
		p.lastError = err.Error()
		return err
	}
	done := make(chan struct{})
	p.cmd = cmd
	p.done = done
	p.log = log
	go func() {
		err := cmd.Wait()
		log.Close()
		p.mu.Lock()
		if err != nil {
			p.lastError = err.Error()
		}
		if p.cmd == cmd {
			p.cmd = nil
			p.done = nil
		}
		p.mu.Unlock()
		close(done)
	}()
	return nil
}

func Stop(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	manager.Lock()
	p := manager.procs[id]
	manager.Unlock()
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		return nil
	}
	cmd, done, log := p.cmd, p.done, p.log
	_ = cmd.Process.Signal(os.Interrupt)
	p.mu.Unlock()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	if log != nil {
		_ = log.Close()
	}
	return nil
}

func Restart(id string) error {
	if err := Stop(id); err != nil {
		return err
	}
	return Start(id)
}

func Remove(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if err := Stop(id); err != nil {
		return err
	}
	return os.RemoveAll(dir(id))
}

func Logs(id string, lines int) ([]string, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	b, err := os.Open(logPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer b.Close()
	var out []string
	s := bufio.NewScanner(b)
	for s.Scan() {
		out = append(out, s.Text())
		if lines > 0 && len(out) > lines {
			out = out[1:]
		}
	}
	return out, s.Err()
}

func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), err
}
