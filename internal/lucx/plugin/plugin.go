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
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

const maxPackageSize = 200 << 20

var pluginID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// networkNone requests network isolation for the plugin process (Stage 2).
// It is the only enforceable permission token in apiVersion 1.
const networkNone = "none"

func validateID(id string) error {
	if !pluginID.MatchString(id) {
		return errors.New("invalid plugin ID")
	}
	return nil
}

type HealthCheck struct {
	Type            string `json:"type"` // "http" | "tcp"
	Target          string `json:"target"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
}

type Resources struct {
	MaxMemoryMB   int `json:"maxMemoryMB,omitempty"`
	MaxCPUPercent int `json:"maxCPUPercent,omitempty"`
}

// Equal compares resources blocks for the preview-vs-installed check.
func (r *Resources) Equal(b *Resources) bool {
	if r == nil || b == nil {
		return r == b
	}
	return *r == *b
}

// Equal compares health blocks for the preview-vs-installed check.
func (h *HealthCheck) Equal(b *HealthCheck) bool {
	if h == nil || b == nil {
		return h == b
	}
	return *h == *b
}

type EnforcementInfo struct {
	NetworkIsolated bool `json:"networkIsolated"`
	MemoryLimitMB   int  `json:"memoryLimitMB,omitempty"`
	Enforceable     bool `json:"enforceable"`
	CPULimitPercent int  `json:"cpuLimitPercent,omitempty"`
	CPUEnforced     bool `json:"cpuEnforced,omitempty"`
}

type HealthStatus struct {
	Status    string `json:"status"` // "pass" | "fail" | "unknown"
	Detail    string `json:"detail,omitempty"`
	CheckedAt int64  `json:"checkedAt,omitempty"`
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
	Resources     *Resources          `json:"resources,omitempty"`
	Health        *HealthCheck        `json:"health,omitempty"`
	Events        []string            `json:"events,omitempty"`
}

type Status struct {
	Manifest    Manifest        `json:"manifest"`
	Installed   bool            `json:"installed"`
	Running     bool            `json:"running"`
	PID         int             `json:"pid,omitempty"`
	Runner      string          `json:"runner,omitempty"`
	BinaryPath  string          `json:"binaryPath"`
	DataPath    string          `json:"dataPath"`
	LogPath     string          `json:"logPath"`
	LastError   string          `json:"lastError,omitempty"`
	Enforcement EnforcementInfo `json:"enforcement"`
	Events      *EventsStatus   `json:"events,omitempty"`
	Health      *HealthStatus   `json:"health,omitempty"`
}

type managed struct {
	mu              sync.Mutex
	cmd             *exec.Cmd
	done            chan struct{}
	log             *os.File
	mode            string
	unit            string
	lastError       string
	healthStatus    string
	healthDetail    string
	healthCheckedAt int64
}

var manager = struct {
	sync.Mutex
	procs map[string]*managed
}{procs: map[string]*managed{}}

// binFolder is the test seam for the plugin storage root.
var binFolder = config.GetBinFolderPath

func root() string                  { return filepath.Join(binFolder(), "plugins") }
func dir(id string) string          { return filepath.Join(root(), id) }
func manifestPath(id string) string { return filepath.Join(dir(id), "manifest.json") }
func dataPath(id string) string     { return filepath.Join(dir(id), "data") }
func logPath(id string) string      { return filepath.Join(dir(id), "plugin.log") }

func denyNetwork(m Manifest) bool {
	for _, rule := range m.Permissions["network"] {
		if rule == networkNone {
			return true
		}
	}
	return false
}

func memLimit(m Manifest) int {
	if m.Resources == nil {
		return 0
	}
	return m.Resources.MaxMemoryMB
}

func cpuLimit(m Manifest) int {
	if m.Resources == nil {
		return 0
	}
	return m.Resources.MaxCPUPercent
}

// needsEnforcement reports whether the manifest asks for enforcement the
// platform must apply at process start.
func needsEnforcement(m Manifest) bool {
	return denyNetwork(m) || memLimit(m) > 0 || cpuRequiresUnit(m)
}

func validatePermissions(p map[string][]string) error {
	for kind, rules := range p {
		if kind != "network" && kind != "panelApi" && kind != "bot" && kind != "sub" {
			return fmt.Errorf("unknown plugin permission %q (allowed: network, panelApi, bot, sub)", kind)
		}
		if len(rules) > 16 {
			return fmt.Errorf("plugin permission %q has too many entries", kind)
		}
		for _, rule := range rules {
			if rule == "" || len(rule) > 128 {
				return fmt.Errorf("plugin permission %q has an invalid entry", kind)
			}
		}
	}
	if err := validatePanelPermissions(p["panelApi"]); err != nil {
		return err
	}
	if err := validateBotPermissions(p["bot"]); err != nil {
		return err
	}
	if err := validateSubPermissions(p["sub"]); err != nil {
		return err
	}
	for _, rule := range p["network"] {
		if rule != networkNone {
			return fmt.Errorf("unsupported network rule %q in apiVersion 1; only %q is enforced", rule, networkNone)
		}
	}
	return nil
}

func validateTCP(hostPort string) error {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return errors.New("plugin health tcp target must be host:port")
	}
	if host == "" {
		return errors.New("plugin health tcp target requires a host")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return errors.New("plugin health tcp target requires a valid port")
	}
	return nil
}

func validateHealth(h *HealthCheck, isolated bool) error {
	if h == nil {
		return nil
	}
	if h.Type != "http" && h.Type != "tcp" {
		return errors.New("plugin health type must be http or tcp")
	}
	if h.Target == "" {
		return errors.New("plugin health requires a target")
	}
	if h.Type == "http" {
		u, err := url.Parse(h.Target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("plugin health http target must be an http(s) URL")
		}
	} else if err := validateTCP(h.Target); err != nil {
		return err
	}
	if h.IntervalSeconds == 0 {
		h.IntervalSeconds = 30
	}
	if h.IntervalSeconds < 5 || h.IntervalSeconds > 3600 {
		return errors.New("plugin health intervalSeconds must be between 5 and 3600")
	}
	if h.TimeoutSeconds == 0 {
		h.TimeoutSeconds = 5
	}
	if h.TimeoutSeconds < 1 || h.TimeoutSeconds > 60 || h.TimeoutSeconds > h.IntervalSeconds {
		return errors.New("plugin health timeoutSeconds must be 1..60 and not exceed intervalSeconds")
	}
	if isolated {
		return errors.New("health probes cannot reach a network-isolated plugin")
	}
	return nil
}

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
	if err := validatePermissions(m.Permissions); err != nil {
		return err
	}
	if m.Resources != nil && m.Resources.MaxMemoryMB != 0 && (m.Resources.MaxMemoryMB < 16 || m.Resources.MaxMemoryMB > 131072) {
		return errors.New("plugin resources.maxMemoryMB must be between 16 and 131072, or 0/absent")
	}
	if m.Resources != nil && m.Resources.MaxCPUPercent != 0 && (m.Resources.MaxCPUPercent < 1 || m.Resources.MaxCPUPercent > 100) {
		return errors.New("plugin resources.maxCPUPercent must be between 1 and 100, or 0/absent")
	}
	if err := validateEvents(m.Events); err != nil {
		return err
	}
	if len(m.Permissions["sub"]) > 0 && denyNetwork(m) {
		// LUCX-HOOK: a sub/after socket plugin cannot be kernel-isolated —
		// a fresh netns would strand the panel's socket POSTs.
		return errors.New("a plugin with the sub permission cannot also declare network:[\"none\"] - the panel could not reach its socket")
	}
	if panelDeniedByNetwork(m) {
		return errors.New("a plugin with panelApi or bot permissions cannot also declare network:[\"none\"] - it would not be able to reach the panel")
	}
	return validateHealth(m.Health, denyNetwork(m))
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
		if rel == "data" || strings.HasPrefix(rel, "data"+string(os.PathSeparator)) {
			continue
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
	if info.Mode().IsDir() {
		return m, errors.New("plugin entrypoint is a directory")
	}
	// NTFS carries no exec bit: the 0700 guarantee is Unix-only, so the
	// entrypoint check applies where the bits are real.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
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
	s.Enforcement = EnforcementInfo{NetworkIsolated: denyNetwork(m), MemoryLimitMB: memLimit(m), Enforceable: enforcementSupported}
	if cpu := cpuLimit(m); cpu > 0 {
		s.Enforcement.CPULimitPercent = cpu
	}
	if m.Health != nil {
		health := &HealthStatus{Status: "unknown"}
		if p != nil {
			p.mu.Lock()
			if p.healthStatus != "" {
				health.Status = p.healthStatus
				health.Detail = p.healthDetail
				health.CheckedAt = p.healthCheckedAt
			}
			p.mu.Unlock()
		}
		s.Health = health
	}
	if p == nil && runtime.GOOS == "linux" && lookPathUnit() {
		st := unitState(unitName(id))
		if st.running {
			s.Running, s.Runner = true, runnerUnit
			if st.pid > 0 {
				s.PID = st.pid
			}
			if st.detail != "" {
				s.LastError = st.detail
			}
			s.Enforcement.CPUEnforced = cpuLimit(m) > 0
		}
	}
	if p != nil {
		p.mu.Lock()
		s.Runner = p.mode
		switch p.mode {
		case runnerUnit:
			st := unitState(p.unit)
			s.Running = st.running
			if st.running && st.pid > 0 {
				s.PID = st.pid
				s.Enforcement.CPUEnforced = cpuLimit(m) > 0
			}
			if st.detail != "" {
				s.LastError = st.detail
			}
		default:
			s.Running = p.cmd != nil && p.cmd.Process != nil
			if s.Running {
				s.PID = p.cmd.Process.Pid
			}
			s.LastError = p.lastError
		}
		p.mu.Unlock()
	}
	if len(m.Events) > 0 {
		if s.Running {
			ensureEventSubscriber(m, s.Runner)
			if eventsStatus(id) == nil {
				registerSocket(id, socketPath(m, s.Runner))
			}
		}
		s.Events = eventsStatus(id)
	}
	return s, nil
}

// startFailure turns raw start errors into operator-facing messages; the
// EPERM mapping covers sandboxed hosts (hardened systemd units, containers)
// where the netns clone for permissions.network ["none"] is denied.
func startFailure(err error, m Manifest) string {
	if needsEnforcement(m) && errors.Is(err, syscall.EPERM) {
		return "network isolation is not permitted in this host's sandbox (the panel's systemd unit blocks it); plugin not started (fail closed)"
	}
	return err.Error()
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
	if p.mode == runnerUnit && unitState(unitName(id)).running {
		return nil
	}
	if p.cmd != nil {
		return nil
	}
	if binAbs, err := filepath.Abs(bin); err == nil {
		bin = binAbs
	}
	if len(m.Permissions["panelApi"]) > 0 {
		if _, err := GrantPanelToken(id); err != nil {
			return err
		}
	}
	mode := selectRunner(m)
	if cpuRequiresUnit(m) && mode != runnerUnit {
		p.lastError = "plugin CPU capping requires the systemd unit runner"
		return errors.New(p.lastError)
	}
	if mode == runnerUnit {
		if err := startUnit(p, m, bin); err != nil {
			p.lastError = err.Error()
		} else {
			if needsSocket(m) {
				registerSocket(id, socketPath(m, runnerUnit))
			}
			if len(m.Events) > 0 {
				ensureEventSubscriber(m, runnerUnit)
			}
			p.lastError = ""
			return nil
		}
	}
	sock := socketPath(m, runnerProcess)
	if err := startChild(p, m, bin, sock); err != nil {
		p.lastError = err.Error()
		return errors.New(p.lastError)
	}
	/* The unit runner refused: the process runner applies its own enforcement
	(netns clone, RLIMIT_AS) and refuses fail-closed on hosts where that is
	impossible, so an enforcement manifest never starts unsandboxed silently. */
	if needsSocket(m) {
		registerSocket(id, sock)
	}
	if len(m.Events) > 0 {
		ensureEventSubscriber(m, runnerProcess)
	}
	p.mode = runnerProcess
	p.lastError = ""
	return nil
}

/*
	startChild is the classic Stage 1-2 supervised child process: panel-owned

	stdio, health goroutine and exit watcher included. sock is the unix socket

path a plugin listing `events` or granted the `sub` seam must bind (empty
otherwise).
*/
func startChild(p *managed, m Manifest, bin, sock string) error {
	log, err := os.OpenFile(logPath(m.ID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	// cmd.Dir makes the kernel resolve a relative argv0 against the plugin
	// directory (Go chdirs the child before execve), so the exec path must be
	// absolute or it doubles into dir/bin/plugins/<id>/entrypoint.
	cmd := exec.CommandContext(context.Background(), bin)
	if err := prepareCmd(cmd, m); err != nil {
		log.Close()
		return err
	}
	cmd.Dir = dir(m.ID)
	cmd.Env = append(os.Environ(), "LUCX_PLUGIN_ID="+m.ID, "LUCX_PLUGIN_VERSION="+m.Version, "LUCX_PLUGIN_DATA="+dataPath(m.ID))
	if sock != "" && needsSocket(m) {
		cmd.Env = append(cmd.Env, "LUCX_PLUGIN_SOCKET="+sock)
	}
	cmd.Env = append(cmd.Env, pluginEnv(m)...)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		log.Close()
		return errors.New(startFailure(err, m))
	}
	done := make(chan struct{})
	p.cmd = cmd
	p.done = done
	p.log = log
	if m.Health != nil {
		go runHealth(p, done, *m.Health)
	}
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
		p.mode = ""
		p.mu.Unlock()
		removeEventSubscriber(m.ID)
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
		/* The panel may have restarted while the unit kept running; stop by
		name when it does. A missing unit (already collected) makes stop fail
		harmlessly, so only the still-running case is stopped. */
		if runtime.GOOS == "linux" && lookPathUnit() && unitState(unitName(id)).running {
			_ = stopUnit(unitName(id))
			waitUnit(unitName(id))
		}
		return nil
	}
	p.mu.Lock()
	if p.mode == runnerUnit {
		name := p.unit
		if unitState(name).running {
			_ = stopUnit(name)
			waitUnit(name)
		}
		p.mode = ""
		p.unit = ""
		p.mu.Unlock()
		removeEventSubscriber(id)
		return nil
	}
	if p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		removeEventSubscriber(id)
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
	removeEventSubscriber(id)
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
	forgetPanelToken(id)
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
