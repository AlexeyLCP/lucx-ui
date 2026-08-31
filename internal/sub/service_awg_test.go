package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/awg/vpnuri"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// awgLinkInbound builds an AWG inbound with one client and the given settings
// JSON, mirroring shareLinkInbound in service_sharelink_test.go.
func awgLinkInbound(settings string) *model.Inbound {
	return &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     51820,
		Protocol: model.AWG,
		Remark:   "awg-link",
		Settings: settings,
	}
}

const awgLinkClientSettings = `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
	`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"2",` +
	`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
	`"h1":"100000-500000","h2":"600000-900000","h3":"1000000-1500000","h4":"1600000-2000000",` +
	`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`

func TestGenAwgLink_HeaderProtectionKeyOmittedWhenEmpty(t *testing.T) {
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(awgLinkClientSettings), "user")
	if link == "" {
		t.Fatal("expected a non-empty amneziawg:// link")
	}
	if strings.Contains(link, "headerprotectionkey=") {
		t.Errorf("headerProtectionKey param must be absent when empty, got:\n%s", link)
	}
}

func TestGenAwgLink_HeaderProtectionKeyEmittedWhenSet(t *testing.T) {
	withAwgSupport(t, true)
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"3",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000",` +
		`"headerProtectionKey":"aBcD...base64hpk==",` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(settings), "user")
	if !strings.Contains(link, "headerprotectionkey=aBcD...base64hpk%3D%3D") {
		t.Errorf("headerProtectionKey param (base64 == percent-encoded) must appear when set + awgVersion=3, got:\n%s", link)
	}
}

func TestGenAwgLink_HeaderProtectionKeyEmittedOnV31(t *testing.T) {
	withAwgSupport(t, true)
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"3.1",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000",` +
		`"headerProtectionKey":"aBcD...base64hpk==","randomTrailers":true,` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(settings), "user")
	if !strings.Contains(link, "headerprotectionkey=") {
		t.Errorf("HPK must appear for awgVersion=3.1, got:\n%s", link)
	}
	if !strings.Contains(link, "randomtrailers=true") {
		t.Errorf("randomtrailers must appear for awgVersion=3.1, got:\n%s", link)
	}
}

func TestGenAwgLink_HeaderProtectionKeyOmittedOnNonV3(t *testing.T) {
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"2",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000",` +
		`"headerProtectionKey":"aBcD...base64hpk==",` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(settings), "user")
	if strings.Contains(link, "headerprotectionkey=") {
		t.Errorf("headerProtectionKey must be omitted when awgVersion != '3', got:\n%s", link)
	}
}

func TestGenAwgLink_DeviceFieldsGatedToV3(t *testing.T) {
	withAwgSupport(t, true)
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"3",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000",` +
		`"contentPaddingAddition":64,"rekeyAfterTime":120,"rekeyTimeout":5,` +
		`"rejectAfterTime":180,"keepaliveTimeout":10,"maxHandshakeAttempts":18,` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(settings), "user")
	for _, p := range []string{"contentpaddingaddition=64", "rekeyaftertime=120", "rekeytimeout=5"} {
		if !strings.Contains(link, p) {
			t.Errorf("expected %q in v3 share-link, got:\n%s", p, link)
		}
	}
}

func TestGenAwgLink_PrefersInboundPeerAddress(t *testing.T) {
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"2",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"1","h2":"2","h3":"3","h4":"4",` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true,"allowedIPs":["10.8.0.3/32"]}]}`
	ib := awgLinkInbound(settings)
	ib.Id = 7
	s := &SubService{}
	s.primeLinkClients(ib.Id, []model.Client{{
		Email: "user", PrivateKey: "peerPriv", AllowedIPs: []string{"10.201.0.2/32"},
	}}, true)
	link := s.genAwgLink(ib, "user")
	if !strings.Contains(link, "address=10.8.0.3") {
		t.Errorf("share-link must use inbound peer address, got:\n%s", link)
	}
	if strings.Contains(link, "10.201.0.2") {
		t.Errorf("must not use table address, got:\n%s", link)
	}
}

func TestGenAwgLink_DeviceFieldsOmittedOnNonV3(t *testing.T) {
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"2",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000",` +
		`"contentPaddingAddition":64,"rekeyAfterTime":120,` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(settings), "user")
	if strings.Contains(link, "contentpaddingaddition") {
		t.Errorf("contentpaddingaddition must be omitted when awgVersion != '3', got:\n%s", link)
	}
	if strings.Contains(link, "rekeyaftertime") {
		t.Errorf("rekeyaftertime must be omitted when awgVersion != '3', got:\n%s", link)
	}
}

// TestGenAwgLink_VpnEnvelopeLine locks the second subscription line: the same
// client conf re-encoded as the AmneziaVPN vpn:// envelope. NekoBox+ imports
// AWG from .conf / vpn:// only and Exclave does not parse amneziawg:// at
// all, so a URI-only line never added the node (tester report; HYDRA emits
// vpn:// for the same reason). The envelope must round-trip to a .conf that
// still carries the obfuscation block.
func TestGenAwgLink_VpnEnvelopeLine(t *testing.T) {
	s := &SubService{}
	link := s.genAwgLink(awgLinkInbound(awgLinkClientSettings), "user")
	lines := splitLinkLines(link)
	if len(lines) != 2 {
		t.Fatalf("AWG sub link must carry amneziawg:// + vpn:// lines, got %d:\n%s", len(lines), link)
	}
	if !strings.HasPrefix(lines[0], "amneziawg://") {
		t.Fatalf("first line must stay the amneziawg:// URI, got:\n%s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "vpn://") {
		t.Fatalf("second line must be the vpn:// envelope, got:\n%s", lines[1])
	}
	payload, err := vpnuri.Decode(lines[1])
	if err != nil {
		t.Fatalf("vpn:// line must decode: %v", err)
	}
	conf, err := vpnuri.ConfFromPayload(payload)
	if err != nil {
		t.Fatalf("vpn:// payload must carry the awg container: %v", err)
	}
	for _, want := range []string{"[Interface]", "[Peer]", "Jc = 8", "S3 = 20", "Endpoint = 203.0.113.1:51820"} {
		if !strings.Contains(conf, want) {
			t.Errorf("vpn:// conf missing %q, got:\n%s", want, conf)
		}
	}
}

// withAwgSupport forces the host-capability probe for the duration of a test.
// Without it these link tests silently depend on whether the machine running
// them has the amneziawg module — they passed on a lab node and would fail in CI.
func withAwgSupport(t *testing.T, supported bool) {
	t.Helper()
	awg.SetModuleSupportsAwg3(&supported)
	awg.SetModuleSupportsAwg31(&supported)
	t.Cleanup(func() {
		awg.SetModuleSupportsAwg3(nil)
		awg.SetModuleSupportsAwg31(nil)
	})
}

// The server omits 3.x lines the host tools cannot parse, so a link that turns
// them on hands the client a config the server never applied. For RandomTrailers
// that is fatal: the receiver checks the length against its own flag, so the
// server drops every handshake from a client that padded.
func TestGenAwgLink_OmitsV3FieldsWhenTheHostCannotApplyThem(t *testing.T) {
	settings := `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"3.1",` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000",` +
		`"headerProtectionKey":"aBcD...base64hpk==","randomTrailers":true,"disableCookies":true,` +
		`"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}

	withAwgSupport(t, false)
	for _, param := range []string{"randomtrailers=", "disablecookies=", "headerprotectionkey="} {
		if link := s.genAwgLink(awgLinkInbound(settings), "user"); strings.Contains(link, param) {
			t.Fatalf("%s must be absent when the host cannot apply it, got:\n%s", param, link)
		}
	}

	withAwgSupport(t, true)
	link := s.genAwgLink(awgLinkInbound(settings), "user")
	for _, param := range []string{"randomtrailers=true", "disablecookies=true"} {
		if !strings.Contains(link, param) {
			t.Fatalf("%s must appear when the host supports it, got:\n%s", param, link)
		}
	}
}

// A field of blanks is not a value: the client would write "H1 =  " into its
// own .conf and awg setconf then rejects the file whole.
func TestGenAwgLink_BlankFieldsAreNotValues(t *testing.T) {
	withAwgSupport(t, true)
	const base = `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,`
	const clients = `"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	for _, tc := range []struct {
		name, settings, absent string
	}{
		{"blank header", base + `"awgVersion":"2","h1":"   ",` + clients, "h1="},
		{"blank key", base + `"awgVersion":"3","headerProtectionKey":"  ",` + clients, "headerprotectionkey="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := splitLinkLines(s.genAwgLink(awgLinkInbound(tc.settings), "user"))
			if len(lines) == 0 {
				t.Fatal("expected a non-empty amneziawg:// link")
			}
			if strings.Contains(lines[0], tc.absent) {
				t.Errorf("%q must not reach the share link, got:\n%s", tc.absent, lines[0])
			}
		})
	}
}

// A blank I-field is not a value: the client writes the param into its own
// .conf as "I1 =  ", and awg setconf then rejects that file whole.
func TestGenAwgLink_BlankIFieldIsNotAValue(t *testing.T) {
	const base = `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"2","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,`
	const clients = `"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	for _, tc := range []struct{ name, i1, want string }{
		{"blanks only", "   ", ""},
		// Padding around a value reaches no tag parser, so it is not part of
		// the value and the link carries the trimmed form.
		{"whitespace edges are trimmed off", " <b 0x00> ", "<b 0x00>"},
		{"plain descriptor", "<b 0x00>", "<b 0x00>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := splitLinkLines(s.genAwgLink(awgLinkInbound(base+`"i1":"`+tc.i1+`","i2":"<b 0xff>",`+clients), "user"))
			if len(lines) == 0 {
				t.Fatal("expected a non-empty amneziawg:// link")
			}
			u, err := url.Parse(lines[0])
			if err != nil {
				t.Fatalf("share link must parse: %v, got:\n%s", err, lines[0])
			}
			q := u.Query()
			if got := q.Get("i1"); got != tc.want {
				t.Errorf("i1 = %q, want %q, in:\n%s", got, tc.want, lines[0])
			}
			if got := q.Get("i2"); got != "<b 0xff>" {
				t.Errorf("the rest of the I-set must survive, i2 = %q, in:\n%s", got, lines[0])
			}
		})
	}
}

func TestGenAwgLink_HostDestPortAndRemark(t *testing.T) {
	ib := awgLinkInbound(awgLinkClientSettings)
	ib.StreamSettings = `{"externalProxy":[{"dest":"test.com","port":443,"remark":"cdn","isHost":true,"remarkFinal":true}]}`
	s := &SubService{}
	link := s.genAwgLink(ib, "user")
	if !strings.Contains(link, "@test.com:443") {
		t.Errorf("amneziawg:// must use host dest:port, got:\n%s", link)
	}
	if strings.Contains(link, ":51820") {
		t.Errorf("inbound listen port must not leak, got:\n%s", link)
	}
	lines := splitLinkLines(link)
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "vpn://") {
		t.Fatalf("want amneziawg:// + vpn://, got %d lines", len(lines))
	}
	payload, err := vpnuri.Decode(lines[1])
	if err != nil {
		t.Fatalf("vpn:// decode: %v", err)
	}
	conf, err := vpnuri.ConfFromPayload(payload)
	if err != nil {
		t.Fatalf("vpn:// conf: %v", err)
	}
	if !strings.Contains(conf, "Endpoint = test.com:443") {
		t.Errorf("vpn:// Endpoint must be host dest:port, got:\n%s", conf)
	}
	if !strings.HasPrefix(strings.TrimSpace(conf), "# cdn") {
		t.Errorf("vpn:// must name the host remark, got:\n%s", conf)
	}
}

// One field the client's engine cannot read must not cost it the other four:
// grammar is a property of the value, unlike the netlink budget above it.
func TestGenAwgLink_UnportableIFieldIsDroppedAlone(t *testing.T) {
	const base = `{"privateKey":"serverPrivKeyBase64==","publicKey":"serverPubKeyBase64==",` +
		`"address":"10.8.0.1/24","mtu":1320,"awgVersion":"2","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,`
	const clients = `"clients":[{"publicKey":"peerPub","privateKey":"peerPriv","preSharedKey":"peerPsk","email":"user","enable":true}]}`
	s := &SubService{}
	lines := splitLinkLines(s.genAwgLink(awgLinkInbound(base+
		`"i1":"<b 0x01>","i2":"<t>","i3":"<c>","i4":"<r 8>","i5":"<rc 4>",`+clients), "user"))
	if len(lines) == 0 {
		t.Fatal("expected a non-empty amneziawg:// link")
	}
	u, err := url.Parse(lines[0])
	if err != nil {
		t.Fatalf("share link must parse: %v, got:\n%s", err, lines[0])
	}
	q := u.Query()
	if q.Has("i3") {
		t.Errorf("<c> must not reach the share link, got i3 = %q", q.Get("i3"))
	}
	for key, want := range map[string]string{"i1": "<b 0x01>", "i2": "<t>", "i4": "<r 8>", "i5": "<rc 4>"} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q, in:\n%s", key, got, want, lines[0])
		}
	}
}
