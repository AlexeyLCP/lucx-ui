package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// 44 base64 chars = 32 bytes, the only shape the AWG3 cipher accepts.
const awgWarningTestKey = "MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18="

// The threshold drops from 3492 to 3456 once a header-protection key is on the
// device, so the same set has to start warning the moment the key is added.
func TestAwgIFieldBudgetWarning_HeaderProtectionKeyLowersTheThreshold(t *testing.T) {
	// 3480 chars = 3488 IBytes: inside 3492, outside 3456.
	between := strings.Repeat("x", 3480)
	if warn := AwgIFieldBudgetWarning(`{"i1":"` + between + `"}`); warn != "" {
		t.Fatalf("warning without a header-protection key = %q, want none", warn)
	}
	warn := AwgIFieldBudgetWarning(`{"i1":"` + between + `","headerProtectionKey":"` + awgWarningTestKey + `"}`)
	if !strings.Contains(warn, "3488 > 3456") {
		t.Fatalf("warning with a header-protection key = %q, want it to measure against 3456", warn)
	}
}

// The set belongs to a server already running it, so the advice tail points at
// a knob the operator may not own; the measured sizes are what they can act on.
func TestAwgIFieldBudgetWarning_KeepsTheSizesAndDropsTheAdvice(t *testing.T) {
	warn := AwgIFieldBudgetWarning(`{"i1":"` + strings.Repeat("x", oversizeIFieldChars) + `"}`)
	if !strings.Contains(warn, awg.ErrIFieldsTooLarge.Error()) {
		t.Fatalf("warning = %q, want it to name %v", warn, awg.ErrIFieldsTooLarge)
	}
	if !strings.Contains(warn, "3604 > 3492 worst-case bytes for awgN") {
		t.Fatalf("warning = %q, want the measured sizes and the interface shape", warn)
	}
	if strings.Contains(warn, "shorten or drop") {
		t.Fatalf("warning = %q, still carries the advice tail", warn)
	}
}

// A save only ever reaches the warning when the budget is the sole complaint,
// so anything else the validator finds must leave the warning silent.
func TestAwgIFieldBudgetWarning_SilentUnlessTheBudgetIsTheOnlyProblem(t *testing.T) {
	oversize := strings.Repeat("x", oversizeIFieldChars)
	for _, tc := range []struct{ name, settings string }{
		{"within budget", `{"i1":"` + strings.Repeat("x", normalIFieldChars) + `"}`},
		{"no i-fields at all", `{"dns":"1.1.1.1"}`},
		{"malformed json", `{`},
		{"control char in an oversize field", `{"i1":"` + oversize + `\nEndpoint = attacker.example"}`},
		{"bad header protection key on an oversize set", `{"i1":"` + oversize + `","headerProtectionKey":"nope"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if warn := AwgIFieldBudgetWarning(tc.settings); warn != "" {
				t.Fatalf("AwgIFieldBudgetWarning(%s) = %q, want none", tc.name, warn)
			}
		})
	}
}

// countIFieldBudgetLogs counts budget warnings in the panel's log ring buffer
// (10240 entries), which is process-wide — callers compare a delta, not a total.
func countIFieldBudgetLogs(t *testing.T) int {
	t.Helper()
	n := 0
	for _, line := range logger.GetLogs(10240, "warning") {
		if strings.Contains(line, awg.ErrIFieldsTooLarge.Error()) {
			n++
		}
	}
	return n
}

// The refusal logged two Warning lines every 5 seconds forever, because the
// node stayed dirty. A save logs the same fact once and the retries stop.
func TestAwgOversizeIFieldSetLogsOncePerSave(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)

	before := countIFieldBudgetLogs(t)
	created, _, err := (&InboundService{}).AddInbound(&model.Inbound{
		Tag:            "in-51833-udp",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           51833,
		Protocol:       model.AWG,
		StreamSettings: `{"network":"udp"}`,
		Settings:       awgIFieldSettings(t, oversize, "1.1.1.1"),
	})
	if err != nil {
		t.Fatalf("AddInbound(oversize I-set) = %v, want nil", err)
	}
	if got := countIFieldBudgetLogs(t) - before; got != 1 {
		t.Fatalf("AddInbound logged %d budget lines, want exactly 1", got)
	}

	before = countIFieldBudgetLogs(t)
	update := *created
	update.Settings = awgIFieldSettings(t, oversize, "9.9.9.9")
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound(oversize I-set) = %v, want nil", err)
	}
	if got := countIFieldBudgetLogs(t) - before; got != 1 {
		t.Fatalf("UpdateInbound logged %d budget lines, want exactly 1", got)
	}
}

// The note is what tells an operator their server is fine and their clients are
// not — the one signal that this class of defect ever produces.
func TestAwgIFieldExportNote(t *testing.T) {
	for _, tc := range []struct{ name, settings, want string }{
		{"all portable", `{"i1":"<b 0x01>","i2":"<t>"}`, ""},
		{"kernel-only tag", `{"i1":"<b 0x01>","i3":"<c>"}`, "I3"},
		{"two lost, in order", `{"i2":"<d>","i1":"<c>","i5":"<t>"}`, "I1, I2"},
		{"blank is not a loss", `{"i1":"   ","i2":""}`, ""},
		{"padding is not a loss", `{"i1":" <b 0x01> "}`, ""},
		{"unparsable settings", `{nope`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AwgIFieldExportNote(tc.settings); got != tc.want {
				t.Errorf("AwgIFieldExportNote(%s) = %q, want %q", tc.settings, got, tc.want)
			}
		})
	}
}
