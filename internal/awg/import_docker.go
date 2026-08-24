// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

var dockerConfNames = []string{"awg0.conf", "wg0.conf", "awg.conf"}

func scanLiveDocker(paths DiscoverPaths) []ImportCandidate {
	list := paths.DockerList
	read := paths.DockerRead
	if list == nil {
		list = defaultListDockerContainers
	}
	if read == nil {
		read = defaultReadDockerFile
	}
	var out []ImportCandidate
	for _, name := range list() {
		sub, dirs, ok := dockerProtoOf(name)
		if !ok {
			continue
		}
		if c, ok := readDockerCandidate(name, sub, dirs, read); ok {
			out = append(out, c)
		}
	}
	return out
}

func dockerProtoOf(name string) (string, []string, bool) {
	switch {
	case name == "amnezia-wireguard" || strings.HasPrefix(name, "amnezia-wireguard-"):
		return "wireguard", []string{"/opt/amnezia/wireguard"}, true
	case name == "amnezia-awg3" || strings.HasPrefix(name, "amnezia-awg3-"):
		return "awg3", []string{"/opt/amnezia/awg3", "/opt/amnezia/awg"}, true
	case name == "amnezia-awg2" || strings.HasPrefix(name, "amnezia-awg2-"):
		return "awg2", []string{"/opt/amnezia/awg2", "/opt/amnezia/awg"}, true
	case name == "amnezia-awg" || strings.HasPrefix(name, "amnezia-awg-"):
		return "awg", []string{"/opt/amnezia/awg"}, true
	default:
		return "", nil, false
	}
}

func readDockerCandidate(container, sub string, dirs []string, read func(string, string) ([]byte, error)) (ImportCandidate, bool) {
	for _, dir := range dirs {
		for _, name := range dockerConfNames {
			path := dir + "/" + name
			data, err := read(container, path)
			if err != nil || len(data) == 0 {
				continue
			}
			text := string(data)
			if strings.HasPrefix(text, xuiManagedMarker) {
				continue
			}
			conf := ParseServerConf(text)
			if conf.PrivateKey == "" || conf.ListenPort == 0 {
				continue
			}
			tablePath := dir + "/clientsTable"
			table, _ := read(container, tablePath)
			keys := parseAmneziaClientsTable(table, container+":"+tablePath)
			c := ImportCandidate{
				ID:           ImportSourceDocker + ":" + container,
				Source:       ImportSourceDocker,
				Ifname:       sub,
				ConfPath:     container + ":" + path,
				Port:         conf.ListenPort,
				Address:      conf.Address,
				AwgVersion:   conf.AwgVersion,
				Backend:      "docker",
				DropOnImport: true,
				Warning:      "Docker Amnezia must be stopped after import; clients reconnect on the kernel interface.",
				Conf:         conf,
				Keys:         keys,
				ConfText:     text,
				TableText:    string(table),
			}
			return c, true
		}
	}
	return ImportCandidate{}, false
}

func defaultListDockerContainers() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, dockerBin(), "ps", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}

func defaultReadDockerFile(container, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, dockerBin(), "exec", container, "cat", path).Output()
}

func dockerBin() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/bin/docker", "/usr/local/bin/docker"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return "docker"
}
