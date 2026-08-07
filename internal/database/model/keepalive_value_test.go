package model

import (
	"encoding/json"
	"testing"
)

func TestKeepAliveValue_JSONRoundTrip(t *testing.T) {
	cases := []struct {
		raw  string
		want KeepAliveValue
		zero bool
	}{
		{`25`, "25", false},
		{`"25"`, "25", false},
		{`"15-25"`, "15-25", false},
		{`0`, "0", true},
		{`""`, "", true},
		{`null`, "", true},
	}
	for _, tc := range cases {
		var k KeepAliveValue
		if err := json.Unmarshal([]byte(tc.raw), &k); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if k != tc.want {
			t.Errorf("unmarshal %s = %q, want %q", tc.raw, k, tc.want)
		}
		if k.IsZero() != tc.zero {
			t.Errorf("IsZero(%q) = %v, want %v", k, k.IsZero(), tc.zero)
		}
	}
}

func TestKeepAliveValue_Int(t *testing.T) {
	if KeepAliveValue("25").Int() != 25 {
		t.Fatal("25")
	}
	if KeepAliveValue("15-25").Int() != 15 {
		t.Fatal("range lo")
	}
	if KeepAliveValue("0").Int() != 0 {
		t.Fatal("zero")
	}
}

func TestKeepAliveValue_MarshalNumberWhenSingle(t *testing.T) {
	b, err := json.Marshal(KeepAliveValue("25"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "25" {
		t.Fatalf("marshal single = %s, want number 25", b)
	}
	b, err = json.Marshal(KeepAliveValue("15-25"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"15-25"` {
		t.Fatalf("marshal range = %s, want quoted string", b)
	}
}
