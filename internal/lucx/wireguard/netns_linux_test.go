//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"golang.org/x/sys/unix"
)

func TestKernelWireguardNetNS(t *testing.T) {
	if os.Getenv("LUCX_WG_NETNS_TEST") != "1" {
		t.Skip("opt-in root Linux namespace test")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("unshare", "--net", "--", exe, "-test.run=^TestKernelWireguardNetNSWorker$", "-test.v")
	cmd.Env = append(os.Environ(), "LUCX_WG_NETNS_WORKER=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("kernel WireGuard namespace test failed: %v\n%s", err, out)
	}
}

func TestKernelWireguardNetNSProbe(t *testing.T) {
	mode := os.Getenv("LUCX_WG_NETNS_PROBE")
	if mode == "" {
		t.Skip("namespace probe only")
	}
	protocol, address := "tcp4", "198.51.100.1:80"
	if mode == "udp" {
		protocol, address = "udp4", "198.51.100.1:53"
	}
	c, err := net.DialTimeout(protocol, address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err = c.Write([]byte("wg-probe")); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 32)
	n, err := c.Read(b)
	if os.Getenv("LUCX_WG_EXPECT_DROP") == "1" {
		if err == nil {
			t.Fatal("traffic passed without a transparent listener")
		}
		return
	}
	if err != nil || string(b[:n]) != "wg-ok" {
		t.Fatalf("transparent probe failed: %v", err)
	}
}

func TestKernelWireguardNetNSWorker(t *testing.T) {
	if os.Getenv("LUCX_WG_NETNS_WORKER") != "1" {
		t.Skip("isolated worker only")
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v", args[0], err)
		}
		return string(out)
	}
	run("ip", "link", "set", "lo", "up")
	run("ip", "link", "add", "wg0", "type", "wireguard")
	run("ip", "link", "set", "wg0", "alias", "foreign")
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	private, public, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	inst.Peers[0].PublicKey = public
	inst.FwMark = 0xca6c
	manager := NewManager(t.TempDir(), nil)
	defer manager.StopAll()
	if err = manager.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	ns := fmt.Sprintf("lucx-wgtest-%d", os.Getpid())
	run("ip", "netns", "add", ns)
	defer exec.Command("ip", "netns", "del", ns).Run()
	client := filepath.Join(t.TempDir(), "client.conf")
	serverPublic, err := wgutil.PublicKeyFromPrivate(inst.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf("[Interface]\nPrivateKey = %s\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = 0.0.0.0/0\nEndpoint = 127.0.0.1:%d\nPersistentKeepalive = 1\n", private, serverPublic, inst.Peers[0].PSK, inst.Port)
	if err = os.WriteFile(client, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	run("ip", "link", "add", "wgclient", "type", "wireguard")
	run("wg", "setconf", "wgclient", client)
	run("ip", "link", "set", "wgclient", "netns", ns)
	run("ip", "-n", ns, "address", "add", "10.90.0.2/32", "dev", "wgclient")
	run("ip", "-n", ns, "link", "set", "lo", "up")
	run("ip", "-n", ns, "link", "set", "wgclient", "mtu", "1280", "up")
	run("ip", "-n", ns, "route", "add", "default", "dev", "wgclient")
	restore := `table ip wg_test_restore {
 chain prerouting {
  type filter hook prerouting priority mangle; policy accept;
  meta l4proto udp meta mark set ct mark;
 }
}
`
	if _, err = systemCommand(restore, "nft", "-f", "-"); err != nil {
		t.Fatal(err)
	}
	lc := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var e error
		if err := raw.Control(func(fd uintptr) { e = unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_TRANSPARENT, 1) }); err != nil {
			return err
		}
		return e
	}}
	address := fmt.Sprintf("127.0.0.1:%d", inst.TproxyPort)
	tcp, err := lc.Listen(context.Background(), "tcp4", address)
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	go func() {
		for {
			c, e := tcp.Accept()
			if e != nil {
				return
			}
			go func() {
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				b := make([]byte, 32)
				if _, e := c.Read(b); e == nil {
					c.Write([]byte("wg-ok"))
				}
			}()
		}
	}()
	udp, err := lc.ListenPacket(context.Background(), "udp4", address)
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	reply, err := lc.ListenPacket(context.Background(), "udp4", "198.51.100.1:53")
	if err != nil {
		t.Fatal(err)
	}
	defer reply.Close()
	go func() {
		b := make([]byte, 32)
		for {
			_, peer, e := udp.ReadFrom(b)
			if e != nil {
				return
			}
			reply.WriteTo([]byte("wg-ok"), peer)
		}
	}()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	probe := func(mode string, drop bool) {
		t.Helper()
		cmd := exec.Command("ip", "netns", "exec", ns, exe, "-test.run=^TestKernelWireguardNetNSProbe$", "-test.v")
		cmd.Env = append(os.Environ(), "LUCX_WG_NETNS_PROBE="+mode)
		if drop {
			cmd.Env = append(cmd.Env, "LUCX_WG_EXPECT_DROP=1")
		}
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%s probe failed: %v\n%s", mode, e, out)
		}
	}
	run("iptables", "-P", "INPUT", "DROP")
	run("iptables", "-A", "INPUT", "-i", "lo", "-j", "ACCEPT")
	probe("tcp", false)
	probe("udp", false)
	before := strings.TrimSpace(run("wg", "show", inst.Name(), "latest-handshakes"))
	if err = manager.Ensure(inst); err != nil {
		t.Fatalf("%v; nft JSON: %s", err, run("nft", "-j", "list", "table", "inet", inst.FirewallTable()))
	}
	after := strings.TrimSpace(run("wg", "show", inst.Name(), "latest-handshakes"))
	if before != after {
		t.Fatal("no-op reconcile discarded handshake")
	}
	inst.FwMark = 1234
	if err = manager.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(run("wg", "show", inst.Name(), "fwmark")) != "0x4d2" {
		t.Fatal("transport mark was not applied")
	}
	if strings.TrimSpace(run("wg", "show", inst.Name(), "latest-handshakes")) != after {
		t.Fatal("transport mark change reset the handshake")
	}
	probe("udp", false)
	run("nft", "delete", "table", "inet", inst.FirewallTable())
	if err = manager.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	probe("udp", false)
	usages := manager.CollectTraffic()
	var up, down int64
	for _, u := range usages {
		up += u.Up
		down += u.Down
	}
	if up == 0 || down == 0 {
		t.Fatal("kernel traffic was not accounted")
	}
	invalid := testInbound(t)
	invalid.Settings = strings.Replace(invalid.Settings, testKey(1), "invalid", 1)
	if err = manager.RevokeDisallowed(invalid); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(run("wg", "show", inst.Name(), "peers")) != "" {
		t.Fatal("deleted peer retained access under invalid device settings")
	}
	if err = manager.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	run("ip", "netns", "exec", ns, "wg", "set", "wgclient", "peer", serverPublic, "remove")
	run("ip", "netns", "exec", ns, "wg", "setconf", "wgclient", client)
	probe("udp", false)
	udp.Close()
	reply.Close()
	tcp.Close()
	probe("udp", true)
	if !regexp.MustCompile(`counter packets [1-9][0-9]* bytes`).MatchString(run("nft", "list", "table", "inet", inst.FirewallTable())) {
		t.Fatal("missing transparent listener did not hit the drop guard")
	}
	if err = manager.Remove(inst.ID); err != nil {
		t.Fatal(err)
	}
	run("ip", "link", "show", "wg0")
	if out, err := exec.Command("ip", "link", "show", inst.Name()).CombinedOutput(); err == nil {
		t.Fatalf("managed device survived deletion: %s", out)
	}
}

func TestSystemCommandReportsBoundedRedactedStderr(t *testing.T) {
	secret := strings.Repeat("A", 43) + "="
	_, err := systemCommand("", "sh", "-c", "printf '%s' '"+secret+" "+strings.Repeat("x", 800)+"' >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "exit 7") || !strings.Contains(err.Error(), "[redacted-key]") || strings.Contains(err.Error(), secret) || len(err.Error()) > 650 {
		t.Fatalf("unsafe or missing command diagnostics: %v", err)
	}
}

func TestSystemCommandDiagnosticTruncationPreservesUTF8(t *testing.T) {
	_, err := systemCommand("", "sh", "-c", "printf '%s' '"+"x"+strings.Repeat("я", 400)+"' >&2; exit 7")
	if err == nil || !utf8.ValidString(err.Error()) || len(err.Error()) > 650 {
		t.Fatalf("invalid or unbounded UTF-8 diagnostic: %v", err)
	}
}
