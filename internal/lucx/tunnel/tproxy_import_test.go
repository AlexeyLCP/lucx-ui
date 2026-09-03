// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestDiscoverTproxyAt(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"listen":"127.0.0.1:9","public_hostname":"dns.example.com","public_upstream":"http://127.0.0.1:89","profiles_file":"` + filepath.ToSlash(filepath.Join(dir, "profiles.json")) + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles := `{"profiles":[{"name":"dns","secret":"000102030405060708090a0b0c0d0e0f","backend":"127.0.0.1:2398","carrier_mode":"https"}]}`
	if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(profiles), 0o600); err != nil {
		t.Fatal(err)
	}
	got := DiscoverTproxyAt(dir)
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].Hostname != "dns.example.com" || got[0].Secret != "000102030405060708090a0b0c0d0e0f" || got[0].Upstream != "http://127.0.0.1:89" {
		t.Fatalf("%+v", got[0])
	}
	c := got[0].Config()
	if !c.ExternalTLS || c.SiteSource != "upstream" || c.CarrierMode != "https" {
		t.Fatalf("%+v", c)
	}
}

func TestTproxyExternalTLSSkipsSidecars(t *testing.T) {
	ib := &model.Inbound{
		Id:       3,
		Protocol: model.Tproxy,
		Enable:   true,
		Port:     443,
		Settings: `{"hostname":"dns.example.com","secret":"000102030405060708090a0b0c0d0e0f","externalTLS":true,"siteSource":"upstream","siteUpstream":"http://127.0.0.1:89"}`,
	}
	insts, ok := TproxyInstancesFromInbound(ib, "", "")
	if !ok || len(insts) != 3 {
		t.Fatalf("ok=%v n=%d", ok, len(insts))
	}
	for _, inst := range insts {
		if inst.Enabled {
			t.Fatalf("external TLS must not start %s", inst.Key)
		}
	}
}
