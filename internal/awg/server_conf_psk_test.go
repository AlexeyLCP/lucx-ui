package awg

import (
	"strings"
	"testing"
)

// TestRenderServerConf_EmptyPSKOmitted is the regression test for the
// "creating a client drops the interface" bug (VladufQa, 2026-08-03). A client
// that reaches renderServerConf without a PSK (e.g. added via the inbound-form
// update path, which does not run defaultAwgClients) used to render
// "PresharedKey = " with an empty value; awg setconf rejects that line,
// awg-quick rolls back the interface, and reconcile reports "Device <awgN>
// does not exist". The empty line must be omitted (WireGuard convention for
// "no PSK"), matching renderClientConf and SyncPeers.
func TestRenderServerConf_EmptyPSKOmitted(t *testing.T) {
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		Address: "10.8.0.1/24",
		Peers: []PeerSpec{
			{PublicKey: "peer-pub", PSK: "", Keepalive: "25", AllowedIPs: "10.8.0.2/32"},
		},
	}
	conf := renderServerConf(inst)

	if strings.Contains(conf, "PresharedKey") {
		t.Errorf("empty PSK must be omitted from the server .conf, got:\n%s", conf)
	}
	for _, want := range []string{
		"[Peer]",
		"PublicKey = peer-pub",
		"AllowedIPs = 10.8.0.2/32",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("server .conf missing %q\nConf:\n%s", want, conf)
		}
	}
	if strings.Contains(conf, "PersistentKeepalive") {
		t.Errorf("PersistentKeepalive is client-export only, got:\n%s", conf)
	}
}

// TestRenderServerConf_NonEmptyPSKWritten confirms a client WITH a PSK still
// gets the line (the fix must not drop legitimate PSKs).
func TestRenderServerConf_NonEmptyPSKWritten(t *testing.T) {
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		Address: "10.8.0.1/24",
		Peers: []PeerSpec{
			{PublicKey: "peer-pub", PSK: "cHJlc2hhcmVkLWtleQ==", Keepalive: "25", AllowedIPs: "10.8.0.2/32"},
		},
	}
	conf := renderServerConf(inst)
	if !strings.Contains(conf, "PresharedKey = cHJlc2hhcmVkLWtleQ==") {
		t.Errorf("non-empty PSK must be written to the server .conf, got:\n%s", conf)
	}
}

// TestRenderServerConf_WhitespaceOnlyPSKOmitted treats a whitespace-only PSK as
// empty (matches the strings.TrimSpace guard).
func TestRenderServerConf_WhitespaceOnlyPSKOmitted(t *testing.T) {
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		Address: "10.8.0.1/24",
		Peers: []PeerSpec{
			{PublicKey: "peer-pub", PSK: "   ", Keepalive: "25", AllowedIPs: "10.8.0.2/32"},
		},
	}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "PresharedKey") {
		t.Errorf("whitespace-only PSK must be omitted, got:\n%s", conf)
	}
}

// TestRenderServerConf_ManagedMarkerFirstLine is the lucx.67 ownership-marker
// regression test. Every rendered server .conf must begin with xuiManagedMarker
// so the orphan sweep / Remove can tell LucX-UI configs apart from foreign ones
// (e.g. WGDashboard's awg0.conf) that share the awg{N}.conf naming. The marker
// must be a '#' comment on the very first line so awg-quick ignores it.
func TestRenderServerConf_ManagedMarkerFirstLine(t *testing.T) {
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		Address: "10.8.0.1/24",
		Peers: []PeerSpec{
			{PublicKey: "peer-pub", PSK: "cHJlc2hhcmVkLWtleQ==", Keepalive: "25", AllowedIPs: "10.8.0.2/32"},
		},
	}
	conf := renderServerConf(inst)
	if !strings.HasPrefix(conf, xuiManagedMarker+"\n") {
		t.Errorf("server .conf must start with the ownership marker %q, got first line %q",
			xuiManagedMarker, strings.SplitN(conf, "\n", 2)[0])
	}
	if !strings.HasPrefix(xuiManagedMarker, "#") {
		t.Errorf("ownership marker must be a '#' comment so awg-quick ignores it, got %q", xuiManagedMarker)
	}
	// The marker must precede [Interface].
	if strings.Index(conf, "[Interface]") < strings.Index(conf, xuiManagedMarker) {
		t.Errorf("ownership marker must precede [Interface], got:\n%s", conf)
	}
}
