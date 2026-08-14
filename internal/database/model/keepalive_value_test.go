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

func TestKeepAliveValue_Scan(t *testing.T) {
	cases := []struct {
		name string
		src  any
		want KeepAliveValue
	}{
		{"legacy int64 column", int64(25), "25"},
		{"legacy int64 zero", int64(0), "0"},
		{"text column", "15-25", "15-25"},
		{"text column padded", "  25  ", "25"},
		{"bytes column", []byte("15-25"), "15-25"},
		{"numeric column", float64(25), "25"},
		{"null column", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var k KeepAliveValue
			if err := k.Scan(tc.src); err != nil {
				t.Fatalf("Scan(%#v): %v", tc.src, err)
			}
			if k != tc.want {
				t.Errorf("Scan(%#v) = %q, want %q", tc.src, k, tc.want)
			}
		})
	}
}

func TestKeepAliveValue_ScanUnsupported(t *testing.T) {
	var k KeepAliveValue
	err := k.Scan(struct{}{})
	if err == nil {
		t.Fatal("expected an error for an unsupported source type")
	}
	if err.Error() != "model: cannot scan struct {} into KeepAliveValue" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKeepAliveValue_Value(t *testing.T) {
	for _, tc := range []struct {
		in   KeepAliveValue
		want string
	}{
		{"25", "25"},
		{"15-25", "15-25"},
		{"  25 ", "25"},
		{"", ""},
	} {
		got, err := tc.in.Value()
		if err != nil {
			t.Fatalf("Value(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("Value(%q) = %#v, want %q", tc.in, got, tc.want)
		}
	}
}

func TestKeepAliveValue_UnmarshalVanillaClientJSON(t *testing.T) {
	raw := `{"email":"a@test","keepAlive":25,"enable":true,"allowedIPs":["10.0.0.2/32"]}`
	var c Client
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("vanilla client json: %v", err)
	}
	if c.KeepAlive != "25" {
		t.Fatalf("KeepAlive = %q, want \"25\"", c.KeepAlive)
	}
	if c.KeepAlive.Int() != 25 {
		t.Fatalf("Int() = %d, want 25", c.KeepAlive.Int())
	}

	var many []Client
	if err := json.Unmarshal([]byte(`[
		{"email":"one","keepAlive":25},
		{"email":"two","keepAlive":0},
		{"email":"three"}
	]`), &many); err != nil {
		t.Fatalf("vanilla clients array: %v", err)
	}
	if many[0].KeepAlive != "25" || many[1].KeepAlive != "0" || many[2].KeepAlive != "" {
		t.Fatalf("slice keepalive = %q/%q/%q", many[0].KeepAlive, many[1].KeepAlive, many[2].KeepAlive)
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
