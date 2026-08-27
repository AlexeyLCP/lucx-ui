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

// The server emits its CPS burst before every handshake initiation it sends,
// and either side can initiate. Leaving I1-I5 out made the mimicry one-sided:
// a watcher saw the imitated protocol from the client and bare AmneziaWG back.
func TestRenderServerConf_CarriesIFields(t *testing.T) {
	inst := Instance{
		Ifname:     "awg9",
		PrivateKey: "cJp4uHNyKcRVeTSanE/Xn/Y7OTaTjxbYq+wUOxOKMWQ=",
		Port:       51820,
		AwgVersion: "2",
		Jc:         6, Jmin: 11, Jmax: 76,
		S1: 40, S2: 55, S3: 56, S4: 12,
		I1: "<b 0x160301><r 32>",
		I2: "<b 0x1403030001>",
		I5: "<b 0x0d0a0d0a>",
	}
	conf := renderServerConf(inst)
	for _, want := range []string{"I1 = <b 0x160301><r 32>", "I2 = <b 0x1403030001>", "I5 = <b 0x0d0a0d0a>"} {
		if !strings.Contains(conf, want) {
			t.Errorf("server conf must carry %q, got:\n%s", want, conf)
		}
	}
	// An unset field must stay out: "I3 = " is rejected by awg setconf.
	if strings.Contains(conf, "I3 =") || strings.Contains(conf, "I4 =") {
		t.Errorf("empty I-fields must be omitted, got:\n%s", conf)
	}
}

// The netlink read budget is a property of the interface, not of who sends the
// packets, so the server is gated exactly like the client export.
func TestRenderServerConf_OmitsIFieldsOverTheNetlinkBudget(t *testing.T) {
	huge := "<b 0x" + strings.Repeat("ab", 1800) + ">"
	inst := Instance{
		Ifname:     "awg9",
		PrivateKey: "cJp4uHNyKcRVeTSanE/Xn/Y7OTaTjxbYq+wUOxOKMWQ=",
		Port:       51820,
		AwgVersion: "2",
		I1:         huge, I2: huge,
	}
	if conf := renderServerConf(inst); strings.Contains(conf, "I1 =") {
		t.Errorf("an oversized set leaves the interface unreadable by `awg show`; it must be omitted, got %d bytes for I1", len(huge))
	}
}
