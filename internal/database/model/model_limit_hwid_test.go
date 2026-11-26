// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// B4: the HWID limit rides the settings wire — a limit set on the master must
// reach the nodes through the clients[] JSON (and the per-client push
// payload); ToRecord/ToClient copies are the two legs of that trip. If either
// copy breaks, the node enforces unlimited again while showing 0 in its card.
func TestClientLimitHwidRoundTrip(t *testing.T) {
	c := Client{Email: "a@x", LimitHwid: 2}
	r := c.ToRecord()
	if r.LimitHwid != 2 {
		t.Fatalf("ToRecord lost LimitHwid: %+v", r)
	}
	c2 := r.ToClient()
	if c2.LimitHwid != 2 {
		t.Fatalf("ToClient lost LimitHwid: %+v", c2)
	}
	// Wire key: node-side panels unmarshal clients[] with this exact name.
	raw := string(mustMarshal(t, c))
	if !strings.Contains(raw, `"limitHwid":2`) {
		t.Fatalf("wire json lacks limitHwid: %s", raw)
	}
	// A limit-less client stays byte-compatible: omitempty omits the key.
	plain := string(mustMarshal(t, Client{Email: "b@x"}))
	if strings.Contains(plain, "limitHwid") {
		t.Fatalf("limitHwid leaked into a limit-less client: %s", plain)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
