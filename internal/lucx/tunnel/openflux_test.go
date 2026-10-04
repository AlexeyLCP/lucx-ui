// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestOpenfluxConfAndLink(t *testing.T) {
	cfg := OpenfluxConfig{
		Secret:    "0123456789abcdef",
		YandexURL: "https://docs.example/view?id=1",
	}
	if err := cfg.Validate(18445); err != nil {
		t.Fatal(err)
	}
	conf := cfg.RenderConf("openflux-3", 18445)
	if !strings.Contains(conf, "Mode = l4") || !strings.Contains(conf, "Listen = 0.0.0.0:18445") {
		t.Fatalf("conf missing l4 listen:\n%s", conf)
	}
	if !strings.Contains(conf, "Type = yandex") || strings.Contains(conf, "Mode = l3") {
		t.Fatalf("conf shape:\n%s", conf)
	}
	link, err := cfg.ClientURI("exit", "203.0.113.5", 18445)
	if err != nil || !strings.HasPrefix(link, openfluxLinkPrefix) {
		t.Fatalf("link %q err %v", link, err)
	}
	body, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, openfluxLinkPrefix))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	var got openfluxShare
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Negotiate || got.Secret != cfg.Secret || got.Context != cfg.YandexURL {
		t.Fatalf("payload %+v", got)
	}
	if len(got.Transports) != 2 || got.Transports[0].Dial != "203.0.113.5:18445" {
		t.Fatalf("transports %+v", got.Transports)
	}
}

func TestOpenfluxStoredBlockDecodes(t *testing.T) {
	raw := []byte(`{"negotiate":true,"secret":"0123456789abcdef","context":"http://openflux","transports":[{"type":"direct","priority":100,"dial":"1.2.3.4:18445"}]}`)
	packed := openfluxStoredDeflate(raw)
	got, err := io.ReadAll(flate.NewReader(bytes.NewReader(packed)))
	if err != nil || string(got) != string(raw) {
		t.Fatalf("stored block %q err %v", got, err)
	}
}

func openfluxStoredDeflate(data []byte) []byte {
	var out []byte
	for offset := 0; offset < len(data) || offset == 0; {
		n := len(data) - offset
		if n > 65535 {
			n = 65535
		}
		final := byte(0)
		if offset+n >= len(data) {
			final = 1
		}
		nlen := n ^ 0xffff
		out = append(out, final, byte(n), byte(n>>8), byte(nlen), byte(nlen>>8))
		out = append(out, data[offset:offset+n]...)
		offset += n
		if len(data) == 0 {
			break
		}
	}
	return out
}

func TestOpenfluxInstanceDisabledWithoutSecret(t *testing.T) {
	ib := &model.Inbound{Id: 4, Protocol: model.Openflux, Enable: true, Port: 18445, Settings: `{}`}
	inst, ok := OpenfluxInstanceFromInbound(ib)
	if !ok || inst.Enabled || inst.Key != "openflux-4" {
		t.Fatalf("empty secret must not start: %+v ok=%v", inst, ok)
	}
}

func TestOpenfluxRejectsHashInURL(t *testing.T) {
	cfg := OpenfluxConfig{Secret: "0123456789abcdef", YandexURL: "https://docs.example/a#b"}
	if err := cfg.Validate(18445); err == nil {
		t.Fatal("hash in URL must be rejected (conf parser cuts at #)")
	}
}
