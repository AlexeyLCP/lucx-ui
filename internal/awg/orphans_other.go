//go:build !linux

package awg

// killStrayTun2socksProcesses is a no-op off Linux. AWG is a Linux kernel
// module; other platforms are not a supported deployment target for the AWG
// sidecar, so orphan sweeping is unnecessary there.
func killStrayTun2socksProcesses() int { return 0 }

// killStrayAwgInterfaces is a no-op off Linux for the same reason.
func killStrayAwgInterfaces() int { return 0 }