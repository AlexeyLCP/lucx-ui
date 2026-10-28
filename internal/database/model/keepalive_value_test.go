package model

import (
	"encoding/json"
	"testing"
)

func TestKeepAliveValue_JSONRoundTrip(t *testing.T) {
	cases := []struct {
		raw  string
		want KeepAliveValue
	}{
		{`25`, "25"},
		{`"25"`, "25"},
		{`"15-25"`, "15-25"},
		{`0`, "0"},
				{`null`, ""},
	}
	for _, tc := range cases {
		var k KeepAliveValue
		if err := json.Unmarshal([]byte(tc.raw), &k); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if k != tc.want {
			t.Errorf("unmarshal %s = %q, want %q", tc.raw, k, tc.want)
		}
	}
	// Marshaling a scalar renders bare; marshaling a range renders a string.
	b, err := json.Marshal(KeepAliveValue("25"))
	if err != nil || string(b) != "25" {
		t.Fatalf("marshal scalar = %q err %v", b, err)
	}
	b, err = json.Marshal(KeepAliveValue("15-25"))
	if err != nil || string(b) != `"15-25"` {
		t.Fatalf("marshal range = %q err %v", b, err)
	}
}

func TestKeepAliveValue_Seconds(t *testing.T) {
	if KeepAliveValue("25").Seconds() != 25 {
		t.Fatal("25")
	}
	if KeepAliveValue("15-25").Seconds() != 0 {
		t.Fatal("range must not parse as a scalar")
	}
	if KeepAliveValue("15-25").MinSeconds() != 15 {
		t.Fatal("range lo")
	}
	if KeepAliveValue("0").Seconds() != 0 {
		t.Fatal("zero")
	}
}

func TestKeepAliveValue_Validate(t *testing.T) {
	if err := KeepAliveValue("15-25").Validate(true); err != nil {
		t.Fatalf("range allowed: %v", err)
	}
	if err := KeepAliveValue("15-25").Validate(false); err == nil {
		t.Fatal("range must be refused when ranges are off")
	}
	if err := KeepAliveValue("25").Validate(false); err != nil {
		t.Fatalf("scalar: %v", err)
	}
	if err := KeepAliveValue("-1").Validate(false); err == nil {
		t.Fatal("negative keepalive must be refused")
	}
}