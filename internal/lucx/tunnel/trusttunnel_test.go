// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestTrustTunnelValidate(t *testing.T) {
	cfg := DefaultTrustTunnelConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty hostname must fail")
	}
	cfg.Hostname = "vpn.example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cfg.Listen = "no-port"
	if err := cfg.Validate(); err == nil {
		t.Fatal("bad listen must fail")
	}
	cfg = DefaultTrustTunnelConfig()
	cfg.Hostname = "vpn.example.com"
	cfg.UpstreamProtocol = "http4"
	if err := cfg.Validate(); err == nil {
		t.Fatal("bad upstream protocol must fail")
	}
	cfg = DefaultTrustTunnelConfig()
	cfg.Hostname = "vpn.example.com"
	cfg.ListenPreset = "turbo"
	if err := cfg.Validate(); err == nil {
		t.Fatal("bad listen preset must fail")
	}
}

func TestTrustTunnelRenderVpnToml(t *testing.T) {
	cfg := DefaultTrustTunnelConfig()
	cfg.Hostname = "vpn.example.com"
	cfg.Listen = "0.0.0.0:8443"

	got := cfg.RenderVpnToml("/w/creds.toml", "/w/rules.toml", "", "")
	for _, want := range []string{
		`listen_address = "0.0.0.0:8443"`,
		`credentials_file = "/w/creds.toml"`,
		`rules_file = "/w/rules.toml"`,
		"[listen_protocols.http2]",
		"upload_buffer_size = 524288",
		"initial_stream_window_size = 4194304",
		"initial_connection_window_size = 67108864",
		"[forward_protocol]",
		"direct = {}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("vpn.toml missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "socks5") || strings.Contains(got, "[metrics]") {
		t.Fatal("direct config must not carry socks5/metrics")
	}

	cfg.ListenPreset = "stock"
	got = cfg.Merge().RenderVpnToml("/w/creds.toml", "/w/rules.toml", "", "")
	for _, want := range []string{
		"upload_buffer_size = 32768",
		"initial_stream_window_size = 131072",
		"initial_connection_window_size = 8388608",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stock vpn.toml missing %q:\n%s", want, got)
		}
	}

	got = cfg.RenderVpnToml("/w/creds.toml", "/w/rules.toml", "127.0.0.1:12345", "127.0.0.1:1987")
	if !strings.Contains(got, "[forward_protocol.socks5]") || !strings.Contains(got, `address = "127.0.0.1:12345"`) {
		t.Fatalf("routed config missing socks5 block:\n%s", got)
	}
	if !strings.Contains(got, "[metrics]") || !strings.Contains(got, `address = "127.0.0.1:1987"`) {
		t.Fatalf("metrics block missing:\n%s", got)
	}
}

func TestTrustTunnelRenderHostsAndCredentials(t *testing.T) {
	cfg := DefaultTrustTunnelConfig()
	cfg.Hostname = "vpn.example.com"
	hosts := cfg.RenderHostsToml("/root/cert/d/fullchain.pem", "/root/cert/d/privkey.pem")
	for _, want := range []string{"[[main_hosts]]", `hostname = "vpn.example.com"`, `cert_chain_path = "/root/cert/d/fullchain.pem"`, `private_key_path = "/root/cert/d/privkey.pem"`} {
		if !strings.Contains(hosts, want) {
			t.Fatalf("hosts.toml missing %q:\n%s", want, hosts)
		}
	}
	creds := RenderTrustTunnelCredentials([]AuthPair{{User: "tu1", Pass: "p1"}, {User: " ", Pass: "x"}})
	if strings.Count(creds, "[[client]]") != 1 {
		t.Fatalf("blank user must be skipped:\n%s", creds)
	}
	if !strings.Contains(creds, `username = "tu1"`) || !strings.Contains(creds, `password = "p1"`) {
		t.Fatalf("credentials wrong:\n%s", creds)
	}
}

// writeTestCert generates a self-signed cert+key for the given DNS SANs into
// dir and returns their paths.
func writeTestCert(t *testing.T, dir string, notAfter time.Time, dnsNames ...string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")
	certOut, _ := os.Create(certPath)
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	certOut.Close()
	keyDer, _ := x509.MarshalECPrivateKey(key)
	keyOut, _ := os.Create(keyPath)
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}); err != nil {
		t.Fatal(err)
	}
	keyOut.Close()
	return certPath, keyPath
}

func writeFutureCert(t *testing.T, dir, dns string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: dns},
		NotBefore:    time.Now().Add(24 * time.Hour),
		NotAfter:     time.Now().Add(48 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{dns},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "future.pem")
	keyPath := filepath.Join(dir, "future-key.pem")
	certOut, _ := os.Create(certPath)
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	certOut.Close()
	keyDer, _ := x509.MarshalECPrivateKey(key)
	keyOut, _ := os.Create(keyPath)
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}); err != nil {
		t.Fatal(err)
	}
	keyOut.Close()
	return certPath, keyPath
}

func TestValidateCertFiles(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "vpn.example.com")

	if err := ValidateCertFiles(certPath, keyPath, "vpn.example.com"); err != nil {
		t.Fatalf("valid cert rejected: %v", err)
	}
	if err := ValidateCertFiles(certPath, keyPath, "other.example.com"); err == nil {
		t.Fatal("cert must not cover a foreign domain")
	}
	if err := ValidateCertFiles("", keyPath, "vpn.example.com"); err == nil {
		t.Fatal("missing cert path must fail")
	}
	if err := ValidateCertFiles(filepath.Join(dir, "nope.pem"), keyPath, "vpn.example.com"); err == nil {
		t.Fatal("unreadable cert must fail")
	}

	expired, expiredKey := writeTestCert(t, dir, time.Now().Add(-time.Hour), "vpn.example.com")
	if err := ValidateCertFiles(expired, expiredKey, "vpn.example.com"); err == nil {
		t.Fatal("expired cert must fail")
	}

	futC, futK := writeFutureCert(t, t.TempDir(), "vpn.example.com")
	if err := ValidateCertFiles(futC, futK, "vpn.example.com"); err == nil {
		t.Fatal("not-yet-valid cert must fail")
	}
}

func TestCertFileHash(t *testing.T) {
	dir := t.TempDir()
	certPath, _ := writeTestCert(t, dir, time.Now().Add(time.Hour), "a.example.com")
	h1 := CertFileHash(certPath)
	if len(h1) != 64 {
		t.Fatalf("hash = %q", h1)
	}
	if CertFileHash(filepath.Join(dir, "missing")) != "" {
		t.Fatal("missing file must yield empty hash")
	}
}

func TestTrustTunnelClientDeepLink(t *testing.T) {
	cfg := DefaultTrustTunnelConfig()
	cfg.Hostname = "vpn.example.com"
	cfg.ClientDNS = "1.1.1.1, tls://8.8.8.8"
	link := cfg.ClientDeepLink("vpn.example.com:8443", AuthPair{User: "tu1", Pass: "p1"}, "my-vpn")
	if !strings.HasPrefix(link, "tt://?") {
		t.Fatalf("link prefix: %s", link)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "tt://?"))
	if err != nil {
		t.Fatalf("base64url decode: %v", err)
	}
	// Parse TLVs back.
	tlvs := map[byte][][]byte{}
	i := 0
	for i < len(payload) {
		tag := payload[i]
		i++
		// varint length (test values are small — 1-byte varints)
		l := int(payload[i])
		i++
		tlvs[tag] = append(tlvs[tag], payload[i:i+l])
		i += l
	}
	if string(tlvs[0x00][0]) != string([]byte{0x01}) {
		t.Fatalf("version tag = %v", tlvs[0x00])
	}
	if string(tlvs[0x01][0]) != "vpn.example.com" {
		t.Fatalf("hostname = %q", tlvs[0x01][0])
	}
	if string(tlvs[0x02][0]) != "vpn.example.com:8443" {
		t.Fatalf("address = %q", tlvs[0x02][0])
	}
	if string(tlvs[0x05][0]) != "tu1" || string(tlvs[0x06][0]) != "p1" {
		t.Fatal("credentials wrong")
	}
	if string(tlvs[0x0C][0]) != "my-vpn" {
		t.Fatalf("name = %q", tlvs[0x0C][0])
	}
	if _, ok := tlvs[0x04]; ok {
		t.Fatal("has_ipv6=true (default) must be omitted")
	}
	if _, ok := tlvs[0x09]; ok {
		t.Fatal("http2 (default) must be omitted")
	}
	dnsVal := tlvs[0x0D][0]
	// [len][elem][len][elem]
	if int(dnsVal[0]) != 7 || string(dnsVal[1:8]) != "1.1.1.1" {
		t.Fatalf("dns list first elem wrong: %v", dnsVal)
	}

	if cfg.ClientDeepLink("", AuthPair{User: "u", Pass: "p"}, "") != "" {
		t.Fatal("empty address must yield empty link")
	}
}

func TestParseTrustTunnelMetrics(t *testing.T) {
	body := `# HELP client_sessions Number of active client sessions
# TYPE client_sessions gauge
client_sessions{protocol_type="http2"} 5
# HELP inbound_traffic_bytes Total number of bytes uploaded by clients
# TYPE inbound_traffic_bytes counter
inbound_traffic_bytes{protocol_type="http1"} 1000
inbound_traffic_bytes{protocol_type="http2"} 2345
# HELP outbound_traffic_bytes Total number of bytes downloaded by clients
# TYPE outbound_traffic_bytes counter
outbound_traffic_bytes{protocol_type="http2"} 7654321
outbound_tcp_sockets 12
`
	up, down, sessions, ok := parseTrustTunnelMetrics(body)
	if !ok {
		t.Fatal("parse must succeed")
	}
	if up != 3345 {
		t.Fatalf("up = %d", up)
	}
	if down != 7654321 {
		t.Fatalf("down = %d", down)
	}
	if sessions != 5 {
		t.Fatalf("sessions = %d", sessions)
	}
	if _, _, _, ok := parseTrustTunnelMetrics("# only comments\n"); ok {
		t.Fatal("no counters must yield ok=false")
	}
}

func TestTrustTunnelSoleClient(t *testing.T) {
	if got := TrustTunnelSoleClient(nil); got != "" {
		t.Fatalf("nil = %q", got)
	}
	if got := TrustTunnelSoleClient([]string{" ", ""}); got != "" {
		t.Fatalf("blank = %q", got)
	}
	if got := TrustTunnelSoleClient([]string{"a@b"}); got != "a@b" {
		t.Fatalf("one = %q", got)
	}
	if got := TrustTunnelSoleClient([]string{"a@b", "c@d"}); got != "" {
		t.Fatalf("two = %q", got)
	}
}

func TestTrustTunnelInstanceFromInbound(t *testing.T) {
	dir := t.TempDir()
	old := tunnelDir
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = old })

	certPath, keyPath := writeTestCert(t, dir, time.Now().Add(time.Hour), "vpn.example.com")
	secret := []byte("panel-secret")
	settings := `{"hostname":"vpn.example.com","listen":"0.0.0.0:8443","ipv6":true,"upstreamProtocol":"http2","certFile":"` +
		strings.ReplaceAll(certPath, `\`, `\\`) + `","keyFile":"` + strings.ReplaceAll(keyPath, `\`, `\\`) +
		`","clients":[{"email":"u@m","enable":true}]}`
	ib := &model.Inbound{Id: 5, Enable: true, Protocol: model.TrustTunnel, Settings: settings}

	inst, ok := TrustTunnelInstanceFromInbound(ib, secret, "", "")
	if !ok || !inst.Enabled {
		t.Fatalf("valid inbound must be enabled: ok=%v enabled=%v", ok, inst.Enabled)
	}
	if inst.Key != "trusttunnel-5" {
		t.Fatalf("key = %s", inst.Key)
	}
	if len(inst.ExtraFiles) != 3 {
		t.Fatalf("extra files = %d", len(inst.ExtraFiles))
	}
	if !strings.Contains(inst.ConfigText, "listen_address") {
		t.Fatal("vpn.toml missing")
	}
	if inst.ProbePort != 8443 {
		t.Fatalf("probe port = %d", inst.ProbePort)
	}

	// No clients → down.
	ibNoClients := &model.Inbound{Id: 6, Enable: true, Protocol: model.TrustTunnel, Settings: `{"hostname":"vpn.example.com","certFile":"` + strings.ReplaceAll(certPath, `\`, `\\`) + `","keyFile":"` + strings.ReplaceAll(keyPath, `\`, `\\`) + `"}`}
	inst, _ = TrustTunnelInstanceFromInbound(ibNoClients, secret, "", "")
	if inst.Enabled {
		t.Fatal("clientless inbound must be down")
	}

	// Foreign hostname vs cert SAN → down.
	ibBadHost := &model.Inbound{Id: 7, Enable: true, Protocol: model.TrustTunnel, Settings: `{"hostname":"other.example.com","certFile":"` + strings.ReplaceAll(certPath, `\`, `\\`) + `","keyFile":"` + strings.ReplaceAll(keyPath, `\`, `\\`) + `","clients":[{"email":"u@m","enable":true}]}`}
	inst, _ = TrustTunnelInstanceFromInbound(ibBadHost, secret, "", "")
	if inst.Enabled {
		t.Fatal("SAN mismatch must keep the instance down")
	}
}
