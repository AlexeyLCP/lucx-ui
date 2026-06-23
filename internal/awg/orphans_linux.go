//go:build linux

package awg

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// killStrayTun2socksProcesses terminates orphaned tun2socks daemons left over
// from a previous x-ui run and returns how many were killed. x-ui starts one
// tun2socks per AWG inbound; a survivor holds a TUN device and SOCKS port that
// the new run needs. x-ui is the sole owner of tun2socks, so any process
// matching the name at startup is an orphan and is safe to kill.
func killStrayTun2socksProcesses() int {
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if !cmdlineContains(pid, "tun2socks") {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			killed++
		}
	}
	return killed
}

// killStrayAwgInterfaces removes AWG kernel interfaces left over from a
// previous x-ui run and returns how many were removed. A survivor holds the
// inbound's UDP port with stale obfuscation, so new clients cannot connect.
// x-ui is the sole owner of awgN interfaces, so any "awg*" interface at
// startup is an orphan and is safe to delete.
func killStrayAwgInterfaces() int {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "awg") {
			continue
		}
		if err := exec.Command("ip", "link", "del", name).Run(); err == nil {
			killed++
		}
	}
	return killed
}

func cmdlineContains(pid int, needle string) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), needle)
}