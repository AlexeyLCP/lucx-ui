// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import "testing"

func TestAwgInboundWanted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, filter int
		want       bool
	}{
		{4, 0, true},
		{4, -1, true},
		{4, 4, true},
		{4, 7, false},
		{0, 7, false},
	}
	for _, tc := range cases {
		if got := awgInboundWanted(tc.id, tc.filter); got != tc.want {
			t.Errorf("awgInboundWanted(%d, %d) = %v, want %v", tc.id, tc.filter, got, tc.want)
		}
	}
}
