package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

func TestNewCsqttMemberAPIUsesLocalAndNodeHosts(t *testing.T) {
	local, err := newCsqttMemberAPI(&model.Inbound{Protocol: model.Csqtt}, tunnel.CsqttConfig{SubHost: "public.example", WebPass: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(local.base, "127.0.0.1:46002") {
		t.Fatalf("local API base = %q", local.base)
	}

	nodeID := 7
	node, err := newCsqttMemberAPI(&model.Inbound{Protocol: model.Csqtt, NodeID: &nodeID}, tunnel.CsqttConfig{SubHost: "node.example", WebPass: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if node.base != "https://node.example:46002" || node.host != "node.example" {
		t.Fatalf("node API = base %q host %q", node.base, node.host)
	}
}

func TestCsqttMemberAPIUsesAPIKeyAndExpectedEndpoints(t *testing.T) {
	var calls []string
	members := map[string]csqttMemberServerClient{
		"member-pass": {Name: "alice", Password: "member-pass", Active: true, Link: "csqtt://member"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "web-secret" {
			t.Fatalf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case http.MethodGet + " /api/clients":
			list := make([]csqttMemberServerClient, 0, len(members))
			for _, member := range members {
				list = append(list, member)
			}
			_ = json.NewEncoder(w).Encode(list)
		case http.MethodPost + " /api/clients":
			var request csqttMemberClientRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			members["new-pass"] = csqttMemberServerClient{Name: request.Name, Password: "new-pass", Active: true, Link: "csqtt://new"}
			_ = json.NewEncoder(w).Encode(csqttMemberCreated{Password: "new-pass", Link: "csqtt://new"})
		case http.MethodPost + " /api/clients/member-pass/toggle":
			member := members["member-pass"]
			member.Active = !member.Active
			members["member-pass"] = member
		case http.MethodDelete + " /api/clients/member-pass":
			delete(members, "member-pass")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	api := &csqttMemberAPI{base: server.URL, key: "web-secret", client: server.Client()}
	clients, err := api.list(context.Background())
	if err != nil || len(clients) != 1 || clients[0].Link != "csqtt://member" {
		t.Fatalf("list = %+v, err=%v", clients, err)
	}
	created, err := api.create(context.Background(), "bob", "hash-a,hash-b")
	if err != nil || created.Password != "new-pass" {
		t.Fatalf("create = %+v, err=%v", created, err)
	}
	if err := api.toggle(context.Background(), "member-pass"); err != nil {
		t.Fatal(err)
	}
	if members["member-pass"].Active {
		t.Fatal("toggle did not change member state")
	}
	if err := api.delete(context.Background(), "member-pass"); err != nil {
		t.Fatal(err)
	}
	if _, ok := members["member-pass"]; ok {
		t.Fatal("delete did not remove member")
	}
	clients, err = api.list(context.Background())
	if err != nil || len(clients) != 1 || clients[0].Password != "new-pass" {
		t.Fatalf("final list = %+v, err=%v", clients, err)
	}
	got := strings.Join(calls, ",")
	for _, want := range []string{"GET /api/clients", "POST /api/clients", "POST /api/clients/member-pass/toggle", "DELETE /api/clients/member-pass"} {
		if !strings.Contains(got, want) {
			t.Errorf("calls %q missing %q", got, want)
		}
	}
}

func TestCsqttMemberAPIFallsBackOnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	api := &csqttMemberAPI{base: server.URL, key: "wrong", client: server.Client()}
	if _, err := api.list(context.Background()); err == nil {
		t.Fatal("list succeeded on unauthorized API")
	}
}
