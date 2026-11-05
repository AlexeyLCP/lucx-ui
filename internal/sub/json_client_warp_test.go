// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientwarp"
)

var testWarpCfg = &clientwarp.Config{
	Domains:   []string{"geosite:youtube", "domain:ytimg.com"},
	Endpoints: []string{"162.159.192.1:4500", "8.39.214.9:4177"},
}

var testWarpAcc = &model.ClientWarpAccount{SubId: "s1", PrivateKey: "cHJpdg==", AddressV4: "172.16.0.2"}

func injectDoc(t *testing.T, in string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(injectClientWarp([]byte(in), testWarpCfg, testWarpAcc), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func docRules(doc map[string]any) []any {
	return doc["routing"].(map[string]any)["rules"].([]any)
}

func ruleJSON(r any) string {
	b, _ := json.Marshal(r)
	return string(b)
}

func TestInjectClientWarp_SplitsProxiedRule(t *testing.T) {
	doc := injectDoc(t, `{"outbounds":[{"tag":"proxy"},{"tag":"direct"}],"routing":{"rules":[
		{"type":"field","domain":["geosite:category-ads"],"outboundTag":"block"},
		{"type":"field","domain":["geosite:github","geosite:youtube","domain:ytimg.com"],"outboundTag":"proxy"},
		{"type":"field","network":"tcp,udp","outboundTag":"proxy"}]}}`)

	rules := docRules(doc)
	want := []string{
		`{"domain":["geosite:category-ads"],"outboundTag":"block","type":"field"}`,
		`{"balancerTag":"client-warp","domain":["geosite:youtube","domain:ytimg.com"],"type":"field"}`,
		`{"domain":["geosite:github"],"outboundTag":"proxy","type":"field"}`,
		`{"network":"tcp,udp","outboundTag":"proxy","type":"field"}`,
	}
	if len(rules) != len(want) {
		t.Fatalf("rules = %d, want %d: %v", len(rules), len(want), rules)
	}
	for i := range want {
		if got := ruleJSON(rules[i]); got != want[i] {
			t.Fatalf("rule %d = %s, want %s", i, got, want[i])
		}
	}

	balancers := doc["routing"].(map[string]any)["balancers"].([]any)
	if got := ruleJSON(balancers[0]); got != `{"fallbackTag":"proxy","selector":["client-warp"],"strategy":{"type":"leastPing"},"tag":"client-warp"}` {
		t.Fatalf("balancer = %s", got)
	}
	outbounds := doc["outbounds"].([]any)
	if len(outbounds) != 4 {
		t.Fatalf("outbounds = %d, want 4", len(outbounds))
	}
	warp2 := outbounds[3].(map[string]any)
	settings := warp2["settings"].(map[string]any)
	peer := settings["peers"].([]any)[0].(map[string]any)
	if warp2["tag"] != "client-warp-2" || peer["endpoint"] != "8.39.214.9:4177" ||
		settings["secretKey"] != "cHJpdg==" || settings["address"].([]any)[0] != "172.16.0.2/32" ||
		settings["mtu"] != float64(clientwarp.DefaultMTU) || peer["publicKey"] != clientwarp.PeerPublicKey {
		t.Fatalf("second WARP outbound = %v", warp2)
	}
	if _, ok := warp2["streamSettings"].(map[string]any)["finalmask"]; !ok {
		t.Fatalf("WARP outbound has no finalmask: %v", warp2["streamSettings"])
	}
	obs := doc["observatory"].(map[string]any)
	if ruleJSON(obs["subjectSelector"]) != `["client-warp"]` {
		t.Fatalf("observatory = %v", obs)
	}
}

// A rule left without domains must disappear: with no matcher it would catch all traffic.
func TestInjectClientWarp_DropsEmptiedRuleKeepsOtherMatchers(t *testing.T) {
	doc := injectDoc(t, `{"outbounds":[{"tag":"proxy"}],"routing":{"rules":[
		{"type":"field","domain":["geosite:youtube"],"outboundTag":"proxy"},
		{"type":"field","domain":["domain:ytimg.com"],"port":"443","outboundTag":"proxy"}]}}`)
	rules := docRules(doc)
	want := []string{
		`{"balancerTag":"client-warp","domain":["geosite:youtube"],"type":"field"}`,
		`{"balancerTag":"client-warp","domain":["domain:ytimg.com"],"type":"field"}`,
		`{"outboundTag":"proxy","port":"443","type":"field"}`,
	}
	if len(rules) != len(want) {
		t.Fatalf("rules = %v", rules)
	}
	for i := range want {
		if got := ruleJSON(rules[i]); got != want[i] {
			t.Fatalf("rule %d = %s, want %s", i, got, want[i])
		}
	}
}

func TestInjectClientWarp_LeavesDocumentAlone(t *testing.T) {
	tests := map[string]string{
		"routed direct":    `{"outbounds":[{"tag":"proxy"}],"routing":{"rules":[{"type":"field","domain":["geosite:youtube"],"outboundTag":"direct"}]}}`,
		"blocked":          `{"outbounds":[{"tag":"proxy"}],"routing":{"rules":[{"type":"field","domain":["geosite:youtube"],"outboundTag":"block"}]}}`,
		"not mentioned":    `{"outbounds":[{"tag":"proxy"}],"routing":{"rules":[{"type":"field","network":"tcp,udp","outboundTag":"proxy"}]}}`,
		"already injected": `{"outbounds":[{"tag":"proxy"},{"tag":"client-warp"}],"routing":{"rules":[{"type":"field","domain":["geosite:youtube"],"outboundTag":"proxy"}]}}`,
		"no routing":       `{"outbounds":[{"tag":"proxy"}]}`,
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			if got := string(injectClientWarp([]byte(in), testWarpCfg, testWarpAcc)); got != in {
				t.Fatalf("document changed:\n%s", got)
			}
		})
	}
}

// Balancer documents already run a burstObservatory; a second observer would
// make xray refuse the config, so the WARP selector joins the existing one.
func TestInjectClientWarp_BalancerDocumentSharesBurstObservatory(t *testing.T) {
	doc := injectDoc(t, `{"outbounds":[{"tag":"bal-1-vless"},{"tag":"bal-1-vless-2"}],
		"burstObservatory":{"subjectSelector":["bal-1-"],"pingConfig":{"interval":"1m"}},
		"routing":{"balancers":[{"tag":"balancer","selector":["bal-1-"]}],
		"rules":[{"type":"field","domain":["geosite:youtube","geosite:x"],"balancerTag":"balancer"}]}}`)
	if _, has := doc["observatory"]; has {
		t.Fatal("a second observer was added")
	}
	if got := ruleJSON(doc["burstObservatory"].(map[string]any)["subjectSelector"]); got != `["bal-1-","client-warp"]` {
		t.Fatalf("burstObservatory selector = %s", got)
	}
	balancers := doc["routing"].(map[string]any)["balancers"].([]any)
	if len(balancers) != 2 || balancers[1].(map[string]any)["fallbackTag"] != "bal-1-vless" {
		t.Fatalf("balancers = %v", balancers)
	}
}

// fakeWarpAPI counts registrations and answers like api.cloudflareclient.com.
func fakeWarpAPI(t *testing.T, status int) *atomic.Int32 {
	t.Helper()
	var regs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/reg":
			regs.Add(1)
			body, _ := io.ReadAll(r.Body)
			var req map[string]any
			if json.Unmarshal(body, &req) != nil || req["key"] == "" {
				t.Errorf("registration without a public key: %s", body)
			}
			w.WriteHeader(status)
			if status == http.StatusOK {
				_, _ = io.WriteString(w, `{"id":"acc-1","token":"tok","config":{"interface":{"addresses":{"v4":"172.16.0.2"}}}}`)
			}
		case r.Method == http.MethodPatch && r.URL.Path == "/reg/acc-1":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("PATCH without the account token")
			}
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	oldURL, oldClient := clientWarpRegURL, clientWarpHTTPClient
	clientWarpRegURL, clientWarpHTTPClient = srv.URL+"/reg", srv.Client()
	t.Cleanup(func() { clientWarpRegURL, clientWarpHTTPClient = oldURL, oldClient })
	return &regs
}

const (
	clientWarpRules   = `[{"type":"field","domain":["geosite:youtube"],"outboundTag":"proxy"}]`
	clientWarpSetting = `{"domains":["geosite:youtube"],"endpoints":["162.159.192.1:4500"]}`
)

func TestSubJsonClientWarp_RegistersOncePerSubscription(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4911, 1, dnsTestStream)
	regs := fakeWarpAPI(t, http.StatusOK)

	js := NewSubJsonService("", clientWarpRules, "", "", NewSubService(""))
	js.SetClientWarpConfig(clientWarpSetting)

	first := onlySubJsonDoc(t, js, "s1")
	var acc model.ClientWarpAccount
	if err := database.GetDB().Where("sub_id = ?", "s1").First(&acc).Error; err != nil {
		t.Fatalf("account not stored: %v", err)
	}
	warp := first["outbounds"].([]any)[len(first["outbounds"].([]any))-1].(map[string]any)
	if warp["tag"] != "client-warp" || warp["settings"].(map[string]any)["secretKey"] != acc.PrivateKey {
		t.Fatalf("document does not use the stored account: %v", warp)
	}

	// A rebuilt service (settings change, restart) must reuse the stored account.
	js2 := NewSubJsonService("", clientWarpRules, "", "", NewSubService(""))
	js2.SetClientWarpConfig(clientWarpSetting)
	second := onlySubJsonDoc(t, js2, "s1")
	if got := regs.Load(); got != 1 {
		t.Fatalf("registrations = %d, want 1", got)
	}
	warp2 := second["outbounds"].([]any)[len(second["outbounds"].([]any))-1].(map[string]any)
	if warp2["settings"].(map[string]any)["secretKey"] != acc.PrivateKey {
		t.Fatal("second request used a different key")
	}
}

func TestSubJsonClientWarp_FailedRegistrationServesPlainDocsAndBacksOff(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4912, 1, dnsTestStream)
	regs := fakeWarpAPI(t, http.StatusTooManyRequests)

	js := NewSubJsonService("", clientWarpRules, "", "", NewSubService(""))
	js.SetClientWarpConfig(clientWarpSetting)
	for range 2 {
		doc := onlySubJsonDoc(t, js, "s1")
		if strings.Contains(ruleJSON(doc["outbounds"]), "client-warp") {
			t.Fatalf("WARP injected without an account: %v", doc["outbounds"])
		}
		if got := ruleJSON(docRules(doc)[0]); !strings.Contains(got, `"outboundTag":"proxy"`) {
			t.Fatalf("proxy rule changed: %s", got)
		}
	}
	if got := regs.Load(); got != 1 {
		t.Fatalf("registrations = %d, want 1 (retry must wait)", got)
	}
}

func TestSubJsonClientWarp_OffByDefault(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4913, 1, dnsTestStream)
	regs := fakeWarpAPI(t, http.StatusOK)

	for _, setting := range []string{"", `{"domains":[]}`} {
		js := NewSubJsonService("", clientWarpRules, "", "", NewSubService(""))
		js.SetClientWarpConfig(setting)
		if doc := onlySubJsonDoc(t, js, "s1"); strings.Contains(ruleJSON(doc), "client-warp") {
			t.Fatalf("setting %q injected WARP", setting)
		}
	}
	if got := regs.Load(); got != 0 {
		t.Fatalf("registrations = %d, want 0", got)
	}
}
