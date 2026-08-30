package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The budget is worst-case on both sides, but the text named awgo-1 — the
// outbound baseline — for an inbound too, sending operators after no interface.
func TestIFieldBudgetErrorNamesTheInterfaceItIsAbout(t *testing.T) {
	settings := `{"i1":"` + strings.Repeat("x", oversizeIFieldChars) + `"}`
	tests := []struct {
		side   string
		err    error
		want   string
		unwant string
	}{
		{"inbound", validateAwgSettingsJSON(settings), "awgN", "awgo-"},
		{"outbound", checkOutboundIFields(&model.AwgOutbound{Id: 7, Settings: settings}), "awgo-7", "awgN"},
		{"new outbound", checkOutboundIFields(&model.AwgOutbound{Settings: settings}), "awgo-N", "awgo-0"},
	}
	for _, tc := range tests {
		t.Run(tc.side, func(t *testing.T) {
			if !errors.Is(tc.err, awg.ErrIFieldsTooLarge) {
				t.Fatalf("err = %v, want awg.ErrIFieldsTooLarge", tc.err)
			}
			got := tc.err.Error()
			if !strings.Contains(got, tc.want) || strings.Contains(got, tc.unwant) {
				t.Fatalf("message = %q, want it to name %q and never %q", got, tc.want, tc.unwant)
			}
		})
	}
}
