// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestOlcrtcKey(t *testing.T) {
	if got := OlcrtcKey(5); got != "olcrtc-5" {
		t.Fatalf("got %q", got)
	}
}

func TestOlcrtcInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id: 2, Enable: true, Protocol: model.Olcrtc, Remark: "rtc",
		Settings: `{"provider":"jitsi","roomId":"https://meet.jit.si/r","cryptoKey":"` + strings.Repeat("a", 64) + `","transport":"datachannel","dns":"8.8.8.8:53"}`,
	}
	inst, ok := OlcrtcInstanceFromInbound(ib)
	if !ok || !inst.Enabled {
		t.Fatal("expected enabled")
	}
	if inst.ManageKey() != "olcrtc-2" {
		t.Fatalf("key %q", inst.ManageKey())
	}
	if !strings.Contains(inst.ConfigText, "mode: srv") {
		t.Fatalf("yaml:\n%s", inst.ConfigText)
	}
	if !strings.Contains(inst.ConfigText, "jitsi") {
		t.Fatal("missing provider")
	}
}

func TestQwdttInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id: 1, Enable: true, Protocol: model.Qwdtt, Port: 56000,
		Settings: `{"listenAddr":"0.0.0.0:56000","wgPort":56001,"password":"secret","dns":"1.1.1.1","subHost":"1.2.3.4:56000"}`,
	}
	inst, ok := QwdttInstanceFromInbound(ib)
	if !ok || !inst.Enabled {
		t.Fatal("expected enabled")
	}
	if inst.ManageKey() != QwdttKey {
		t.Fatalf("key %q", inst.ManageKey())
	}
	if len(inst.Args) == 0 {
		t.Fatal("expected CLI args")
	}
	joined := strings.Join(inst.Args, " ")
	if !strings.Contains(joined, "-listen") || !strings.Contains(joined, "secret") {
		t.Fatalf("args %v", inst.Args)
	}
}

func TestQwdttSingleKey(t *testing.T) {
	// Two different inbound ids still map to the same manager key.
	a := &model.Inbound{Id: 1, Enable: true, Protocol: model.Qwdtt, Settings: `{"listenAddr":"0.0.0.0:56000","wgPort":1,"password":"p","dns":"8.8.8.8"}`}
	b := &model.Inbound{Id: 9, Enable: true, Protocol: model.Qwdtt, Settings: `{"listenAddr":"0.0.0.0:56000","wgPort":1,"password":"p","dns":"8.8.8.8"}`}
	ia, _ := QwdttInstanceFromInbound(a)
	ib, _ := QwdttInstanceFromInbound(b)
	if ia.ManageKey() != ib.ManageKey() {
		t.Fatal("qWDTT must use a single manager key")
	}
}
