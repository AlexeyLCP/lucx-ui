package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

const tunnelCredsSettings = `{"clients":[{"email":"user@lucx.local","enable":true},{"email":"off@lucx.local","enable":false}]}`

func seedTunnelCredsInbound(t *testing.T, protocol model.Protocol) *model.Inbound {
	t.Helper()
	seedInboundConflict(t, "creds-"+string(protocol), "0.0.0.0", 443, protocol, "", tunnelCredsSettings)
	var ib model.Inbound
	if err := database.GetDB().Where("tag = ?", "creds-"+string(protocol)).First(&ib).Error; err != nil {
		t.Fatalf("read back seeded inbound: %v", err)
	}
	return &ib
}

// The pair the card shows must be byte-identical to the one the sidecar writes
// into its credentials file — otherwise the operator types a login that the
// server rejects.
func TestTunnelClientCredentials_MatchesSidecarDerivation(t *testing.T) {
	setupConflictDB(t)
	svc := &ClientService{}
	secret, err := (&SettingService{}).GetSecret()
	if err != nil || len(secret) == 0 {
		t.Fatalf("GetSecret: %v (len=%d)", err, len(secret))
	}

	tests := []struct {
		protocol model.Protocol
		want     func(ib *model.Inbound) tunnel.AuthPair
	}{
		{model.TrustTunnel, func(ib *model.Inbound) tunnel.AuthPair {
			return tunnel.TrustTunnelClientAuth(secret, ib.Id, "user@lucx.local")
		}},
		{model.Mieru, func(ib *model.Inbound) tunnel.AuthPair {
			return tunnel.MieruClientAuth(secret, ib.Id, "user@lucx.local")
		}},
		{model.Naive, func(ib *model.Inbound) tunnel.AuthPair {
			return tunnel.ClientAuthForInbound(secret, ib.Id, "user@lucx.local")
		}},
	}

	for _, tc := range tests {
		t.Run(string(tc.protocol), func(t *testing.T) {
			ib := seedTunnelCredsInbound(t, tc.protocol)
			got, err := svc.TunnelClientCredentials(ib.Id, "user@lucx.local")
			if err != nil {
				t.Fatalf("TunnelClientCredentials: %v", err)
			}
			want := tc.want(ib)
			if got.Username != want.User || got.Password != want.Pass {
				t.Fatalf("pair = %s/%s, want %s/%s", got.Username, got.Password, want.User, want.Pass)
			}
			if got.Protocol != string(tc.protocol) {
				t.Fatalf("protocol = %q, want %q", got.Protocol, tc.protocol)
			}
		})
	}
}

func TestTunnelClientCredentials_Rejections(t *testing.T) {
	setupConflictDB(t)
	svc := &ClientService{}
	tt := seedTunnelCredsInbound(t, model.TrustTunnel)
	vless := seedTunnelCredsInbound(t, model.VLESS)

	tests := []struct {
		name      string
		inboundId int
		email     string
	}{
		{"protocol without derived credentials", vless.Id, "user@lucx.local"},
		{"client not attached", tt.Id, "stranger@lucx.local"},
		{"client disabled on the inbound", tt.Id, "off@lucx.local"},
		{"unknown inbound", tt.Id + 1000, "user@lucx.local"},
		{"empty email", tt.Id, "  "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creds, err := svc.TunnelClientCredentials(tc.inboundId, tc.email)
			if err == nil {
				t.Fatalf("expected an error, got creds %+v", creds)
			}
			if creds != nil {
				t.Fatalf("credentials must be nil on error, got %+v", creds)
			}
		})
	}
}
