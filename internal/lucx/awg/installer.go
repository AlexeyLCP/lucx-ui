// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InstallResult reports the outcome of an AWG installation attempt.
type InstallResult struct {
	KernelModule bool   // kernel module successfully built and loaded
	Tools        bool   // awg/awg-quick tools installed
	RebootNeeded bool   // kernel updated, reboot required
	Log          string // human-readable installation log
}

// InstallAWG installs the AmneziaWG kernel module and tools from source.
// Follows the pumbaX/awg-multi-script approach: git clone + dkms + make install.
// Idempotent — safe to call if already installed.
func InstallAWG() (*InstallResult, error) {
	result := &InstallResult{}

	// Already installed?
	if IsAWGInstalled() {
		result.KernelModule = true
		result.Tools = true
		result.Log = "AWG already installed"
		return result, nil
	}

	var logLines []string

	// 1. Kernel module
	if !kernelModuleLoaded() {
		logLines = append(logLines, "Building kernel module from source...")
		if err := installKernelModule(); err != nil {
			logLines = append(logLines, fmt.Sprintf("Kernel module build failed: %v", err))
		} else {
			// Load the module
			if out, err := exec.Command("modprobe", "amneziawg").CombinedOutput(); err != nil {
				logLines = append(logLines, fmt.Sprintf("modprobe failed (reboot may be needed): %v\n%s", err, string(out)))
				result.RebootNeeded = true
			} else {
				// Enable autostart
				os.WriteFile("/etc/modules-load.d/amneziawg.conf", []byte("amneziawg\n"), 0644)
				result.KernelModule = true
				logLines = append(logLines, "Kernel module installed and loaded")
			}
		}
	}

	// 2. Tools
	if !toolsInstalled() {
		logLines = append(logLines, "Building tools from source...")
		if err := installTools(); err != nil {
			logLines = append(logLines, fmt.Sprintf("Tools build failed: %v", err))
		} else {
			result.Tools = true
			logLines = append(logLines, "Tools installed")
		}
	}

	result.Log = strings.Join(logLines, "; ")
	return result, nil
}

// IsAWGInstalled checks whether both the kernel module and tools are available.
func IsAWGInstalled() bool {
	return kernelModuleLoaded() && toolsInstalled()
}

func kernelModuleLoaded() bool {
	_, err := os.Stat("/sys/module/amneziawg")
	return err == nil
}

func toolsInstalled() bool {
	_, err := exec.LookPath("awg")
	return err == nil
}

func installKernelModule() error {
	// Clone and build via DKMS (pumbaX method)
	cloneDir := "/tmp/amneziawg-linux-kernel-module"
	os.RemoveAll(cloneDir)

	clone := exec.Command("git", "clone", "--depth", "1",
		"https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git", cloneDir)
	if out, err := clone.CombinedOutput(); err != nil {
		return fmt.Errorf("clone: %w\n%s", err, string(out))
	}

	cmds := [][]string{
		{"make", "-C", cloneDir + "/src", "dkms-install"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w\n%s", args[0], err, string(out))
		}
	}

	// DKMS add/build/install
	modVer := "1.0.0"
	for _, args := range [][]string{
		{"dkms", "add", "-m", "amneziawg", "-v", modVer},
		{"dkms", "build", "-m", "amneziawg", "-v", modVer},
		{"dkms", "install", "-m", "amneziawg", "-v", modVer},
	} {
		exec.Command(args[0], args[1:]...).Run()
	}

	os.RemoveAll(cloneDir)
	return nil
}

func installTools() error {
	cloneDir := "/tmp/amneziawg-tools"
	os.RemoveAll(cloneDir)

	clone := exec.Command("git", "clone", "--depth", "1",
		"https://github.com/amnezia-vpn/amneziawg-tools.git", cloneDir)
	if out, err := clone.CombinedOutput(); err != nil {
		return fmt.Errorf("clone tools: %w\n%s", err, string(out))
	}

	make := exec.Command("make", "-C", cloneDir+"/src")
	if out, err := make.CombinedOutput(); err != nil {
		os.RemoveAll(cloneDir)
		return fmt.Errorf("make tools: %w\n%s", err, string(out))
	}

	install := exec.Command("make", "-C", cloneDir+"/src", "install")
	if out, err := install.CombinedOutput(); err != nil {
		os.RemoveAll(cloneDir)
		return fmt.Errorf("install tools: %w\n%s", err, string(out))
	}

	os.RemoveAll(cloneDir)
	return nil
}
