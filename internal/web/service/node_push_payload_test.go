package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func payloadClientEmails(t *testing.T, settings string) []string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("settings not valid json: %v", err)
	}
	raw, _ := parsed["clients"].([]any)
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if email, _ := cm["email"].(string); email != "" {
			out = append(out, email)
		}
	}
	return out
}

// Stripping disabled clients from a node push made the node delete the row and
// the master mirror that back: every depleted client destroyed, not disabled.
func TestNodePushKeepsDisabledClientsLocalPushStripsThem(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "alive@x", ID: "11111111-1111-1111-1111-111111111111", Enable: true},
		{Email: "quota@x", ID: "22222222-2222-2222-2222-222222222222", Enable: true},
	}
	ib := mkInbound(t, 30301, model.VLESS, clientsSettings(t, clients))

	for _, c := range clients {
		row := &xray.ClientTraffic{InboundId: ib.Id, Email: c.Email, Enable: true}
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed traffic %s: %v", c.Email, err)
		}
	}
	if err := db.Model(xray.ClientTraffic{}).
		Where("email = ?", "quota@x").
		Update("enable", false).Error; err != nil {
		t.Fatalf("deplete quota@x: %v", err)
	}

	t.Run("node payload keeps the depletion-disabled client", func(t *testing.T) {
		built, err := svc.buildInboundForNodePush(db, ib)
		if err != nil {
			t.Fatalf("buildInboundForNodePush: %v", err)
		}
		got := payloadClientEmails(t, built.Settings)
		if len(got) != 2 {
			t.Fatalf("node push dropped a client: got %v, want both alive@x and quota@x", got)
		}
	})

	t.Run("local xray payload still strips it", func(t *testing.T) {
		built, err := svc.buildInboundForLocalRuntime(db, ib)
		if err != nil {
			t.Fatalf("buildInboundForLocalRuntime: %v", err)
		}
		got := payloadClientEmails(t, built.Settings)
		if len(got) != 1 || got[0] != "alive@x" {
			t.Fatalf("local runtime payload = %v, want only alive@x", got)
		}
	})
}

// The push path must carry settings VERBATIM: per-user credentials are
// derived by each panel from its own secret, and the node's users file can
// only contain credentials minted by the node. Minting a seed or rewriting a
// password here would diverge the master's subscription from what the node
// actually serves (369 report: master handed out per-user passwords derived
// from master-side HMAC inputs the seedless node inbound never had). The same
// minting was once silently re-added by the v3.6.0 merge (298899e6 → lost in
// ddd92d06) — this test is the merge tripwire.
func TestNodePushCarriesSettingsVerbatim(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	settings := `{"password":"keep-shared-pass","clients":[{"email":"n@x","enable":true}]}`
	ib := mkInbound(t, 30302, model.Anytls, settings)

	built, err := svc.buildInboundForNodePush(db, ib)
	if err != nil {
		t.Fatalf("buildInboundForNodePush: %v", err)
	}
	if built.Settings != settings {
		t.Fatalf("push rewrote settings:\ngot:  %s\nwant: %s", built.Settings, settings)
	}
	if built.Settings == "" {
		t.Fatal("push produced empty settings")
	}
}
