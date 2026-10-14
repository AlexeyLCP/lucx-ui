// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const acmeHelperPath = "../../../bin/lucx-acme.sh"

func httpRedirectSite(port int) string {
	return "\thandle /.well-known/acme-challenge/* {\n" +
		"\t\treverse_proxy 127.0.0.1:" + strconv.Itoa(port) + "\n" +
		"\t}\n" +
		"\thandle {\n\t\tredir https://{host}{uri} permanent\n\t}\n}\n"
}

func useAcmePortFile(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "acme-http-port")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prev := acmePortFile
	acmePortFile = func() string { return path }
	t.Cleanup(func() { acmePortFile = prev })
}

func TestAcmeHTTPChallengePort_FromFile(t *testing.T) {
	for content, want := range map[string]int{
		"":        defaultAcmeHTTPChallengePort,
		"31000\n": 31000,
		"80":      defaultAcmeHTTPChallengePort,
		"70000":   defaultAcmeHTTPChallengePort,
		"port":    defaultAcmeHTTPChallengePort,
	} {
		useAcmePortFile(t, content)
		if got := AcmeHTTPChallengePort(); got != want {
			t.Errorf("file %q: port %d, want %d", content, got, want)
		}
	}
}

func TestAcmeHTTPChallengePort_SameFileAndDefaultAsShellHelper(t *testing.T) {
	raw, err := os.ReadFile(acmeHelperPath)
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`(?m)^LUCX_ACME_DEFAULT_PORT=(\d+)$`).FindSubmatch(raw); m == nil || string(m[1]) != strconv.Itoa(defaultAcmeHTTPChallengePort) {
		t.Fatalf("bin/lucx-acme.sh default port %q != Go %d", m, defaultAcmeHTTPChallengePort)
	}
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := os.WriteFile(filepath.Join(dbDir, "acme-http-port"), []byte("31000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AcmeHTTPChallengePort(); got != 31000 {
		t.Fatalf("Go read %d from $XUI_DB_FOLDER/acme-http-port, want 31000", got)
	}
	e := newAcmeShellEnv(t, nil)
	if out, code := e.run(e.root, []string{"XUI_DB_FOLDER=" + dbDir}, "", "port"); code != 0 || strings.TrimSpace(out) != "31000" {
		t.Fatalf("shell port = %q (exit %d), want 31000", out, code)
	}
}

func TestRenderCoverCaddyfile_AcmeRelayOnHTTP(t *testing.T) {
	useAcmePortFile(t, "31000")
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{publicDir: "/var/www/site"})
	if want := ":80 {\n" + httpRedirectSite(31000); !strings.Contains(got, want) {
		t.Fatalf("missing :80 block\n%s\nin:\n%s", want, got)
	}
	if n := strings.Count(got, "acme-challenge"); n != 1 {
		t.Fatalf("acme relay must live only in the :80 block, found %d:\n%s", n, got)
	}
}

func TestRenderCoverCaddyfile_NoAcmeRelayWithoutHTTP(t *testing.T) {
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{publicDir: "/var/www/site", skipHTTP: true, httpsPort: 8443})
	if strings.Contains(got, "acme-challenge") {
		t.Fatalf("loopback cover has no :80, yet got an acme relay:\n%s", got)
	}
}

func TestRenderCaddyfileAcme_RelaysForeignChallenges(t *testing.T) {
	useAcmePortFile(t, "")
	cfg := DefaultNaiveConfig()
	cfg.UseAcme = true
	cfg.Domain = "n.example.org"
	cfg.AuthUser = "u"
	cfg.AuthPass = "p"

	got := cfg.RenderCaddyfile(nil, "")
	if want := "http:// {\n" + httpRedirectSite(defaultAcmeHTTPChallengePort); !strings.Contains(got, want) {
		t.Fatalf("missing http:// block\n%s\nin:\n%s", want, got)
	}

	cfg.Listen = "203.0.113.5"
	got = cfg.RenderCaddyfile(nil, "")
	if want := "http:// {\n\tbind \"203.0.113.5\"\n" + httpRedirectSite(defaultAcmeHTTPChallengePort); !strings.Contains(got, want) {
		t.Fatalf("http:// block must keep the inbound bind\n%s\nin:\n%s", want, got)
	}
}

func TestNoAcmeRelayWithoutPort80Site(t *testing.T) {
	manual := DefaultNaiveConfig()
	manual.Domain = "n.example.org"
	manual.CertFile = "/c.pem"
	manual.KeyFile = "/k.pem"
	cases := map[string]string{
		"naive manual tls": manual.RenderCaddyfile(nil, ""),
		"tproxy":           RenderTproxyCaddyfile("proxy.example.com", 443, "/c.pem", "/k.pem", 24002, false, nil),
		"unified gateway": RenderUnifiedGatewayCaddyfile(443, []GatewayRoute{{SNI: "shop.example.com", Chan: "cover-1"}}, "cover-1", "",
			[]string{RenderCoverSite("shop.example.com", "/c.pem", "/k.pem", coverAttach{bind: "l4chan/cover-1"}), manual.RenderSite("naive-2", nil, "")}),
	}
	for name, got := range cases {
		if strings.Contains(got, "acme-challenge") || strings.Contains(got, "http://") {
			t.Errorf("%s: no :80 listener today, so no acme relay expected:\n%s", name, got)
		}
	}
}

// acmeShellEnv runs a copy of bin/lucx-acme.sh with stubbed ss/ip/socat, so the
// helper's tunnel/ dir, acme.sh home and the ACME port file are all temporary.
type acmeShellEnv struct {
	t                          *testing.T
	root, bin, stubs, acme, db string
	socatLog                   string
}

func newAcmeShellEnv(t *testing.T, listeners map[int]string) *acmeShellEnv {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	root := t.TempDir()
	e := &acmeShellEnv{t: t, root: root, bin: filepath.Join(root, "bin"), stubs: filepath.Join(root, "stubs"), acme: filepath.Join(root, "acme"), db: filepath.Join(root, "etc")}
	e.socatLog = filepath.Join(root, "socat.log")
	for _, d := range []string{e.bin, filepath.Join(e.bin, "tunnel"), e.stubs, e.acme, e.db} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src, err := os.ReadFile(acmeHelperPath)
	if err != nil {
		t.Fatal(err)
	}
	e.write(filepath.Join(e.bin, "lucx-acme.sh"), string(src))
	ss := "#!/bin/sh\necho 'State Recv-Q Send-Q Local Address:Port Peer Address:Port Process'\n"
	for port, owner := range listeners {
		ss += "echo 'LISTEN 0 4096 *:" + strconv.Itoa(port) + " *:* users:((\"" + owner + "\",pid=42,fd=7))'\n"
	}
	e.write(filepath.Join(e.stubs, "ss"), ss)
	e.write(filepath.Join(e.stubs, "ip"), "#!/bin/sh\necho '    inet 203.0.113.5/24 scope global eth0'\n")
	e.write(filepath.Join(e.stubs, "socat"), "#!/bin/sh\necho \"$*\" >> '"+e.socatLog+"'\nexec sleep 60\n")
	return e
}

func (e *acmeShellEnv) write(path, body string) {
	e.t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		e.t.Fatal(err)
	}
}

func (e *acmeShellEnv) caddyRelay(port int) {
	e.write(filepath.Join(e.bin, "tunnel", "cover-1.caddyfile"), ":80 {\n"+httpRedirectSite(port))
}

func (e *acmeShellEnv) domainConf(dir, domain, body string) string {
	e.t.Helper()
	d := filepath.Join(e.acme, dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		e.t.Fatal(err)
	}
	p := filepath.Join(d, domain+".conf")
	e.write(p, body)
	return p
}

func (e *acmeShellEnv) run(dir string, env []string, stdin string, args ...string) (string, int) {
	e.t.Helper()
	cmd := exec.CommandContext(context.Background(), "bash", append([]string{filepath.Join(e.bin, "lucx-acme.sh")}, args...)...)
	return e.exec(cmd, dir, env, stdin)
}

func (e *acmeShellEnv) source(env []string, stdin, script string) (string, int) {
	e.t.Helper()
	cmd := exec.CommandContext(context.Background(), "bash", "-c", "source '"+filepath.Join(e.bin, "lucx-acme.sh")+"'\n"+script)
	return e.exec(cmd, e.root, env, stdin)
}

func (e *acmeShellEnv) exec(cmd *exec.Cmd, dir string, env []string, stdin string) (string, int) {
	e.t.Helper()
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append([]string{"PATH=" + e.stubs + ":/usr/bin:/bin", "LE_WORKING_DIR=" + e.acme, "HOME=" + e.root, "XUI_DB_FOLDER=" + e.db}, env...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return string(out), 0
}

func (e *acmeShellEnv) socatCalls() []string {
	raw, _ := os.ReadFile(e.socatLog)
	return strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(raw)), " ", "|"))
}

// waitSocatCalls lets the nohup'ed stub write its log line before we read it.
func (e *acmeShellEnv) waitSocatCalls(n int) []string {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if calls := e.socatCalls(); len(calls) >= n {
			return calls
		}
		time.Sleep(20 * time.Millisecond)
	}
	return e.socatCalls()
}

func readConf(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLucxAcmeHelper_PreHookForwardsFreePort80AndPostHookStopsIt(t *testing.T) {
	e := newAcmeShellEnv(t, nil)
	conf := e.domainConf("example.com_ecc", "example.com", "Le_Webroot='no'\nLe_HTTPPort='29380'\n")
	dir := filepath.Dir(conf)
	if out, code := e.run(dir, []string{"Le_Domain=example.com"}, "", "pre-hook"); code != 0 {
		t.Fatalf("pre-hook exit %d:\n%s", code, out)
	}
	want := map[string]bool{
		"TCP4-LISTEN:80,reuseaddr,fork|TCP4:127.0.0.1:29380":            true,
		"TCP6-LISTEN:80,ipv6only=1,reuseaddr,fork|TCP4:127.0.0.1:29380": true,
	}
	calls := e.waitSocatCalls(2)
	if len(calls) != 2 || !want[calls[0]] || !want[calls[1]] || calls[0] == calls[1] {
		t.Fatalf("socat calls %q, want v4 and v6 :80 forwards to 29380", calls)
	}
	pidsRaw, err := os.ReadFile(filepath.Join(dir, ".lucx-acme-forward.pids"))
	if err != nil {
		t.Fatal(err)
	}
	pids := strings.Fields(string(pidsRaw))
	if out, code := e.run(dir, []string{"Le_Domain=example.com"}, "", "post-hook"); code != 0 {
		t.Fatalf("post-hook exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lucx-acme-forward.pids")); !os.IsNotExist(err) {
		t.Fatalf("post-hook left the pid file: %v", err)
	}
	for _, p := range pids {
		pid, _ := strconv.Atoi(p)
		deadline := time.Now().Add(3 * time.Second)
		for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if syscall.Kill(pid, 0) == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("forwarder %d still running after post-hook", pid)
		}
	}
}

func TestLucxAcmeHelper_PreHookWithPort80Owners(t *testing.T) {
	cases := []struct {
		name      string
		listeners map[int]string
		relay     int
		env       []string
		conf      string
		wantExit  int
		wantSocat []string
		wantOut   string
	}{
		{name: "LucX Caddy relays to our port", listeners: map[int]string{80: "caddy-naive-lin"}, relay: 29380, conf: "29380"},
		{
			name: "Caddy not reconciled yet after a port change", listeners: map[int]string{80: "caddy-naive-lin"}, relay: 29380, conf: "31000",
			wantSocat: []string{"TCP4-LISTEN:29380,bind=127.0.0.1,reuseaddr,fork|TCP4:127.0.0.1:31000"},
		},
		{
			name: "first issue passes the port in env", listeners: map[int]string{80: "caddy-layer4-li"}, relay: 29380, env: []string{"LUCX_ACME_LISTEN_PORT=31000"},
			wantSocat: []string{"TCP4-LISTEN:29380,bind=127.0.0.1,reuseaddr,fork|TCP4:127.0.0.1:31000"},
		},
		{name: "LucX Caddy without relay", listeners: map[int]string{80: "caddy-layer4-li"}, conf: "29380", wantExit: 1, wantOut: "without an ACME relay"},
		{name: "foreign owner", listeners: map[int]string{80: "nginx"}, relay: 29380, conf: "29380", wantExit: 1, wantOut: "held by 'nginx'"},
		{name: "plain :80 cert left alone", listeners: map[int]string{80: "nginx"}, conf: "80"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newAcmeShellEnv(t, tc.listeners)
			if tc.relay != 0 {
				e.caddyRelay(tc.relay)
			}
			body := "Le_Webroot='no'\n"
			if tc.conf != "" {
				body += "Le_HTTPPort='" + tc.conf + "'\n"
			}
			conf := e.domainConf("example.com", "example.com", body)
			out, code := e.run(filepath.Dir(conf), append([]string{"Le_Domain=example.com"}, tc.env...), "", "pre-hook")
			if code != tc.wantExit || !strings.Contains(out, tc.wantOut) {
				t.Fatalf("exit %d output %q, want exit %d containing %q", code, out, tc.wantExit, tc.wantOut)
			}
			if got := e.waitSocatCalls(len(tc.wantSocat)); strings.Join(got, "\n") != strings.Join(tc.wantSocat, "\n") {
				t.Fatalf("socat calls %q, want %q", got, tc.wantSocat)
			}
			_, _ = e.run(filepath.Dir(conf), nil, "", "post-hook")
		})
	}
}

func decodeAcmeHook(t *testing.T, conf, key string) string {
	t.Helper()
	m := regexp.MustCompile(key + `='__ACME_BASE64__START_([A-Za-z0-9+/=]+)__ACME_BASE64__END_'`).FindStringSubmatch(readConf(t, conf))
	if m == nil {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLucxAcmeHelper_MigrateAndRestore(t *testing.T) {
	e := newAcmeShellEnv(t, nil)
	e.write(filepath.Join(e.db, "acme-http-port"), "31000\n")
	ourHook := "__ACME_BASE64__START_" + base64.StdEncoding.EncodeToString([]byte("bash '/old/bin/lucx-acme.sh' pre-hook")) + "__ACME_BASE64__END_"
	plain := e.domainConf("a.example.com", "a.example.com", "Le_Webroot='no'\nLe_HTTPPort='80'\nLe_Listen_V6='1'\n")
	ecc := e.domainConf("b.example.com_ecc", "b.example.com", "Le_Webroot='no'\n")
	moved := e.domainConf("c.example.com", "c.example.com", "Le_Webroot='no'\nLe_HTTPPort='29380'\nLe_PreHook='"+ourHook+"'\n")
	own := e.domainConf("d.example.com", "d.example.com", "Le_Webroot='no'\nLe_PreHook='__ACME_BASE64__START_c3RvcA==__ACME_BASE64__END_'\n")
	webroot := e.domainConf("e.example.com", "e.example.com", "Le_Webroot='/var/www'\n")
	custom := e.domainConf("f.example.com", "f.example.com", "Le_Webroot='no'\nLe_HTTPPort='8080'\n")
	untouched := map[string]string{own: readConf(t, own), webroot: readConf(t, webroot), custom: readConf(t, custom)}

	out, code := e.run(e.root, nil, "", "migrate")
	if code != 0 || !strings.Contains(out, "3 standalone certificate(s)") {
		t.Fatalf("migrate exit %d output %q", code, out)
	}
	self := filepath.Join(e.bin, "lucx-acme.sh")
	for _, p := range []string{plain, ecc, moved} {
		got := readConf(t, p)
		for _, w := range []string{"Le_HTTPPort='31000'", "Le_LocalAddress='127.0.0.1'", "Le_Listen_V4='1'"} {
			if !strings.Contains(got, w) {
				t.Errorf("%s missing %q:\n%s", p, w, got)
			}
		}
		if strings.Contains(got, "Le_Listen_V6") {
			t.Errorf("%s keeps Le_Listen_V6 next to a 127.0.0.1 bind:\n%s", p, got)
		}
		if decodeAcmeHook(t, p, "Le_PreHook") != "[ ! -f '"+self+"' ] || bash '"+self+"' pre-hook" ||
			decodeAcmeHook(t, p, "Le_PostHook") != "[ ! -f '"+self+"' ] || bash '"+self+"' post-hook" {
			t.Errorf("%s hooks not ours:\n%s", p, got)
		}
	}
	for p, before := range untouched {
		if got := readConf(t, p); got != before {
			t.Errorf("%s changed:\n%s", p, got)
		}
	}
	if out, _ := e.run(e.root, nil, "", "migrate"); out != "" {
		t.Fatalf("second migrate must be a no-op, got %q", out)
	}

	out, code = e.run(e.root, nil, "", "restore")
	if code != 0 || !strings.Contains(out, "3 certificate(s)") {
		t.Fatalf("restore exit %d output %q", code, out)
	}
	for _, p := range []string{plain, ecc, moved} {
		got := readConf(t, p)
		for _, gone := range []string{"Le_HTTPPort", "Le_LocalAddress", "Le_Listen_V4", "Le_PreHook", "Le_PostHook"} {
			if strings.Contains(got, gone) {
				t.Errorf("restore left %s in %s:\n%s", gone, p, got)
			}
		}
	}
	for p, before := range untouched {
		if got := readConf(t, p); got != before {
			t.Errorf("restore changed %s:\n%s", p, got)
		}
	}
}

func TestLucxAcmeHelper_PromptRejectsPort80AndSavesChoice(t *testing.T) {
	e := newAcmeShellEnv(t, map[int]string{80: "caddy-naive-lin", 31001: "sshd"})
	e.caddyRelay(29380)
	ourHook := "__ACME_BASE64__START_" + base64.StdEncoding.EncodeToString([]byte("lucx-acme.sh pre-hook")) + "__ACME_BASE64__END_"
	conf := e.domainConf("a.example.com", "a.example.com", "Le_Webroot='no'\nLe_HTTPPort='29380'\nLe_PreHook='"+ourHook+"'\nLe_PostHook='x'\n")

	out, code := e.source(nil, "80\n31001\n31000\n", `WebPort=""; lucx_acme_prompt_port WebPort || exit 7; echo "PORT=$WebPort"; printf 'ARG=%s\n' "${LUCX_ACME_ARGS[@]}"`)
	if code != 0 {
		t.Fatalf("prompt exit %d:\n%s", code, out)
	}
	for _, w := range []string{
		"Port 80 is forwarded to it automatically", "Port 80 is forwarded by LucX-UI itself", "Port 31001 is already used by 'sshd'",
		"PORT=31000", "ARG=--local-address\nARG=127.0.0.1\nARG=--listen-v4\nARG=--pre-hook", "ARG=--post-hook",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q:\n%s", w, out)
		}
	}
	if raw, _ := os.ReadFile(filepath.Join(e.db, "acme-http-port")); strings.TrimSpace(string(raw)) != "31000" {
		t.Fatalf("port file = %q, want 31000", raw)
	}
	if got := readConf(t, conf); !strings.Contains(got, "Le_HTTPPort='31000'") {
		t.Fatalf("existing LucX cert not moved to the new port:\n%s", got)
	}
}

func TestLucxAcmeHelper_PromptRefusesForeignPort80(t *testing.T) {
	e := newAcmeShellEnv(t, map[int]string{80: "nginx"})
	out, code := e.source(nil, "\n", `WebPort=""; lucx_acme_prompt_port WebPort`)
	if code != 1 || !strings.Contains(out, "held by 'nginx'") {
		t.Fatalf("exit %d output %q, want refusal naming nginx", code, out)
	}
}

func TestLucxAcmeHelper_PromptNonInteractiveIgnoresPort80(t *testing.T) {
	e := newAcmeShellEnv(t, nil)
	out, code := e.source([]string{"NONINTERACTIVE=1", "XUI_ACME_HTTP_PORT=80"}, "", `WebPort=""; lucx_acme_prompt_port WebPort && echo "PORT=$WebPort"`)
	if code != 0 || !strings.Contains(out, "PORT=29380") || !strings.Contains(out, "XUI_ACME_HTTP_PORT=80 is not usable") {
		t.Fatalf("exit %d output %q", code, out)
	}
}
