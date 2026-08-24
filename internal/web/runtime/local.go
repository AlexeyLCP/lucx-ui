package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/awg" // LUCX-HOOK: AWG sidecar
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel" // LUCX-HOOK: Naive inbound sidecar
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type LocalDeps struct {
	APIPort        func() int
	SetNeedRestart func()
}

type Local struct {
	deps LocalDeps
	mu   sync.Mutex
}

func NewLocal(deps LocalDeps) *Local {
	return &Local{deps: deps}
}

func (l *Local) Name() string { return "local" }

func (l *Local) withAPI(fn func(api *xray.XrayAPI) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	port := l.deps.APIPort()
	if port <= 0 {
		return errors.New("local xray is not running")
	}
	var api xray.XrayAPI
	if err := api.Init(port); err != nil {
		return err
	}
	defer api.Close()
	return fn(&api)
}

func (l *Local) AddInbound(_ context.Context, ib *model.Inbound) error {
	if ib.Protocol == model.MTProto {
		inst, ok := mtproto.InstanceFromInbound(ib)
		if !ok {
			return nil
		}
		return mtproto.GetManager().Ensure(inst)
	}
	// LUCX-HOOK: AWG — delegate to the kernel-interface sidecar manager.
	if ib.Protocol == model.AWG {
		inst, ok := awg.InstanceFromInbound(ib)
		if !ok {
			return nil
		}
		return awg.GetManager().Ensure(inst)
	}
	// LUCX-HOOK: tunnel sidecars as inbounds (not Xray protocols).
	if ib.Protocol == model.Naive {
		return l.ensureNaiveInbound(ib)
	}
	if ib.Protocol == model.Olcrtc {
		return l.ensureOlcrtcInbound(ib)
	}
	if ib.Protocol == model.Qwdtt {
		return l.ensureQwdttInbound(ib)
	}
	if ib.Protocol == model.Mieru {
		return l.ensureMieruInbound(ib)
	}
	if ib.Protocol == model.TrustTunnel {
		return l.ensureTrustTunnelInbound(ib)
	}
	// END LUCX-HOOK
	if ib.Protocol == model.AmneziaWG {
		inst, ok := amneziawg.InstanceFromInbound(ib)
		if !ok {
			return nil
		}
		err := amneziawgnet.GetManager().Ensure(amneziawgnet.Desired{
			Instance: inst,
			Options: amneziawgnet.DeviceOptions{
				HeaderProtectionKey:    inst.Obfuscation.HeaderProtectionKey,
				ContentPaddingAddition: inst.Obfuscation.ContentPaddingAddition,
				RekeyAfterTime:         inst.Obfuscation.RekeyAfterTime,
				RekeyTimeout:           inst.Obfuscation.RekeyTimeout,
				RejectAfterTime:        inst.Obfuscation.RejectAfterTime,
				KeepaliveTimeout:       inst.Obfuscation.KeepaliveTimeout,
				MaxHandshakeAttempts:   inst.Obfuscation.MaxHandshakeAttempts,
				RandomTrailers:         inst.Obfuscation.RandomTrailers,
				DisableCookies:         inst.Obfuscation.DisableCookies,
			},
		})
		// A brand new inbound can be the first one to qualify for
		// injectAmneziawgnetSocks's Xray-side relay inbound (e.g. its first
		// valid peer). Ensure only updates the embedded Device -- flag Xray
		// for a resync so the relay actually gets created within the next
		// ApplyPendingRestart tick instead of only at the next full restart.
		if l.deps.SetNeedRestart != nil {
			l.deps.SetNeedRestart()
		}
		return err
	}
	body, err := json.MarshalIndent(ib.GenXrayInboundConfig(), "", "  ")
	if err != nil {
		return err
	}
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.AddInbound(body)
	})
}

func (l *Local) DelInbound(_ context.Context, ib *model.Inbound) error {
	if ib.Protocol == model.MTProto {
		mtproto.GetManager().Remove(ib.Id)
		return nil
	}
	// LUCX-HOOK: AWG — delegate removal to the sidecar manager.
	if ib.Protocol == model.AWG {
		awg.GetManager().Remove(ib.Id)
		return nil
	}
	// LUCX-HOOK: tunnel inbound teardown.
	if ib.Protocol == model.Naive {
		tunnel.GetManager().Remove(tunnel.NaiveKey(ib.Id))
		return nil
	}
	if ib.Protocol == model.Olcrtc {
		tunnel.GetManager().Remove(tunnel.OlcrtcKey(ib.Id))
		return nil
	}
	if ib.Protocol == model.Qwdtt {
		tunnel.GetManager().Remove(tunnel.QwdttKey)
		return nil
	}
	if ib.Protocol == model.Mieru {
		tunnel.GetManager().Remove(tunnel.MieruKey(ib.Id))
		return nil
	}
	if ib.Protocol == model.TrustTunnel {
		tunnel.GetManager().Remove(tunnel.TrustTunnelKey(ib.Id))
		return nil
	}
	// END LUCX-HOOK
	if ib.Protocol == model.AmneziaWG {
		amneziawgnet.GetManager().Remove(ib.Id)
		// The removed inbound may have been the only one backing Xray's
		// injectAmneziawgnetSocks relay inbound for this tag -- flag a
		// resync so the now-stale relay gets torn down promptly.
		if l.deps.SetNeedRestart != nil {
			l.deps.SetNeedRestart()
		}
		return nil
	}
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.DelInbound(ib.Tag)
	})
}

func (l *Local) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	if oldIb.Protocol == model.MTProto || newIb.Protocol == model.MTProto {
		return l.updateMtprotoInbound(ctx, oldIb, newIb)
	}
	// LUCX-HOOK: tunnel inbound update (Del+Add / Ensure restart).
	if isTunnelInboundProto(oldIb.Protocol) || isTunnelInboundProto(newIb.Protocol) {
		_ = l.DelInbound(ctx, oldIb)
		if !newIb.Enable || !isTunnelInboundProto(newIb.Protocol) {
			if !isTunnelInboundProto(newIb.Protocol) && newIb.Enable {
				return l.AddInbound(ctx, newIb)
			}
			return nil
		}
		return l.AddInbound(ctx, newIb)
	}
	// END LUCX-HOOK
	if oldIb.Protocol == model.AmneziaWG || newIb.Protocol == model.AmneziaWG {
		return l.updateAmneziaWGInbound(ctx, oldIb, newIb)
	}
	_ = l.DelInbound(ctx, oldIb)
	if !newIb.Enable {
		return nil
	}
	return l.AddInbound(ctx, newIb)
}

func isTunnelInboundProto(p model.Protocol) bool {
	return p == model.Naive || p == model.Olcrtc || p == model.Qwdtt || p == model.Mieru || p == model.TrustTunnel
}

// ensureNaiveInbound builds and Ensures a Naive sidecar instance. Panel secret
// is required for per-client basic_auth derivation (read from settings table
// without importing service — avoids an import cycle with runtime).
func (l *Local) ensureNaiveInbound(ib *model.Inbound) error {
	secret := panelSecretBytes()
	inst, ok := tunnel.InstanceFromInbound(ib, secret)
	if !ok {
		return nil
	}
	return tunnel.GetManager().Ensure(inst)
}

func (l *Local) ensureOlcrtcInbound(ib *model.Inbound) error {
	inst, ok := tunnel.OlcrtcInstanceFromInbound(ib)
	if !ok {
		return nil
	}
	return tunnel.GetManager().Ensure(inst)
}

func (l *Local) ensureQwdttInbound(ib *model.Inbound) error {
	inst, ok := tunnel.QwdttInstanceFromInbound(ib)
	if !ok {
		return nil
	}
	return tunnel.GetManager().Ensure(inst)
}

func (l *Local) ensureMieruInbound(ib *model.Inbound) error {
	inst, ok := tunnel.MieruInstanceFromInbound(ib, panelSecretBytes())
	if !ok {
		return nil
	}
	return tunnel.GetManager().Ensure(inst)
}

func (l *Local) ensureTrustTunnelInbound(ib *model.Inbound) error {
	cert, key := panelCertFilesForRuntime()
	inst, ok := tunnel.TrustTunnelInstanceFromInbound(ib, panelSecretBytes(), cert, key)
	if !ok {
		return nil
	}
	return tunnel.GetManager().Ensure(inst)
}

// panelCertFilesForRuntime reads webCertFile/webKeyFile settings (TrustTunnel
// default cert source), mirroring service.panelCertFiles without the import.
func panelCertFilesForRuntime() (cert, key string) {
	var rows []model.Setting
	if err := database.GetDB().Where("key IN ?", []string{"webCertFile", "webKeyFile"}).Find(&rows).Error; err != nil {
		return "", ""
	}
	for _, r := range rows {
		switch r.Key {
		case "webCertFile":
			cert = strings.TrimSpace(r.Value)
		case "webKeyFile":
			key = strings.TrimSpace(r.Value)
		}
	}
	return cert, key
}

func panelSecretBytes() []byte {
	var row model.Setting
	if err := database.GetDB().Where("key = ?", "secret").First(&row).Error; err != nil {
		return nil
	}
	if strings.TrimSpace(row.Value) == "" {
		return nil
	}
	return []byte(row.Value)
}

// updateMtprotoInbound applies an inbound update without the Del+Add sequence
// the xray path uses: Remove would drop the manager's fingerprint state, which
// is what lets Ensure keep the running mtg process (and its live connections)
// when nothing in the generated config changed. The sidecar is only stopped
// when the inbound is disabled, loses its last active secret, or moves to a
// different protocol.
func (l *Local) updateMtprotoInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	if oldIb.Protocol == model.MTProto && newIb.Protocol != model.MTProto {
		mtproto.GetManager().Remove(oldIb.Id)
		if !newIb.Enable {
			return nil
		}
		return l.AddInbound(ctx, newIb)
	}
	if oldIb.Protocol != model.MTProto {
		_ = l.DelInbound(ctx, oldIb)
	}
	if !newIb.Enable {
		mtproto.GetManager().Remove(newIb.Id)
		return nil
	}
	inst, ok := mtproto.InstanceFromInbound(newIb)
	if !ok {
		mtproto.GetManager().Remove(newIb.Id)
		return nil
	}
	return mtproto.GetManager().Ensure(inst)
}

// updateAmneziaWGInbound mirrors updateMtprotoInbound: it skips the
// Remove+Ensure sequence a plain Del+Add would force so that, on an
// AmneziaWG-to-AmneziaWG edit, Manager.Ensure's own fingerprint comparison
// can reconfigure the running embedded Device in place via IpcSet instead
// of always rebuilding it (see internal/amneziawgnet.Manager.ensureLocked --
// only an address/MTU change forces a rebuild there, not a peer edit).
//
// Every exit path below only touches the embedded Device via
// amneziawgnet.GetManager() -- none of it rebuilds Xray's own config, which
// is what actually creates/removes injectAmneziawgnetSocks's relay inbound.
// A peer edit that changes whether this inbound has a qualifying peer at
// all (its first peer added, or its last one removed) must still get that
// relay created or torn down, so flag Xray for a resync unconditionally
// here rather than trying to enumerate which of the branches below need it.
func (l *Local) updateAmneziaWGInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	if l.deps.SetNeedRestart != nil {
		l.deps.SetNeedRestart()
	}
	if oldIb.Protocol == model.AmneziaWG && newIb.Protocol != model.AmneziaWG {
		amneziawgnet.GetManager().Remove(oldIb.Id)
		if !newIb.Enable {
			return nil
		}
		return l.AddInbound(ctx, newIb)
	}
	if oldIb.Protocol != model.AmneziaWG {
		_ = l.DelInbound(ctx, oldIb)
	}
	if !newIb.Enable {
		amneziawgnet.GetManager().Remove(newIb.Id)
		return nil
	}
	inst, ok := amneziawg.InstanceFromInbound(newIb)
	if !ok {
		amneziawgnet.GetManager().Remove(newIb.Id)
		return nil
	}
	return amneziawgnet.GetManager().Ensure(amneziawgnet.Desired{
		Instance: inst,
		Options: amneziawgnet.DeviceOptions{
			HeaderProtectionKey:    inst.Obfuscation.HeaderProtectionKey,
			ContentPaddingAddition: inst.Obfuscation.ContentPaddingAddition,
			RekeyAfterTime:         inst.Obfuscation.RekeyAfterTime,
			RekeyTimeout:           inst.Obfuscation.RekeyTimeout,
			RejectAfterTime:        inst.Obfuscation.RejectAfterTime,
			KeepaliveTimeout:       inst.Obfuscation.KeepaliveTimeout,
			MaxHandshakeAttempts:   inst.Obfuscation.MaxHandshakeAttempts,
			RandomTrailers:         inst.Obfuscation.RandomTrailers,
			DisableCookies:         inst.Obfuscation.DisableCookies,
		},
	})
}

func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {
	if ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {
		return nil
	}
	// LUCX-HOOK: AWG — peer reconciliation is driven by the periodic awg job,
	// which reads the desired peer set from the DB. No live API call here.
	if ib.Protocol == model.AWG {
		return nil
	}
	// LUCX-HOOK: tunnel inbounds — no Xray users.
	if isTunnelInboundProto(ib.Protocol) {
		return nil
	}
	// END LUCX-HOOK
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.AddUser(string(ib.Protocol), ib.Tag, userMap)
	})
}

func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {
	if ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {
		return nil
	}
	// LUCX-HOOK: AWG — peer removal is picked up by the next Reconcile tick.
	if ib.Protocol == model.AWG {
		return nil
	}
	// LUCX-HOOK: tunnel inbounds — no Xray users.
	if isTunnelInboundProto(ib.Protocol) {
		return nil
	}
	// END LUCX-HOOK
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.RemoveUser(ib.Tag, email)
	})
}

func (l *Local) AddClient(ctx context.Context, ib *model.Inbound, client model.Client) error {
	if !client.Enable {
		return nil
	}
	user := map[string]any{
		"email":        client.Email,
		"id":           client.ID,
		"security":     client.Security,
		"flow":         client.Flow,
		"auth":         client.Auth,
		"password":     client.Password,
		"publicKey":    client.PublicKey,
		"allowedIPs":   client.AllowedIPs,
		"preSharedKey": client.PreSharedKey,
		"keepAlive":    wgKeepAlive(client.KeepAlive),
	}
	return l.AddUser(ctx, ib, user)
}

func (l *Local) DeleteUser(ctx context.Context, ib *model.Inbound, email string) error {
	if email == "" {
		return nil
	}
	if err := l.RemoveUser(ctx, ib, email); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	return nil
}

func (l *Local) DeleteClient(context.Context, string) error {
	return nil
}

func (l *Local) UpdateUser(ctx context.Context, ib *model.Inbound, oldEmail string, payload model.Client) error {
	if oldEmail != "" {
		if err := l.RemoveUser(ctx, ib, oldEmail); err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	if !payload.Enable {
		return nil
	}
	user := map[string]any{
		"email":        payload.Email,
		"id":           payload.ID,
		"security":     payload.Security,
		"flow":         payload.Flow,
		"auth":         payload.Auth,
		"password":     payload.Password,
		"publicKey":    payload.PublicKey,
		"allowedIPs":   payload.AllowedIPs,
		"preSharedKey": payload.PreSharedKey,
		"keepAlive":    wgKeepAlive(payload.KeepAlive),
	}
	return l.AddUser(ctx, ib, user)
}

func wgKeepAlive(v model.KeepAliveValue) string {
	if v.IsZero() {
		return ""
	}
	// xray wireguard peers take a plain interval; ranges use lo.
	if n := v.Int(); n > 0 {
		return strconv.Itoa(n)
	}
	return ""
}

func (l *Local) RestartXray(_ context.Context) error {
	if l.deps.SetNeedRestart != nil {
		l.deps.SetNeedRestart()
	}
	return nil
}

func (l *Local) ResetClientTraffic(_ context.Context, _ *model.Inbound, _ string) error {
	return nil
}

func (l *Local) ResetAllTraffics(_ context.Context) error {
	return nil
}

func (l *Local) ResetInboundTraffic(_ context.Context, _ *model.Inbound) error {
	return nil
}
