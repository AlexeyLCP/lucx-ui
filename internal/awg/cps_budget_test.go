// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"errors"
	"strings"
	"testing"
)

// The budget is the number the whole fix hangs on: 3500 for a one-peer client
// interface named awgo-1 with no header-protection key. The largest readable
// set measured on that shape was 3628 bytes, so 3500 keeps the 128-byte margin.
func TestIBytesBudget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ifname string
		hpk    bool
		want   int
	}{
		{"baseline awgo-1", "awgo-1", false, 3500},
		{"same align class awgo-99", "awgo-99", false, 3500},
		{"longer ifname costs one align step", "awgo-100", false, 3496},
		{"shorter ifname gains one align step", "wg0", false, 3504},
		{"header protection key costs 36", "awgo-1", true, 3464},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IBytesBudget(tc.ifname, tc.hpk); got != tc.want {
				t.Fatalf("IBytesBudget(%q, %v) = %d, want %d", tc.ifname, tc.hpk, got, tc.want)
			}
		})
	}
}

func TestIBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields [5]string
		want   int
	}{
		{"empty set", [5]string{}, 0},
		{"one byte pays a full aligned slot", [5]string{"a"}, 8},
		{"len 4..7 share one slot", [5]string{"aaaaaaa"}, 12},
		{"len 8 crosses into the next slot", [5]string{"aaaaaaaa"}, 16},
		{"blank fields are not charged", [5]string{"aaaa", "", "  ", "", ""}, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.fields
			if got := IBytes(f[0], f[1], f[2], f[3], f[4]); got != tc.want {
				t.Fatalf("IBytes(%q) = %d, want %d", f, got, tc.want)
			}
		})
	}
}

// NLA_ALIGN quantises per field, so the character sum is not monotonic in
// IBytes — measured on a real device, 3594 and 3596 chars fail while 3598
// succeeds. Any guard written against len(I1)+…+len(I5) is therefore wrong.
func TestIBytes_NotMonotonicInCharacterSum(t *testing.T) {
	whole := strings.Repeat("a", 10)
	half := strings.Repeat("a", 5)
	if len(whole) != len(half)*2 {
		t.Fatalf("test setup: character sums must match")
	}
	one := IBytes(whole, "", "", "", "")
	split := IBytes(half, half, "", "", "")
	if one != 16 || split != 24 {
		t.Fatalf("one field = %d (want 16), split = %d (want 24) — same 10 characters", one, split)
	}
}

func TestValidateIFields(t *testing.T) {
	underBudget := strings.Repeat("x", 3495) // IBytes 3500, exactly the budget
	overBudget := strings.Repeat("x", 3496)  // IBytes 3504, one align step over

	if err := ValidateIFields("awgo-1", "", underBudget, "", "", "", ""); err != nil {
		t.Fatalf("set at exactly the budget must pass, got %v", err)
	}
	err := ValidateIFields("awgo-1", "", overBudget, "", "", "", "")
	if !errors.Is(err, ErrIFieldsTooLarge) {
		t.Fatalf("want ErrIFieldsTooLarge, got %v", err)
	}
	// The same set becomes illegal once a header-protection key claims its
	// netlink attribute — the guard must read the instance shape, not a constant.
	fits := strings.Repeat("x", 3463)
	if err := ValidateIFields("awgo-1", "", fits, "", "", "", ""); err != nil {
		t.Fatalf("3463 chars must fit without an HPK, got %v", err)
	}
	if err := ValidateIFields("awgo-1", "key=", fits, "", "", "", ""); !errors.Is(err, ErrIFieldsTooLarge) {
		t.Fatalf("3463 chars must not fit with an HPK, got %v", err)
	}
}
