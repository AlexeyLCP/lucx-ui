// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientFingerprint_RestartDetection(t *testing.T) {
	o := &model.AwgOutbound{Id: 2, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if renderClientConf(ci1) == renderClientConf(ci2) {
		t.Fatal("fingerprint must change when Address changes (restart trigger)")
	}
}

func TestCollectClientTraffic_NoInterface(t *testing.T) {
	m := GetManager()
	_, _, _, ok := m.CollectClientTraffic("awgo-99999")
	if ok {
		t.Error("CollectClientTraffic should return ok=false for non-existent interface")
	}
}

// TestParseClientDump locks in the `awg show <iface> dump` field-index
// mapping for CollectClientTraffic. The dump format (matching scrapePeers /
// parseAwgDump) is one interface line followed by one tab-delimited peer line:
//
//	[0]=pubkey  [1]=psk  [2]=endpoint  [3]=allowed-ips
//	[4]=latest-handshake-epoch  [5]=rx  [6]=tx  [7]=keepalive
//
// This guards against a regression to the old plain-`awg show` parser that
// never matched and silently returned zero counters.
func TestParseClientDump(t *testing.T) {
	// Interface line (skipped — only 4 fields, < 7) then one peer row.
	const dump = "awgo-1\tprivate-key-here\thash-table=off\t10.9.0.5/32\t1280\t1472\toff\n" +
		"peerPubKeyXYZ\tpeerPskABC\t1.2.3.4:51820\t10.9.0.1/32\t1700000000\t1024\t2048\toff\n"

	now := time.Unix(1700000010, 0) // 10s after the handshake epoch
	age, rx, tx, ok := parseClientDump(dump, now)
	if !ok {
		t.Fatal("parseClientDump should return ok=true when a peer row is present")
	}
	if rx != 1024 {
		t.Errorf("rx = %d, want 1024 (fields[5])", rx)
	}
	if tx != 2048 {
		t.Errorf("tx = %d, want 2048 (fields[6])", tx)
	}
	if age != 10*time.Second {
		t.Errorf("handshakeAge = %v, want 10s (fields[4] epoch vs now)", age)
	}
}

// TestParseClientDump_RealAwgInterfaceLine confirms the parser SKIPS the
// amneziawg interface row (which has 18+ tab-delimited fields: private key,
// public key, listen port, jc/jmin/jmax/s1-s4/h1-h4/i1-i5…) and does not
// mistake its numeric fields for the peer's rx/tx/handshake. Without skipping
// the first line, the interface row's fields[4..6] (jmin/jmax/s1, all 0 in
// this fixture) would shadow the peer's real counters.
func TestParseClientDump_RealAwgInterfaceLine(t *testing.T) {
	const dump = "clientPrivKey\tclientPubKey\t60920\t4\t40\t70\t15\t88\t15\t25\t123456789\t234567891\t345678912\t456789123\t(null)\t(null)\t(null)\t(null)\t(null)\toff\n" +
		"upstreamPubKey\t(none)\t127.0.0.1:52901\t0.0.0.0/0,::/0\t0\t0\t3996\t25\n"
	_, rx, tx, ok := parseClientDump(dump, time.Now())
	if !ok {
		t.Fatal("parseClientDump should return ok=true when a peer row is present")
	}
	if rx != 0 {
		t.Errorf("rx = %d, want 0 — interface row must be skipped, peer rx is fields[5]=0", rx)
	}
	if tx != 3996 {
		t.Errorf("tx = %d, want 3996 — interface row must be skipped, peer tx is fields[6]=3996", tx)
	}
}

// TestParseClientDump_NoPeerRow confirms an up interface with no parseable
// peer row (peer added but never connected) returns ok=true with zero
// counters — the spec's "up but idle" state.
func TestParseClientDump_NoPeerRow(t *testing.T) {
	const dump = "awgo-1\tprivate-key-here\thash-table=off\t10.9.0.5/32\t1280\t1472\toff\n"
	_, rx, tx, ok := parseClientDump(dump, time.Now())
	if !ok {
		t.Fatal("parseClientDump should return ok=true for an up interface")
	}
	if rx != 0 || tx != 0 {
		t.Errorf("counters = (rx=%d, tx=%d), want (0, 0)", rx, tx)
	}
}

// TestParseClientDump_Empty confirms empty/garbage output is treated as ok=true
// with zero counters (matching CollectClientTraffic's old fall-through
// semantics) — the real down-vs-up distinction is driven by awgShowIfname's
// error, not by the parser.
func TestParseClientDump_Empty(t *testing.T) {
	_, _, _, ok := parseClientDump("", time.Now())
	if !ok {
		t.Fatal("parseClientDump(\"\") should return ok=true (down is signalled by awgShowIfname error)")
	}
}

// TestSweepOrphanClients_Idempotent guards the once-only bookkeeping;
// withTempConfigDir keeps it off the real host's awgo-* interfaces.
func TestSweepOrphanClients_Idempotent(t *testing.T) {
	withTempConfigDir(t)
	m := GetManager()
	m.SweepOrphanClients(nil)
	m.SweepOrphanClients(nil)
}

// The sweep's doc comment promised a check against awg_outbounds, but the body
// consulted the in-memory clients map — empty on every fresh process, so the
// first tick after a restart swept every live outbound away. The wanted set now
// comes from the caller, which reads the rows.
func TestSweepOrphanClients_KeepsWhatTheDatabaseStillWants(t *testing.T) {
	dir := withTempConfigDir(t)
	const kept, orphan = "awgo-3", "awgo-4"
	for _, name := range []string{kept + ".conf", orphan + ".conf"} {
		writeConf(t, name, xuiManagedMarker+"\n[Interface]\nPrivateKey = x\n")
	}

	clientMu.Lock()
	saved := clients
	// The map is deliberately left empty: that is the state a fresh process is
	// in, and the whole point is that it must no longer decide anything.
	clients, clientSwept = map[string]clientState{}, sync.Once{}
	clientMu.Unlock()
	t.Cleanup(func() {
		clientMu.Lock()
		clients = saved
		clientMu.Unlock()
	})

	GetManager().SweepOrphanClients(map[string]struct{}{kept: {}})

	if _, err := os.Stat(filepath.Join(dir, orphan+".conf")); err == nil {
		t.Error("an outbound with no row left must be swept")
	}
	if _, err := os.Stat(filepath.Join(dir, kept+".conf")); err != nil {
		t.Errorf("a live outbound the database still wants was swept away: %v", err)
	}
}

// The blackhole decision in the Xray config reads this: a recorded down must
// stay down and a recorded up must stay up. That it does so without shelling
// out is pinned separately, where the call can be counted.
func TestClientIfaceUp_ReportsTheRecordedState(t *testing.T) {
	withTempConfigDir(t)
	const ifname = "awgo-77"

	clientMu.Lock()
	savedUp := clientUp
	clientUp = map[string]bool{ifname: true}
	clientMu.Unlock()
	t.Cleanup(func() {
		clientMu.Lock()
		clientUp = savedUp
		clientMu.Unlock()
	})

	// No awg binary is reachable here, so a probe could only answer "down".
	// Answering "up" proves the recorded state was read instead.
	if !GetManager().ClientIfaceUp(ifname) {
		t.Fatal("a recorded up interface must be reported up without a probe")
	}

	clientMu.Lock()
	clientUp[ifname] = false
	clientMu.Unlock()
	if GetManager().ClientIfaceUp(ifname) {
		t.Fatal("a recorded down interface must be reported down")
	}
}

// An interface this process has never touched still has to get a real answer,
// or the first config after a boot would blackhole a tunnel that is up.
func TestClientIfaceUp_ProbesOnceForAnUnknownInterface(t *testing.T) {
	withTempConfigDir(t)
	const ifname = "awgo-78"

	clientMu.Lock()
	savedUp := clientUp
	clientUp = map[string]bool{}
	clientMu.Unlock()
	t.Cleanup(func() {
		clientMu.Lock()
		clientUp = savedUp
		clientMu.Unlock()
	})

	_ = GetManager().ClientIfaceUp(ifname)

	clientMu.Lock()
	_, remembered := clientUp[ifname]
	clientMu.Unlock()
	if !remembered {
		t.Fatal("the one probe an unknown interface earns must be remembered, or every config build repeats it")
	}
}
