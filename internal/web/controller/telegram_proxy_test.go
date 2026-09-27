package controller

import (
	"encoding/json"
	"testing"
)

func TestTelegramProxyFormDistinguishesOmittedAndCleared(t *testing.T) {
	for _, tc := range []struct {
		body    string
		present bool
		value   string
	}{
		{`{}`, false, ""},
		{`{"tgBotProxy":""}`, true, ""},
		{`{"tgBotProxy":"socks5://127.0.0.1:10881"}`, true, "socks5://127.0.0.1:10881"},
	} {
		var form updateSettingForm
		if err := json.Unmarshal([]byte(tc.body), &form); err != nil {
			t.Fatal(err)
		}
		if (form.TgBotProxy != nil) != tc.present {
			t.Fatalf("presence mismatch: %s", tc.body)
		}
		if tc.present && *form.TgBotProxy != tc.value {
			t.Fatalf("value mismatch: %s", tc.body)
		}
	}
}
