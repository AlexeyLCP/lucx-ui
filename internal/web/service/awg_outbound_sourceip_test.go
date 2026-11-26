// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestCheckOutboundSourceIP(t *testing.T) {
	ok := func(settings string) {
		t.Helper()
		if err := checkOutboundSourceIP(&model.AwgOutbound{Settings: settings}); err != nil {
			t.Fatalf("must accept %s: %v", settings, err)
		}
	}
	fail := func(settings, want string) {
		t.Helper()
		err := checkOutboundSourceIP(&model.AwgOutbound{Settings: settings})
		if err == nil {
			t.Fatalf("must reject %s", settings)
		}
		if !strings.Contains(fmt.Sprintf("%v", err), want) {
			t.Fatalf("error for %s = %q, want substring %q", settings, err, want)
		}
	}

	ok(`{"endpoint":"203.0.113.7:51820"}`)
	ok(`{"endpoint":"203.0.113.7:51820","sourceIP":""}`)
	ok(`{"endpoint":"203.0.113.7:51820","sourceIP":"198.51.100.9"}`)
	ok(`{"endpoint":"[2606:4700::1111]:51820","sourceIP":"2606:4700::2222"}`)
	fail(`{"endpoint":"203.0.113.7:51820","sourceIP":"x; wget http://evil"}`, "invalid IP")
	fail(`{"endpoint":"203.0.113.7:51820","sourceIP":"2606:4700::2222"}`, "family")
	fail(`{"endpoint":"awg.example.com:51820","sourceIP":"198.51.100.9"}`, "literal IP endpoint")
}
