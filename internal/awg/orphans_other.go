//go:build !linux

package awg

// killStrayAwgInterfaces is a no-op off Linux. AWG is a Linux kernel module;
// other platforms are not a supported deployment target for the AWG sidecar,
// so orphan sweeping is unnecessary there. There is no tun2socks daemon to
// sweep — routing into Xray is via an injected TUN inbound owned by Xray.
func killStrayAwgInterfaces() int { return 0 }