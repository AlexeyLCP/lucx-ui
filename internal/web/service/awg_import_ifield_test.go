package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func importedAwgInbound(t *testing.T, port int, i1 string) *model.Inbound {
	t.Helper()
	return &model.Inbound{
		Tag:            fmt.Sprintf("in-%d-udp", port),
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.AWG,
		StreamSettings: `{"network":"udp"}`,
		Settings:       awgIFieldSettings(t, i1, "1.1.1.1"),
	}
}

// A foreign server is already running: our budget bounds what OUR kernel reads
// back, not what its kernel accepted, and ~21% of Amnezia Pro sets are over it.
func TestAwgImport_OversizeIFieldSetSavesWithWarning(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)

	created, warn, err := (&AwgImportService{}).addImportedInbound(importedAwgInbound(t, 51822, oversize))
	if err != nil {
		t.Fatalf("addImportedInbound(oversize I-set) = %v, want nil", err)
	}
	if !strings.Contains(warn, awg.ErrIFieldsTooLarge.Error()) {
		t.Fatalf("warning = %q, want it to carry %q", warn, awg.ErrIFieldsTooLarge.Error())
	}
	if i1, _ := reloadAwgIFieldSettings(t, created.Id); i1 != oversize {
		t.Fatalf("stored i1 = %d chars, want the imported %d", len(i1), len(oversize))
	}
}

// The warning names a real problem with this one set, so an ordinary import
// must stay silent — otherwise operators learn to ignore it.
func TestAwgImport_WithinBudgetIFieldSetSavesWithoutWarning(t *testing.T) {
	setupConflictDB(t)
	normal := strings.Repeat("x", normalIFieldChars)

	created, warn, err := (&AwgImportService{}).addImportedInbound(importedAwgInbound(t, 51823, normal))
	if err != nil {
		t.Fatalf("addImportedInbound(within-budget I-set) = %v, want nil", err)
	}
	if warn != "" {
		t.Fatalf("warning = %q, want none", warn)
	}
	if i1, _ := reloadAwgIFieldSettings(t, created.Id); i1 != normal {
		t.Fatalf("stored i1 = %d chars, want the imported %d", len(i1), len(normal))
	}
}

// The waiver belongs to the import path and to one import at a time. An
// operator typing the same set in by hand still has to be told.
func TestAwgImport_LeavesManualCreateRejected(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)
	if _, _, err := (&AwgImportService{}).addImportedInbound(importedAwgInbound(t, 51824, oversize)); err != nil {
		t.Fatalf("addImportedInbound(oversize I-set) = %v, want nil", err)
	}

	manual := importedAwgInbound(t, 51825, oversize)
	if _, _, err := (&InboundService{}).AddInbound(manual); !errors.Is(err, awg.ErrIFieldsTooLarge) {
		t.Fatalf("AddInbound(oversize I-set) after an import = %v, want awg.ErrIFieldsTooLarge", err)
	}
}

// Why the warning is enough: the renderers drop an over-budget set whole, so an
// imported one is stored for the record and never reaches a kernel.
func TestAwgImport_RendererKeepsOnlyTheWithinBudgetIFieldSet(t *testing.T) {
	cases := []struct {
		name         string
		i1           string
		wantRendered bool
	}{
		{"oversize", strings.Repeat("x", oversizeIFieldChars), false},
		{"within budget", strings.Repeat("x", normalIFieldChars), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			created, _, err := (&AwgImportService{}).addImportedInbound(importedAwgInbound(t, 51826, tc.i1))
			if err != nil {
				t.Fatalf("addImportedInbound = %v, want nil", err)
			}
			_, obfuscation, _ := inboundAwgHints(reloadAwgSettingsRaw(t, created.Id), true)
			if got := strings.Contains(obfuscation, "I1 = "+tc.i1+"\n"); got != tc.wantRendered {
				t.Fatalf("rendered I1 = %v, want %v; block:\n%s", got, tc.wantRendered, obfuscation)
			}
		})
	}
}
