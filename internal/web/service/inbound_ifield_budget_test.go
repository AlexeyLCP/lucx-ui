package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// 3604 IBytes against a 3492 budget — the set from the field report, since
// nlaBytes rounds (3596+8) up to 3604. Below the budget for the control cases.
const (
	oversizeIFieldChars = 3596
	normalIFieldChars   = 100
)

func awgIFieldSettings(t *testing.T, i1, dns string) string {
	t.Helper()
	return awgIFieldSettingsKeyed(t, "i1", i1, dns)
}

func awgIFieldSettingsKeyed(t *testing.T, key, i1, dns string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"clients": []any{}, key: i1, "dns": dns})
	if err != nil {
		t.Fatalf("marshal awg settings: %v", err)
	}
	return string(raw)
}

func seedAwgIFieldInbound(t *testing.T, i1, dns string) *model.Inbound {
	t.Helper()
	return seedAwgIFieldInboundKeyed(t, "i1", i1, dns)
}

func seedAwgIFieldInboundKeyed(t *testing.T, key, i1, dns string) *model.Inbound {
	t.Helper()
	seedInboundConflict(t, "in-51820-udp", "0.0.0.0", 51820, model.AWG, `{"network":"udp"}`, awgIFieldSettingsKeyed(t, key, i1, dns))
	var in model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-51820-udp").First(&in).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}
	return &in
}

// 5 x 720 chars = 3640 IBytes, over the 3492 budget however the set is split.
func awgSpreadIFieldSettings(t *testing.T, vals [5]string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"clients": []any{}, "dns": "1.1.1.1",
		"i1": vals[0], "i2": vals[1], "i3": vals[2], "i4": vals[3], "i5": vals[4],
	})
	if err != nil {
		t.Fatalf("marshal awg settings: %v", err)
	}
	return string(raw)
}

func reloadAwgSettingsRaw(t *testing.T, id int) string {
	t.Helper()
	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, id).Error; err != nil {
		t.Fatalf("reload inbound %d: %v", id, err)
	}
	return reloaded.Settings
}

func reloadAwgSpreadIFields(t *testing.T, id int) [5]string {
	t.Helper()
	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, id).Error; err != nil {
		t.Fatalf("reload inbound %d: %v", id, err)
	}
	var s struct {
		I1 string `json:"i1"`
		I2 string `json:"i2"`
		I3 string `json:"i3"`
		I4 string `json:"i4"`
		I5 string `json:"i5"`
	}
	if err := json.Unmarshal([]byte(reloaded.Settings), &s); err != nil {
		t.Fatalf("unmarshal reloaded settings: %v", err)
	}
	return [5]string{s.I1, s.I2, s.I3, s.I4, s.I5}
}

func reloadAwgIFieldSettings(t *testing.T, id int) (i1, dns string) {
	t.Helper()
	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, id).Error; err != nil {
		t.Fatalf("reload inbound %d: %v", id, err)
	}
	var s struct {
		I1  string `json:"i1"`
		DNS string `json:"dns"`
	}
	if err := json.Unmarshal([]byte(reloaded.Settings), &s); err != nil {
		t.Fatalf("unmarshal reloaded settings: %v", err)
	}
	return s.I1, s.DNS
}

// A node's reconcile push resubmits the inbound's own stored settings every 5s,
// so budgeting an untouched I-set left the node permanently dirty.
func TestUpdateInbound_UnchangedOversizeIFieldSetStaysSavable(t *testing.T) {
	oversize := strings.Repeat("x", oversizeIFieldChars)
	// encoding/json matches tags case-insensitively, so a hand-written "I1"
	// reaches the budget check and has to reach the exemption too.
	for _, key := range []string{"i1", "I1"} {
		t.Run(key, func(t *testing.T) {
			setupConflictDB(t)
			existing := seedAwgIFieldInboundKeyed(t, key, oversize, "1.1.1.1")

			update := *existing
			if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
				t.Fatalf("UpdateInbound(unchanged oversize I-set) = %v, want nil", err)
			}
			if i1, _ := reloadAwgIFieldSettings(t, existing.Id); i1 != oversize {
				t.Fatalf("stored i1 length = %d, want the untouched %d", len(i1), len(oversize))
			}
		})
	}
}

// Refusing the save protected nothing — the renderers drop an over-budget set
// before a kernel of ours reads it — and froze node reconcile on every retry.
func TestUpdateInbound_ChangedOversizeIFieldSetSaves(t *testing.T) {
	var base [5]string
	for i := range base {
		base[i] = strings.Repeat("x", 720)
	}
	// Every field of the set counts, so moving any one of the five must save.
	for idx := range base {
		t.Run(fmt.Sprintf("i%d", idx+1), func(t *testing.T) {
			setupConflictDB(t)
			seeded := awgSpreadIFieldSettings(t, base)
			seedInboundConflict(t, "in-51820-udp", "0.0.0.0", 51820, model.AWG, `{"network":"udp"}`, seeded)
			var existing model.Inbound
			if err := database.GetDB().Where("tag = ?", "in-51820-udp").First(&existing).Error; err != nil {
				t.Fatalf("read seeded row: %v", err)
			}

			changed := base
			changed[idx] = strings.Repeat("y", 720)
			update := existing
			update.Settings = awgSpreadIFieldSettings(t, changed)
			if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
				t.Fatalf("UpdateInbound(new oversize I-set) = %v, want nil", err)
			}
			if got := reloadAwgSpreadIFields(t, existing.Id); got != changed {
				t.Fatalf("stored i%d = %d chars, want the submitted %d", idx+1, len(got[idx]), len(changed[idx]))
			}
		})
	}
}

// 4040 bytes shrunk to 3640 is still over 3492. Only this downgrade lets an
// operator get there in steps instead of one jump under the budget.
func TestUpdateInbound_SmallerButStillOversizeIFieldSetSaves(t *testing.T) {
	setupConflictDB(t)
	var base, shrunk [5]string
	for i := range base {
		base[i] = strings.Repeat("x", 800)
		shrunk[i] = strings.Repeat("x", 720)
	}
	seedInboundConflict(t, "in-51820-udp", "0.0.0.0", 51820, model.AWG, `{"network":"udp"}`, awgSpreadIFieldSettings(t, base))
	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-51820-udp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	update := existing
	update.Settings = awgSpreadIFieldSettings(t, shrunk)
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound(smaller but still oversize I-set) = %v, want nil", err)
	}
	if got := reloadAwgSpreadIFields(t, existing.Id); got != shrunk {
		t.Fatalf("stored i1 = %d chars, want the shrunk %d", len(got[0]), len(shrunk[0]))
	}
}

// IBytes trims each field before measuring, so a cosmetic space edit changes
// nothing the budget can see — the byte-for-byte comparison refused it anyway.
func TestUpdateInbound_WhitespaceOnlyIFieldEditSaves(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)
	existing := seedAwgIFieldInbound(t, oversize, "1.1.1.1")

	update := *existing
	update.Settings = awgIFieldSettings(t, " "+oversize, "1.1.1.1")
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound(whitespace-only I-field edit) = %v, want nil", err)
	}
	if i1, _ := reloadAwgIFieldSettings(t, existing.Id); i1 != " "+oversize {
		t.Fatalf("stored i1 = %d chars, want the submitted %d", len(i1), len(oversize)+1)
	}
}

// The comparison is over the I1-I5 values, not the settings blob: editing any
// other field of a grandfathered inbound must still save.
func TestUpdateInbound_OtherSettingsFieldSavesWithOversizeIFieldSet(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)
	existing := seedAwgIFieldInbound(t, oversize, "1.1.1.1")

	update := *existing
	update.Settings = awgIFieldSettings(t, oversize, "9.9.9.9")
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound(dns edit, same oversize I-set) = %v, want nil", err)
	}
	i1, dns := reloadAwgIFieldSettings(t, existing.Id)
	if dns != "9.9.9.9" || i1 != oversize {
		t.Fatalf("stored dns = %q (want 9.9.9.9), i1 length = %d (want %d)", dns, len(i1), len(oversize))
	}
}

// A within-budget set stays fully editable — the exemption must not have
// become "never check on update".
func TestUpdateInbound_ChangedWithinBudgetIFieldSetSaves(t *testing.T) {
	setupConflictDB(t)
	existing := seedAwgIFieldInbound(t, strings.Repeat("x", normalIFieldChars), "1.1.1.1")

	wanted := strings.Repeat("y", normalIFieldChars+20)
	update := *existing
	update.Settings = awgIFieldSettings(t, wanted, "1.1.1.1")
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound(new within-budget I-set) = %v, want nil", err)
	}
	if i1, _ := reloadAwgIFieldSettings(t, existing.Id); i1 != wanted {
		t.Fatalf("stored i1 = %d chars, want the new %d", len(i1), len(wanted))
	}
}

// A caller that forgives the budget must still have had every other check run,
// so the budget has to be the last verdict validateAwgSettingsJSON reaches.
func TestUpdateInbound_OversizeIFieldSetStillChecksOtherFields(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)
	existing := seedAwgIFieldInbound(t, oversize, "1.1.1.1")

	update := *existing
	update.Settings = awgIFieldSettings(t, oversize, "9.9.9.9\nEndpoint = attacker.example")
	if _, _, err := (&InboundService{}).UpdateInbound(&update); !errors.Is(err, errAwgControlChar) {
		t.Fatalf("UpdateInbound(injected dns, exempt I-set) = %v, want errAwgControlChar", err)
	}
	if _, dns := reloadAwgIFieldSettings(t, existing.Id); dns != "1.1.1.1" {
		t.Fatalf("a rejected update must not persist, stored dns = %q", dns)
	}
}

// Only the budget was ever downgraded to a warning. Everything else about an
// I-field still refuses the save, over budget or not.
func TestUpdateInbound_ControlCharInWithinBudgetIFieldRejected(t *testing.T) {
	setupConflictDB(t)
	injected := strings.Repeat("x", normalIFieldChars) + "\nEndpoint = attacker.example"
	existing := seedAwgIFieldInbound(t, injected, "1.1.1.1")

	update := *existing
	if _, _, err := (&InboundService{}).UpdateInbound(&update); !errors.Is(err, errAwgControlChar) {
		t.Fatalf("UpdateInbound(unchanged injected i1) = %v, want errAwgControlChar", err)
	}
}

// A protocol switch is still an ordinary save: the set is measured, warned
// about and stored, exactly as it would be on an inbound that was always AWG.
func TestUpdateInbound_ProtocolSwitchToAwgSaves(t *testing.T) {
	setupConflictDB(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)
	seedInboundConflict(t, "in-51820-tcp", "0.0.0.0", 51820, model.VLESS, `{"network":"tcp"}`, awgIFieldSettings(t, oversize, "1.1.1.1"))
	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-51820-tcp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	update := existing
	update.Protocol = model.AWG
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound(VLESS -> AWG, oversize I-set) = %v, want nil", err)
	}
	if i1, _ := reloadAwgIFieldSettings(t, existing.Id); i1 != oversize {
		t.Fatalf("stored i1 = %d chars, want the submitted %d", len(i1), len(oversize))
	}
}

// AddInbound is the door a node push falls through when the tag does not
// resolve, so refusing here left a fresh node dirty on every 5s retry.
func TestAddInbound_OversizeIFieldSetSaves(t *testing.T) {
	// encoding/json matches tags case-insensitively, so a hand-written "I1"
	// reaches the same single reader the budget and the warning now share.
	for _, key := range []string{"i1", "I1"} {
		t.Run(key, func(t *testing.T) {
			setupConflictDB(t)
			oversize := strings.Repeat("x", oversizeIFieldChars)
			in := &model.Inbound{
				Tag:            "in-51821-udp",
				Enable:         true,
				Listen:         "0.0.0.0",
				Port:           51821,
				Protocol:       model.AWG,
				StreamSettings: `{"network":"udp"}`,
				Settings:       awgIFieldSettingsKeyed(t, key, oversize, "1.1.1.1"),
			}
			created, _, err := (&InboundService{}).AddInbound(in)
			if err != nil {
				t.Fatalf("AddInbound(oversize I-set) = %v, want nil", err)
			}
			if i1, _ := reloadAwgIFieldSettings(t, created.Id); i1 != oversize {
				t.Fatalf("stored i1 = %d chars, want the submitted %d", len(i1), len(oversize))
			}
		})
	}
}

// The hole the import waiver opened: stripping I1-I5 to re-run the checks
// behind the budget took the control-character scan over those same fields.
func TestAddInbound_ControlCharInOversizeIFieldRejected(t *testing.T) {
	setupConflictDB(t)
	injected := strings.Repeat("x", oversizeIFieldChars) + "\nEndpoint = attacker.example"
	in := &model.Inbound{
		Tag:            "in-51821-udp",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           51821,
		Protocol:       model.AWG,
		StreamSettings: `{"network":"udp"}`,
		Settings:       awgIFieldSettings(t, injected, "1.1.1.1"),
	}
	if _, _, err := (&InboundService{}).AddInbound(in); !errors.Is(err, errAwgControlChar) {
		t.Fatalf("AddInbound(control char in an oversize i1) = %v, want errAwgControlChar", err)
	}
	var stored int64
	if err := database.GetDB().Model(&model.Inbound{}).Count(&stored).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if stored != 0 {
		t.Fatalf("a rejected create persisted %d inbound(s)", stored)
	}
}
