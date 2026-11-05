// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestValidateSubJsonClientWarpSetting(t *testing.T) {
	const valid = `{"domains":["geosite:youtube"],"endpoints":["162.159.192.1:4500"]}`
	tests := []struct {
		name      string
		value     string
		want      string
		wantError string
	}{
		{name: "blank is trimmed", value: "  ", want: ""},
		{name: "valid passes through trimmed", value: " " + valid + " ", want: valid},
		{name: "malformed JSON is rejected", value: `{"domains":`, wantError: "client WARP is invalid"},
		{name: "missing endpoints is rejected", value: `{"domains":["geosite:youtube"]}`, wantError: "endpoints must list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &entity.AllSetting{SubJsonClientWarp: tt.value}
			err := validateSubJsonClientWarpSetting(settings)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("err=%v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || settings.SubJsonClientWarp != tt.want {
				t.Fatalf("value=%q err=%v", settings.SubJsonClientWarp, err)
			}
		})
	}
}

func TestSubJsonClientWarpSettingDefaultsOffAndPersists(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubJsonClientWarp != "" {
		t.Fatalf("feature must default to off, got %q", settings.SubJsonClientWarp)
	}

	const value = `{"domains":["geosite:youtube"],"endpoints":["8.39.214.9:4177"]}`
	settings.SubJsonClientWarp = value
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSubJsonClientWarp()
	if err != nil || got != value {
		t.Fatalf("stored value = %q, err %v", got, err)
	}
}
