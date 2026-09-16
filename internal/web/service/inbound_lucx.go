package service

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/nodetype"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// inboundAwgHints returns the AWG obfuscation block as it should appear in a
// client .conf [Interface] section (Jc/Jmin/Jmax/S1-S4/H1-H4/I1-I5 lines), the
// server tunnel address, and the inbound's AWG protocol version. All three are
// read from the inbound settings so the clients-page QR/.conf path can render a
// full AmneziaWG client config and gate the per-client export-version selector.
// The obfuscation block is empty when the settings carry no obfuscation
// (lite/level-1); version defaults to "2" for pre-lucx.50 inbounds.
// localInbound must be false for a node-hosted inbound (awg.AwgVersionFieldsAllowed).
//
// LUCX-HOOK: AWG obfuscation hints for the clients-page QR/.conf path.
func inboundAwgHints(settings string, localInbound bool) (address string, obfuscation string, version string) {
	if strings.TrimSpace(settings) == "" {
		return "", "", ""
	}
	var s struct {
		Address                string       `json:"address"`
		Jc                     int          `json:"jc"`
		Jmin                   int          `json:"jmin"`
		Jmax                   int          `json:"jmax"`
		S1                     int          `json:"s1"`
		S2                     int          `json:"s2"`
		S3                     int          `json:"s3"`
		S4                     int          `json:"s4"`
		H1                     string       `json:"h1"`
		H2                     string       `json:"h2"`
		H3                     string       `json:"h3"`
		H4                     string       `json:"h4"`
		I1                     string       `json:"i1"`
		I2                     string       `json:"i2"`
		I3                     string       `json:"i3"`
		I4                     string       `json:"i4"`
		I5                     string       `json:"i5"`
		HeaderProtectionKey    string       `json:"headerProtectionKey"`
		AwgVersion             string       `json:"awgVersion"`
		ContentPaddingAddition awg.AwgTimer `json:"contentPaddingAddition"`
		RekeyAfterTime         awg.AwgTimer `json:"rekeyAfterTime"`
		RekeyTimeout           awg.AwgTimer `json:"rekeyTimeout"`
		RejectAfterTime        awg.AwgTimer `json:"rejectAfterTime"`
		KeepaliveTimeout       awg.AwgTimer `json:"keepaliveTimeout"`
		MaxHandshakeAttempts   awg.AwgTimer `json:"maxHandshakeAttempts"`
		RandomTrailers         bool         `json:"randomTrailers"`
		DisableCookies         bool         `json:"disableCookies"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return "", "", ""
	}
	var b strings.Builder
	if s.Jc > 0 {
		fmt.Fprintf(&b, "Jc = %d\n", s.Jc)
	}
	if s.Jmin > 0 {
		fmt.Fprintf(&b, "Jmin = %d\n", s.Jmin)
	}
	if s.Jmax > 0 {
		fmt.Fprintf(&b, "Jmax = %d\n", s.Jmax)
	}
	// Written always, zeros included, like renderServerConf: 0 means "do not
	// pad", and a dropped line makes the client pad to its own default instead.
	fmt.Fprintf(&b, "S1 = %d\n", s.S1)
	fmt.Fprintf(&b, "S2 = %d\n", s.S2)
	ver := awg.NormalizeAWGVersion(s.AwgVersion)
	// S3/S4 + I1-I5 are AWG v2+; keep the ceiling block aligned with the
	// server conf and with filterAwgObfuscation so v1.5 must-match holds.
	if ver != "1.5" {
		fmt.Fprintf(&b, "S3 = %d\n", s.S3)
		fmt.Fprintf(&b, "S4 = %d\n", s.S4)
	}
	for i, h := range []string{s.H1, s.H2, s.H3, s.H4} {
		if strings.TrimSpace(h) != "" {
			fmt.Fprintf(&b, "H%d = %s\n", i+1, h)
		}
	}
	var out strings.Builder
	out.WriteString(b.String())
	// Same all-or-nothing gate the two .conf renderers use: an oversized set
	// silently vanishes from the real interface, so it must vanish here too.
	iFieldsFit := awg.IBytes(s.I1, s.I2, s.I3, s.I4, s.I5) <= awg.WorstCaseIBytesBudget(strings.TrimSpace(s.HeaderProtectionKey) != "")
	if ver != "1.5" && iFieldsFit {
		for _, ip := range []struct{ idx, val string }{
			{"1", s.I1}, {"2", s.I2}, {"3", s.I3}, {"4", s.I4}, {"5", s.I5},
		} {
			// Per field, not all-or-nothing: grammar is a property of the
			// value, where the budget above is a property of the whole set.
			if awg.PortableIField(ip.val) {
				fmt.Fprintf(&out, "I%s = %s\n", ip.idx, strings.TrimSpace(ip.val))
			}
		}
	}
	// HeaderProtectionKey (AWG3) is emitted ONLY when awgVersion == "3" and the
	// key is non-empty — this obfuscation block represents the inbound's
	// "ceiling" (the full field set for version 3). The clients page then
	// filters it down to the export version chosen in the QR/info modal
	// (filterAwgObfuscation in wireguardConfig.ts). Upstream kernel
	// v3.0.20260731 + tools v3.0.20260730 parse the field; older builds reject
	// it, so v1/v2 inbounds must never carry it. S1-S4 >= 12 is required for the
	// kernel to accept the key (enforced by the generator for v3).
	if strings.TrimSpace(s.HeaderProtectionKey) != "" && awg.AwgVersionFieldsAllowed(awg.IsAwg3Plus(s.AwgVersion), localInbound, awg.ModuleSupportsAwg3()) {
		fmt.Fprintf(&out, "HeaderProtectionKey = %s\n", s.HeaderProtectionKey)
	}
	// AWG3 device-level timers/padding — empty/"0" = kernel default. Emitted only
	// for v3+ so the clients-page filterAwgObfuscation can drop them for < v3.
	// Values are written verbatim (a single "150" or an inclusive range
	// "100-500"); this ceiling block mirrors the H1-H4 ranges already exported,
	// so client configs carry native kernel ranges intact.
	if awg.AwgVersionFieldsAllowed(awg.IsAwg3Plus(s.AwgVersion), localInbound, awg.ModuleSupportsAwg3()) {
		if !s.ContentPaddingAddition.IsZero() {
			fmt.Fprintf(&out, "ContentPaddingAddition = %s\n", s.ContentPaddingAddition)
		}
		if !s.RekeyAfterTime.IsZero() {
			fmt.Fprintf(&out, "RekeyAfterTime = %s\n", s.RekeyAfterTime)
		}
		if !s.RekeyTimeout.IsZero() {
			fmt.Fprintf(&out, "RekeyTimeout = %s\n", s.RekeyTimeout)
		}
		if !s.RejectAfterTime.IsZero() {
			fmt.Fprintf(&out, "RejectAfterTime = %s\n", s.RejectAfterTime)
		}
		if !s.KeepaliveTimeout.IsZero() {
			fmt.Fprintf(&out, "KeepaliveTimeout = %s\n", s.KeepaliveTimeout)
		}
		if !s.MaxHandshakeAttempts.IsZero() {
			fmt.Fprintf(&out, "MaxHandshakeAttempts = %s\n", s.MaxHandshakeAttempts)
		}
	}
	if awg.AwgVersionFieldsAllowed(awg.IsAwg31(s.AwgVersion), localInbound, awg.ModuleSupportsAwg31()) {
		if s.RandomTrailers {
			out.WriteString("RandomTrailers = on\n")
		}
		if s.DisableCookies {
			out.WriteString("DisableCookies = on\n")
		}
	}
	return s.Address, out.String(), awg.NormalizeAWGVersion(s.AwgVersion)
}

// InboundAwgPeerAddresses maps email → first AllowedIPs entry for each client
// stored on this AWG inbound. Used by the clients-page .conf builder and
// subscription export so multi-attach peers get the tunnel IP for THIS inbound,
// not the single clients-table field shared across all attachments.
func InboundAwgPeerAddresses(settings string) map[string]string {
	if strings.TrimSpace(settings) == "" {
		return nil
	}
	var s struct {
		Clients []struct {
			Email      string   `json:"email"`
			AllowedIPs []string `json:"allowedIPs"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil
	}
	out := make(map[string]string, len(s.Clients))
	for _, c := range s.Clients {
		email := strings.TrimSpace(c.Email)
		if email == "" || len(c.AllowedIPs) == 0 {
			continue
		}
		if ip := strings.TrimSpace(c.AllowedIPs[0]); ip != "" {
			out[email] = ip
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// LUCX-HOOK: protocols whose datapath is a sidecar, not an Xray inbound.
func inboundHasSidecar(p model.Protocol) bool {
	switch p {
	case model.AWG, model.MTProto, model.Naive, model.Olcrtc, model.Qwdtt, model.Csqtt, model.Mieru, model.TrustTunnel, model.Anytls, model.Tproxy, model.Cover, model.Gateway:
		return true
	default:
		return false
	}
}

// LUCX-HOOK: awgRoutesThroughXray reports whether an AWG inbound is configured to egress
// through the core's router (the TUN bridge in §xray.go). Such inbounds live
// only in the generated config, so every mutation of one must force a config
// regen — the kernel sidecar push alone never touches Xray.
func awgRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.AWG {
		return false
	}
	var parsed struct {
		RouteThroughXray bool `json:"routeThroughXray"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return false
	}
	return parsed.RouteThroughXray
}

// LUCX-HOOK
// naiveRoutesThroughXray reports whether a Naive inbound uses the SOCKS bridge.
func naiveRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.Naive {
		return false
	}
	var parsed struct {
		RouteThroughXray bool `json:"routeThroughXray"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return false
	}
	return parsed.RouteThroughXray
}

// LUCX-HOOK
// qwdttRoutesThroughXray reports whether qWDTT uses the Xray TUN bridge.
func qwdttRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.Qwdtt {
		return false
	}
	cfg, ok := tunnel.QwdttConfigFromInbound(inbound)
	return ok && cfg.RouteThroughXray
}

func csqttRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.Csqtt {
		return false
	}
	cfg, ok := tunnel.CsqttConfigFromInbound(inbound)
	return ok && cfg.RouteThroughXray
}

// olcrtcRoutesThroughXray reports whether olcRTC uses the SOCKS bridge.
func olcrtcRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.Olcrtc {
		return false
	}
	cfg, ok := tunnel.OlcrtcConfigFromInbound(inbound)
	return ok && cfg.RouteThroughXray && cfg.RouteXrayPort > 0
}

// checkVkTurnExclusive rejects a second qWDTT/CSQTT inbound on the same host
// and rejects mixing the two (overlapping 10.66 TUN). ignoreId=0 on create.
func (s *InboundService) checkVkTurnExclusive(creating model.Protocol, ignoreId int, nodeID *int) error {
	if creating != model.Qwdtt && creating != model.Csqtt {
		return nil
	}
	db := database.GetDB()
	count := func(p model.Protocol, skipSelf bool) (int64, error) {
		q := db.Model(&model.Inbound{}).Where("protocol = ?", p)
		if skipSelf && ignoreId > 0 {
			q = q.Where("id <> ?", ignoreId)
		}
		if nodeID == nil {
			q = q.Where("node_id IS NULL")
		} else {
			q = q.Where("node_id = ?", *nodeID)
		}
		var n int64
		err := q.Count(&n).Error
		return n, err
	}
	n, err := count(creating, true)
	if err != nil {
		return err
	}
	if n > 0 {
		name := "qWDTT"
		if creating == model.Csqtt {
			name = "CSQTT"
		}
		return common.NewError(name, "supports only one inbound per host")
	}
	other := model.Csqtt
	if creating == model.Csqtt {
		other = model.Qwdtt
	}
	n, err = count(other, false)
	if err != nil {
		return err
	}
	if n > 0 {
		return common.NewError("CSQTT and qWDTT cannot run on the same host (overlapping 10.66 TUN)")
	}
	return nil
}

func (s *InboundService) ensureNodeSupportsProtocol(protocol model.Protocol, nodeID *int) error {
	if nodeID == nil || *nodeID <= 0 {
		return nil
	}
	proto := string(protocol)
	if !nodetype.IsLucXOnlyProtocol(proto) {
		return nil
	}
	n, err := (&NodeService{}).GetById(*nodeID)
	if err != nil {
		return common.NewError("node", *nodeID, "not found for LucX protocol", proto)
	}
	info := nodetype.FromJSON(n.Features)
	if n.NodeType != "" {
		info.NodeType = n.NodeType
	}
	if info.NodeType == "" {
		info = nodetype.FromPanelVersion(n.PanelVersion)
	}
	if !info.SupportsProtocol(proto) {
		return common.NewError("protocol", proto, "requires a LucX-UI node with feature", proto, "— node", n.Name, "is", info.NodeType)
	}
	return nil
}

// ensureNodeAuthSeed mints settings.authSeed for HMAC sidecars on a node so
// the master's sub and the node's sidecar share one key. Local inbounds stay
// on HMAC(panel secret, id). Persists before push; skip the in-memory seed
// if the write fails so the next tick cannot rotate it.
func (s *InboundService) ensureNodeAuthSeed(ib *model.Inbound) {
	if ib == nil || ib.NodeID == nil || !tunnel.UsesDerivedAuth(ib.Protocol) {
		return
	}
	if tunnel.AuthSeed(ib.Settings) != "" {
		return
	}
	next, changed := tunnel.EnsureAuthSeed(ib.Settings)
	if !changed {
		return
	}
	if ib.Id > 0 {
		if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", next).Error; err != nil {
			logger.Warning("authSeed persist failed for inbound", ib.Id, ":", err)
			return
		}
	}
	ib.Settings = next
}

// normalizeOlcrtcSettings ensures cryptoKey on save and coerces Telemost to
// vp8channel (datachannel is rejected by Validate and left the process stopped
// forever while the form still showed "enabled" — Vlad thrash 2026-08-12).
func (s *InboundService) normalizeOlcrtcSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.OlcrtcConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = tunnel.CoerceOlcrtcTransport(cfg.Merge())
	cfg = cfg.ClampVP8()
	if c2, err := cfg.EnsureCryptoKey(); err == nil {
		cfg = c2
	}
	if bs, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	if inbound.Remark == "" && cfg.Remark != "" {
		inbound.Remark = cfg.Remark
	}
}

// normalizeQwdttSettings ensures password + public peer (subHost) and syncs
// Port from listenAddr. subHost is required for qwdtt:// ClientURI — without
// it the inbound saves but export/QR/sub stay empty (tester report: "inbound
// creates but nothing to share").
func (s *InboundService) normalizeQwdttSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.QwdttConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	if c2, err := cfg.EnsurePassword(); err == nil {
		cfg = c2
	}
	cfg = cfg.EnsureSubHost()
	if bs, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	if p := tunnel.QwdttDTLSPort(cfg); p > 0 {
		inbound.Port = p
	}
	if inbound.Remark == "" && cfg.Remark != "" {
		inbound.Remark = cfg.Remark
	}
}

func (s *InboundService) normalizeCsqttSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.CsqttConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	if c2, err := cfg.EnsurePassword(); err == nil {
		cfg = c2
	}
	if c2, err := cfg.EnsureWebPass(); err == nil {
		cfg = c2
	}
	cfg = cfg.EnsureSubHost()
	if bs, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	if p := tunnel.CsqttListenPort(cfg); p > 0 {
		inbound.Port = p
	}
	if inbound.Remark == "" && cfg.Remark != "" {
		inbound.Remark = cfg.Remark
	}
}

func settingsIntKey(parsed map[string]any, key string) int {
	switch v := parsed[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

func parseSettingsIntKey(settings string, key string) int {
	if settings == "" {
		return 0
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return 0
	}
	return settingsIntKey(parsed, key)
}

// normalizeNaiveXrayPort allocates/persists the SOCKS bridge port for Naive
// inbounds with routeThroughXray (same logic as mtproto).
func (s *InboundService) normalizeNaiveXrayPort(inbound *model.Inbound, oldSettings string) error {
	return s.normalizeSidecarXrayPort(inbound, oldSettings, model.Naive, "naive")
}

// normalizeOlcrtcXrayPort allocates/persists the SOCKS bridge port for olcRTC
// (binary dials via socks: proxy_addr/port in YAML).
func (s *InboundService) normalizeOlcrtcXrayPort(inbound *model.Inbound, oldSettings string) error {
	return s.normalizeSidecarXrayPort(inbound, oldSettings, model.Olcrtc, "olcrtc")
}

// normalizeMieruXrayPort allocates/persists the SOCKS bridge port for mieru
// (mita dials via native egress.proxies SOCKS5).
func (s *InboundService) normalizeMieruXrayPort(inbound *model.Inbound, oldSettings string) error {
	return s.normalizeSidecarXrayPort(inbound, oldSettings, model.Mieru, "mieru")
}

// mieruRoutesThroughXray reports whether mieru uses the Xray SOCKS bridge.
func mieruRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.Mieru {
		return false
	}
	cfg, ok := tunnel.MieruConfigFromInbound(inbound)
	return ok && cfg.RouteThroughXray && cfg.RouteXrayPort > 0
}

// trustTunnelRoutesThroughXray reports whether TrustTunnel uses the Xray
// SOCKS bridge ([forward_protocol.socks5]).
func trustTunnelRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.TrustTunnel {
		return false
	}
	cfg, ok := tunnel.TrustTunnelConfigFromInbound(inbound)
	return ok && cfg.RouteThroughXray && cfg.RouteXrayPort > 0
}

func retargetAutoTag(c, snapIb *model.Inbound) string {
	if c == nil || snapIb == nil || c.Port == snapIb.Port || !autoGeneratedInboundTag(c) {
		return ""
	}
	bits := inboundTransports(snapIb.Protocol, snapIb.StreamSettings, snapIb.Settings)
	return composeInboundTag(snapIb.Port, c.NodeID, bits)
}

func autoGeneratedInboundTag(ib *model.Inbound) bool {
	if ib == nil {
		return false
	}
	bits := inboundTransports(ib.Protocol, ib.StreamSettings, ib.Settings)
	if isAutoGeneratedTag(ib.Tag, ib.Port, ib.NodeID, bits) {
		return true
	}
	return isAutoGeneratedTag(ib.Tag, ib.Port, nil, bits)
}

func lucxRuntimeSidecar(p model.Protocol) bool {
	switch p {
	case model.AWG, model.AmneziaWG, model.Naive, model.Olcrtc, model.Qwdtt, model.Csqtt,
		model.Mieru, model.TrustTunnel, model.Anytls, model.Tproxy, model.Cover, model.Gateway:
		return true
	}
	return false
}

// LUCX-HOOK: any LucX sidecar whose egress bridge lives only in generated Xray JSON.
func lucxRoutesThroughXray(inbound *model.Inbound) bool {
	return awgRoutesThroughXray(inbound) ||
		naiveRoutesThroughXray(inbound) ||
		qwdttRoutesThroughXray(inbound) ||
		csqttRoutesThroughXray(inbound) ||
		olcrtcRoutesThroughXray(inbound) ||
		mieruRoutesThroughXray(inbound) ||
		trustTunnelRoutesThroughXray(inbound) ||
		tproxyRoutesThroughXray(inbound)
}

func tproxyRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.Tproxy {
		return false
	}
	cfg, ok := tunnel.TproxyConfigFromInbound(inbound)
	return ok && cfg.RouteThroughXray && cfg.RouteXrayPort > 0
}

// normalizeMieruSettings merges defaults into the stored settings WITHOUT
// touching clients[] (multi-client inbound — the config struct has no client
// field, so a plain re-marshal would drop them). Syncs inbound.Port to the
// primary binding so the generic port bookkeeping has a single value.
func (s *InboundService) normalizeMieruSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.MieruConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	var settings map[string]any
	if raw := strings.TrimSpace(inbound.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	settings["portBindings"] = cfg.PortBindings
	settings["mtu"] = cfg.MTU
	settings["loggingLevel"] = cfg.LoggingLevel
	settings["routeThroughXray"] = cfg.RouteThroughXray
	settings["routeXrayPort"] = cfg.RouteXrayPort
	settings["outboundTag"] = cfg.OutboundTag
	// Optional traffic shaping: written only when set, deleted when cleared,
	// so pre-feature inbounds keep their stored settings byte-identical.
	if m := strings.TrimSpace(cfg.Multiplexing); m != "" {
		settings["multiplexing"] = m
	} else {
		delete(settings, "multiplexing")
	}
	if h := strings.TrimSpace(cfg.HandshakeMode); h != "" {
		settings["handshakeMode"] = h
	} else {
		delete(settings, "handshakeMode")
	}
	if tp := cfg.TrafficPattern.Normalized(); tp != nil {
		settings["trafficPattern"] = tp
	} else {
		delete(settings, "trafficPattern")
	}
	if strings.TrimSpace(cfg.Remark) != "" {
		settings["remark"] = cfg.Remark
	}
	if bs, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	inbound.Port = tunnel.MieruPrimaryPort(cfg)
	if inbound.Remark == "" && strings.TrimSpace(cfg.Remark) != "" {
		inbound.Remark = cfg.Remark
	}
}

// normalizeAnytlsSettings merges defaults into the stored settings and mints
// the shared password on first save. Syncs inbound.Port to the TCP listen
// port so the generic port bookkeeping has a single value.
func (s *InboundService) normalizeAnytlsSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.AnytlsConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	if inbound.Port > 0 {
		cfg.Port = inbound.Port
	}
	if strings.TrimSpace(cfg.Password) == "" {
		if c2, err := cfg.EnsurePassword(); err == nil {
			cfg = c2
		}
	}
	var settings map[string]any
	if raw := strings.TrimSpace(inbound.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	settings["port"] = cfg.Port
	settings["password"] = cfg.Password
	settings["sni"] = strings.TrimSpace(cfg.SNI)
	settings["certFile"] = strings.TrimSpace(cfg.CertFile)
	settings["keyFile"] = strings.TrimSpace(cfg.KeyFile)
	if strings.TrimSpace(cfg.Remark) != "" {
		settings["remark"] = cfg.Remark
	}
	if bs, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	inbound.Port = tunnel.AnytlsPrimaryPort(cfg)
	if inbound.Remark == "" && strings.TrimSpace(cfg.Remark) != "" {
		inbound.Remark = cfg.Remark
	}
}

func (s *InboundService) validateAnytlsCert(inbound *model.Inbound) error {
	cfg, ok := tunnel.AnytlsConfigFromInbound(inbound)
	if !ok {
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	panelCert, panelKey := panelCertFiles()
	return cfg.ValidateCert(panelCert, panelKey)
}

func (s *InboundService) normalizeTproxySettings(inbound *model.Inbound) {
	cfg, ok := tunnel.TproxyConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	if inbound.Port > 0 {
		cfg.Port = inbound.Port
	}
	if strings.TrimSpace(cfg.Secret) == "" {
		if c2, err := cfg.EnsureSecret(); err == nil {
			cfg = c2
		}
	}
	var settings map[string]any
	if raw := strings.TrimSpace(inbound.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	settings["port"] = cfg.Port
	settings["hostname"] = strings.TrimSpace(cfg.Hostname)
	settings["secret"] = strings.TrimSpace(cfg.Secret)
	settings["siteSource"] = cfg.SiteSource
	settings["siteDir"] = strings.TrimSpace(cfg.SiteDir)
	settings["siteUpstream"] = strings.TrimSpace(cfg.SiteUpstream)
	settings["carrierMode"] = cfg.CarrierMode
	settings["certFile"] = strings.TrimSpace(cfg.CertFile)
	settings["keyFile"] = strings.TrimSpace(cfg.KeyFile)
	settings["routeThroughXray"] = cfg.RouteThroughXray
	settings["routeXrayPort"] = cfg.RouteXrayPort
	settings["outboundTag"] = strings.TrimSpace(cfg.OutboundTag)
	if strings.TrimSpace(cfg.Remark) != "" {
		settings["remark"] = cfg.Remark
	}
	if bs, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	inbound.Port = tunnel.TproxyPrimaryPort(cfg)
	if inbound.Remark == "" && strings.TrimSpace(cfg.Remark) != "" {
		inbound.Remark = cfg.Remark
	}
}

func (s *InboundService) normalizeTproxyXrayPort(inbound *model.Inbound, oldSettings string) error {
	return s.normalizeSidecarXrayPort(inbound, oldSettings, model.Tproxy, "tproxy")
}

func (s *InboundService) normalizeCoverSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.CoverConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	var settings map[string]any
	if raw := strings.TrimSpace(inbound.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	settings["hostname"] = strings.TrimSpace(cfg.Hostname)
	settings["siteSource"] = cfg.SiteSource
	settings["siteDir"] = strings.TrimSpace(cfg.SiteDir)
	settings["siteUpstream"] = strings.TrimSpace(cfg.SiteUpstream)
	settings["certFile"] = strings.TrimSpace(cfg.CertFile)
	settings["keyFile"] = strings.TrimSpace(cfg.KeyFile)
	settings["routes"] = cfg.Routes
	if strings.TrimSpace(cfg.Remark) != "" {
		settings["remark"] = cfg.Remark
	}
	if bs, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	if !tunnel.IsLoopbackListen(inbound.Listen) {
		inbound.Port = 443
	}
	if inbound.Remark == "" && strings.TrimSpace(cfg.Remark) != "" {
		inbound.Remark = cfg.Remark
	}
}

func (s *InboundService) validateCoverSettings(inbound *model.Inbound) error {
	cfg, ok := tunnel.CoverConfigFromInbound(inbound)
	if !ok {
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.SiteSource == "dir" {
		if err := tunnel.RequireIndexHTML(strings.TrimSpace(cfg.SiteDir)); err != nil {
			return err
		}
	}
	if cfg.SiteSource == "upstream" {
		return nil
	}
	panelCert, panelKey := panelCertFiles()
	return cfg.ValidateCert(panelCert, panelKey)
}

func (s *InboundService) checkSingleCover(inbound *model.Inbound, ignoreId int) error {
	if inbound == nil || inbound.Protocol != model.Cover || inbound.NodeID != nil {
		return nil
	}
	inbounds, err := s.GetAllInbounds()
	if err != nil {
		return err
	}
	for _, o := range inbounds {
		if o == nil || o.Protocol != model.Cover || o.NodeID != nil || o.Id == ignoreId {
			continue
		}
		return common.NewError("cover: only one cover inbound per host (:80 and :443)")
	}
	return nil
}

func (s *InboundService) normalizeGatewaySettings(inbound *model.Inbound) {
	cfg, ok := tunnel.GatewayConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	if inbound.Port <= 0 {
		inbound.Port = 443
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	inbound.Settings = string(body)
}

func (s *InboundService) checkSingleGateway(inbound *model.Inbound, ignoreId int) error {
	if inbound == nil || inbound.Protocol != model.Gateway || inbound.NodeID != nil {
		return nil
	}
	inbounds, err := s.GetAllInbounds()
	if err != nil {
		return err
	}
	for _, o := range inbounds {
		if o == nil || o.Protocol != model.Gateway || o.NodeID != nil || o.Id == ignoreId {
			continue
		}
		return common.NewError("gateway: only one SNI gateway inbound per host")
	}
	return nil
}

func (s *InboundService) validateTproxySettings(inbound *model.Inbound) error {
	cfg, ok := tunnel.TproxyConfigFromInbound(inbound)
	if !ok {
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.SiteSource == "dir" {
		if err := tunnel.RequireIndexHTML(strings.TrimSpace(cfg.SiteDir)); err != nil {
			return err
		}
	}
	if cfg.SiteSource == "upstream" {
		return nil
	}
	panelCert, panelKey := panelCertFiles()
	return cfg.ValidateCert(panelCert, panelKey)
}

func inboundListenRanges(ib *model.Inbound) [][2]int {
	if ib == nil {
		return nil
	}
	if ib.Protocol == model.Mieru {
		cfg, ok := tunnel.MieruConfigFromInbound(ib)
		if !ok {
			if ib.Port > 0 {
				return [][2]int{{ib.Port, ib.Port}}
			}
			return nil
		}
		var out [][2]int
		for _, b := range cfg.Merge().PortBindings {
			if strings.TrimSpace(b.PortRange) != "" {
				lo, hi, ok := tunnel.MieruPortRangeBounds(b.PortRange)
				if ok {
					out = append(out, [2]int{lo, hi})
				}
				continue
			}
			if b.Port > 0 {
				out = append(out, [2]int{b.Port, b.Port})
			}
		}
		return out
	}
	if ib.Port > 0 {
		return [][2]int{{ib.Port, ib.Port}}
	}
	return nil
}

// checkMieruPortConflict rejects bindings colliding with other local
// inbounds. TCP bindings collide with every non-UDP-only listener; UDP
// bindings collide only with UDP-capable listeners (wireguard/AWG/hysteria,
// another mieru UDP binding, TrustTunnel QUIC) — TCP and UDP coexist on one
// port number. Node inbounds listen elsewhere and are skipped.
func (s *InboundService) checkMieruPortConflict(inbound *model.Inbound, ignoreId int) error {
	cfg, ok := tunnel.MieruConfigFromInbound(inbound)
	if !ok {
		return nil
	}
	cfg = cfg.Merge()
	inbounds, err := s.GetAllInbounds()
	if err != nil {
		return err
	}
	for _, b := range cfg.PortBindings {
		lo, hi := b.Port, b.Port
		if strings.TrimSpace(b.PortRange) != "" {
			var rngOK bool
			lo, hi, rngOK = tunnel.MieruPortRangeBounds(b.PortRange)
			if !rngOK {
				continue
			}
		}
		if lo <= 0 {
			continue
		}
		if webPort, err := (&SettingService{}).GetPort(); err == nil && webPort >= lo && webPort <= hi {
			return common.NewErrorf("mieru: port %d-%d collides with the panel itself", lo, hi)
		}
		udp := strings.EqualFold(strings.TrimSpace(b.Protocol), "UDP")
		for _, other := range inbounds {
			if other == nil || other.Id == ignoreId || other.Id == inbound.Id || other.NodeID != nil {
				continue
			}
			hit := 0
			for _, r := range inboundListenRanges(other) {
				if lo <= r[1] && r[0] <= hi {
					hit = r[0]
					break
				}
			}
			if hit == 0 {
				continue
			}
			udpOnly := other.Protocol == model.WireGuard || other.Protocol == model.AWG ||
				other.Protocol == model.AmneziaWG || other.Protocol == model.Hysteria
			if udp {
				if udpOnly || other.Protocol == model.Mieru || other.Protocol == model.TrustTunnel {
					return common.NewErrorf("mieru: UDP port %d-%d collides with inbound %q (port %d)", lo, hi, other.Remark, hit)
				}
				continue
			}
			if !udpOnly {
				return common.NewErrorf("mieru: TCP port %d-%d collides with inbound %q (port %d)", lo, hi, other.Remark, hit)
			}
		}
	}
	return nil
}

// normalizeTrustTunnelSettings merges defaults into the stored settings
// without touching clients[] and syncs inbound.Port from the listen address.
func (s *InboundService) normalizeTrustTunnelSettings(inbound *model.Inbound) {
	cfg, ok := tunnel.TrustTunnelConfigFromInbound(inbound)
	if !ok {
		return
	}
	cfg = cfg.Merge()
	cfg.EnsureClientRandomPrefix()
	var settings map[string]any
	if raw := strings.TrimSpace(inbound.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	settings["hostname"] = cfg.Hostname
	settings["listen"] = cfg.Listen
	settings["ipv6"] = cfg.IPv6
	settings["certFile"] = cfg.CertFile
	settings["keyFile"] = cfg.KeyFile
	settings["clientDns"] = cfg.ClientDNS
	settings["upstreamProtocol"] = cfg.UpstreamProtocol
	settings["routeThroughXray"] = cfg.RouteThroughXray
	settings["routeXrayPort"] = cfg.RouteXrayPort
	settings["outboundTag"] = cfg.OutboundTag
	settings["metricsPort"] = cfg.MetricsPort
	settings["listenPreset"] = cfg.ListenPreset
	settings["clientRandomPrefix"] = cfg.ClientRandomPrefix
	if strings.TrimSpace(cfg.Remark) != "" {
		settings["remark"] = cfg.Remark
	}
	if bs, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	inbound.Port = cfg.ListenPort()
	if inbound.Remark == "" && strings.TrimSpace(cfg.Remark) != "" {
		inbound.Remark = cfg.Remark
	}
}

// validateTrustTunnelCert rejects the save when the endpoint cannot start:
// no hostname, or the resolved certificate (explicit paths or the panel ACME
// cert) does not parse / cover the hostname / expired. Fail-fast by design —
// TrustTunnel without a trusted domain cert is unusable.
func (s *InboundService) validateTrustTunnelCert(inbound *model.Inbound) error {
	cfg, ok := tunnel.TrustTunnelConfigFromInbound(inbound)
	if !ok {
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	panelCert, panelKey := panelCertFiles()
	certFile, keyFile := cfg.ResolveCertPaths(panelCert, panelKey)
	return tunnel.ValidateCertFiles(certFile, keyFile, cfg.Hostname)
}

// normalizeTrustTunnelXrayPort allocates/persists the SOCKS bridge port for a
// routed TrustTunnel inbound (endpoint [forward_protocol.socks5] dials it).
func (s *InboundService) normalizeTrustTunnelXrayPort(inbound *model.Inbound, oldSettings string) error {
	return s.normalizeSidecarXrayPort(inbound, oldSettings, model.TrustTunnel, "trusttunnel")
}

// normalizeTrustTunnelMetricsPort allocates a stable loopback Prometheus port
// for traffic accounting (first save only; kept across edits).
func (s *InboundService) normalizeTrustTunnelMetricsPort(inbound *model.Inbound, oldSettings string) error {
	if inbound.Protocol != model.TrustTunnel {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil || parsed == nil {
		return nil
	}
	if port := settingsIntKey(parsed, "metricsPort"); port > 0 {
		return nil
	}
	if port := parseSettingsIntKey(oldSettings, "metricsPort"); port > 0 {
		parsed["metricsPort"] = port
	} else {
		free, err := mtproto.FreeLocalPort()
		if err != nil {
			return common.NewError("trusttunnel: allocate metrics port: ", err)
		}
		parsed["metricsPort"] = free
	}
	if bs, err := json.MarshalIndent(parsed, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
	return nil
}

// checkTrustTunnelPortConflict rejects a listen port already bound by another
// local inbound (TrustTunnel listens TCP+UDP/QUIC on one port, so both
// directions collide) and the panel's own HTTPS port.
func (s *InboundService) checkTrustTunnelPortConflict(inbound *model.Inbound, ignoreId int) error {
	cfg, ok := tunnel.TrustTunnelConfigFromInbound(inbound)
	if !ok {
		return nil
	}
	port := cfg.ListenPort()
	if port <= 0 {
		return nil
	}
	if webPort, err := (&SettingService{}).GetPort(); err == nil && webPort == port {
		return common.NewErrorf("trusttunnel: port %d is used by the panel itself", port)
	}
	inbounds, err := s.GetAllInbounds()
	if err != nil {
		return err
	}
	for _, other := range inbounds {
		if other == nil || other.Id == ignoreId || other.Id == inbound.Id || other.NodeID != nil {
			continue
		}
		for _, r := range inboundListenRanges(other) {
			if port >= r[0] && port <= r[1] {
				return common.NewErrorf("trusttunnel: port %d collides with inbound %q", port, other.Remark)
			}
		}
	}
	return nil
}

func (s *InboundService) normalizeSidecarXrayPort(inbound *model.Inbound, oldSettings string, proto model.Protocol, label string) error {
	if inbound.Protocol != proto {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil || parsed == nil {
		return nil
	}
	routed, _ := parsed["routeThroughXray"].(bool)
	if !routed {
		_, hadPort := parsed["routeXrayPort"]
		_, hadTag := parsed["outboundTag"]
		if !hadPort && !hadTag {
			return nil
		}
		delete(parsed, "routeXrayPort")
		delete(parsed, "outboundTag")
		if bs, err := json.MarshalIndent(parsed, "", "  "); err == nil {
			inbound.Settings = string(bs)
		} else {
			logger.Warning(label, ": failed to marshal settings after disabling routing:", err)
		}
		return nil
	}

	port := parseSettingsIntKey(oldSettings, "routeXrayPort")
	if port <= 0 && proto != model.Naive {
		port = settingsIntKey(parsed, "routeXrayPort")
	}
	if port <= 0 {
		allocated, err := mtproto.FreeLocalPort()
		if err != nil {
			return common.NewError(label+": could not allocate an Xray egress port:", err)
		}
		port = allocated
	}
	if settingsIntKey(parsed, "routeXrayPort") == port {
		return nil
	}
	parsed["routeXrayPort"] = port
	bs, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return common.NewError(label+": could not persist the Xray egress port:", err)
	}
	inbound.Settings = string(bs)
	return nil
}

// LUCX-HOOK: awgOutboundSubnetConflict reports whether the inbound tunnel
// subnet newNet collides with one AWG outbound's tunnel address outAddr. Only
// an outbound prefix no more specific than the inbound's (oP.Bits() <=
// newNet.Bits(), i.e. a /24 or wider when the inbound is a /24) installs a
// conflicting connected route; a bare /32 host address is exempt because it
// creates no /24 route of its own and defaultAwgClients already keeps client
// IPs off it. Pure (no DB) for unit testing. Returns the masked conflicting
// outbound prefix and true on a clash.
func awgOutboundSubnetConflict(newNet netip.Prefix, outAddr string) (netip.Prefix, bool) {
	outAddr = strings.TrimSpace(outAddr)
	if outAddr == "" {
		return netip.Prefix{}, false
	}
	oP, err := netip.ParsePrefix(outAddr)
	if err != nil {
		return netip.Prefix{}, false
	}
	if oP.Bits() <= newNet.Bits() && newNet.Overlaps(oP.Masked()) {
		return oP.Masked(), true
	}
	return netip.Prefix{}, false
}

// LUCX-HOOK: checkAwgSubnetConflict blocks an AWG inbound whose tunnel subnet
// overlaps another AWG inbound on the SAME host (local panel or the same node).
// Two awg interfaces on one kernel with the same connected subnet install
// duplicate routes; reverse path picks the wrong iface (Pattern 1e). Different
// nodes are separate kernels — same subnet is fine. ignoreId excludes the
// inbound being edited. Outbound clash applies only to local inbounds (outbounds
// live on the master kernel). Empty/unparseable address is not an error here.
func (s *InboundService) checkAwgSubnetConflict(newAddr string, ignoreId int, nodeID *int) error {
	return s.checkAwgSubnetConflictAllow(newAddr, ignoreId, nodeID, false)
}

func (s *InboundService) checkAwgSubnetConflictAllow(newAddr string, ignoreId int, nodeID *int, allowOverlap bool) error {
	if allowOverlap {
		return nil
	}
	newAddr = strings.TrimSpace(newAddr)
	if newAddr == "" {
		return nil
	}
	newP, err := netip.ParsePrefix(newAddr)
	if err != nil {
		return nil
	}
	newNet := newP.Masked()

	db := database.GetDB()
	var candidates []*model.Inbound
	q := db.Model(model.Inbound{}).Where("protocol = ?", model.AWG)
	if ignoreId > 0 {
		q = q.Where("id != ?", ignoreId)
	}
	if nodeID == nil {
		q = q.Where("node_id IS NULL")
	} else {
		q = q.Where("node_id = ?", *nodeID)
	}
	if err := q.Find(&candidates).Error; err != nil {
		return err
	}

	for _, c := range candidates {
		cAddr := awgSettingsAddress(c.Settings)
		if cAddr == "" {
			continue
		}
		cP, pErr := netip.ParsePrefix(cAddr)
		if pErr != nil {
			continue
		}
		if newNet.Overlaps(cP.Masked()) {
			label := c.Remark
			if label == "" {
				label = c.Tag
			}
			return common.NewError("AWG subnet", newNet.String(), "conflicts with inbound", label, "("+cP.Masked().String()+")", "— two AWG inbounds cannot share a tunnel subnet")
		}
	}

	// Outbounds are local to the master kernel — only local inbounds clash.
	if nodeID == nil {
		if outAddrs, oErr := (&AwgOutboundService{}).outboundAddresses(false); oErr == nil {
			for _, oAddr := range outAddrs {
				if oNet, clash := awgOutboundSubnetConflict(newNet, oAddr); clash {
					return common.NewError("AWG subnet", newNet.String(), "conflicts with AWG outbound tunnel", oNet.String(), "— the upstream server's subnet overlaps this inbound's tunnel subnet")
				}
			}
		}
	}
	return nil
}

func (s *InboundService) addInbound(inbound *model.Inbound, allowAwgOverlap bool) (*model.Inbound, bool, error) {
	prev := s.allowAwgOverlap
	s.allowAwgOverlap = allowAwgOverlap
	defer func() { s.allowAwgOverlap = prev }()
	return s.AddInbound(inbound)
}

func (s *InboundService) normalizeLucxSidecarsOnCreate(inbound *model.Inbound) error {
	if err := s.normalizeNaiveXrayPort(inbound, ""); err != nil {
		return err
	}
	if inbound.Protocol == model.Qwdtt {
		if err := s.checkVkTurnExclusive(model.Qwdtt, 0, inbound.NodeID); err != nil {
			return err
		}
		s.normalizeQwdttSettings(inbound)
	}
	if inbound.Protocol == model.Csqtt {
		if err := s.checkVkTurnExclusive(model.Csqtt, 0, inbound.NodeID); err != nil {
			return err
		}
		s.normalizeCsqttSettings(inbound)
	}
	if inbound.Protocol == model.Olcrtc {
		s.normalizeOlcrtcSettings(inbound)
		inbound.Port = 0
		if err := s.normalizeOlcrtcXrayPort(inbound, ""); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Mieru {
		s.normalizeMieruSettings(inbound)
		if cfg, ok := tunnel.MieruConfigFromInbound(inbound); ok {
			if err := cfg.Merge().Validate(); err != nil {
				return err
			}
		}
		if err := s.checkMieruPortConflict(inbound, 0); err != nil {
			return err
		}
		if err := s.normalizeMieruXrayPort(inbound, ""); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.TrustTunnel {
		s.normalizeTrustTunnelSettings(inbound)
		if err := s.checkTrustTunnelPortConflict(inbound, 0); err != nil {
			return err
		}
		if err := s.validateTrustTunnelCert(inbound); err != nil {
			return err
		}
		if err := s.normalizeTrustTunnelXrayPort(inbound, ""); err != nil {
			return err
		}
		if err := s.normalizeTrustTunnelMetricsPort(inbound, ""); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Anytls {
		s.normalizeAnytlsSettings(inbound)
		if err := s.validateAnytlsCert(inbound); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Tproxy {
		s.normalizeTproxySettings(inbound)
		if err := s.validateTproxySettings(inbound); err != nil {
			return err
		}
		if err := s.normalizeTproxyXrayPort(inbound, ""); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Cover {
		s.normalizeCoverSettings(inbound)
		if err := s.validateCoverSettings(inbound); err != nil {
			return err
		}
		if err := s.checkSingleCover(inbound, 0); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Gateway {
		s.normalizeGatewaySettings(inbound)
		if err := s.checkSingleGateway(inbound, 0); err != nil {
			return err
		}
	}
	s.ensureNodeAuthSeed(inbound)
	return nil
}

func (s *InboundService) normalizeLucxSidecarsOnUpdate(inbound, oldInbound *model.Inbound) error {
	if inbound.Protocol == model.Qwdtt {
		if err := s.checkVkTurnExclusive(model.Qwdtt, inbound.Id, inbound.NodeID); err != nil {
			return err
		}
		s.normalizeQwdttSettings(inbound)
	}
	if inbound.Protocol == model.Csqtt {
		if err := s.checkVkTurnExclusive(model.Csqtt, inbound.Id, inbound.NodeID); err != nil {
			return err
		}
		s.normalizeCsqttSettings(inbound)
	}
	if inbound.Protocol == model.Olcrtc {
		s.normalizeOlcrtcSettings(inbound)
		inbound.Port = 0
		if err := s.normalizeOlcrtcXrayPort(inbound, oldInbound.Settings); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Mieru {
		s.normalizeMieruSettings(inbound)
		if cfg, ok := tunnel.MieruConfigFromInbound(inbound); ok {
			if err := cfg.Merge().Validate(); err != nil {
				return err
			}
		}
		if err := s.checkMieruPortConflict(inbound, inbound.Id); err != nil {
			return err
		}
		if err := s.normalizeMieruXrayPort(inbound, oldInbound.Settings); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.TrustTunnel {
		s.normalizeTrustTunnelSettings(inbound)
		if err := s.checkTrustTunnelPortConflict(inbound, inbound.Id); err != nil {
			return err
		}
		if err := s.validateTrustTunnelCert(inbound); err != nil {
			return err
		}
		if err := s.normalizeTrustTunnelXrayPort(inbound, oldInbound.Settings); err != nil {
			return err
		}
		if err := s.normalizeTrustTunnelMetricsPort(inbound, oldInbound.Settings); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Anytls {
		s.normalizeAnytlsSettings(inbound)
		if err := s.validateAnytlsCert(inbound); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Tproxy {
		s.normalizeTproxySettings(inbound)
		if err := s.validateTproxySettings(inbound); err != nil {
			return err
		}
		if err := s.normalizeTproxyXrayPort(inbound, oldInbound.Settings); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Cover {
		s.normalizeCoverSettings(inbound)
		if err := s.validateCoverSettings(inbound); err != nil {
			return err
		}
		if err := s.checkSingleCover(inbound, inbound.Id); err != nil {
			return err
		}
	}
	if inbound.Protocol == model.Gateway {
		s.normalizeGatewaySettings(inbound)
		if err := s.checkSingleGateway(inbound, inbound.Id); err != nil {
			return err
		}
	}
	inbound.Settings = tunnel.PreserveAuthSeed(oldInbound.Settings, inbound.Settings)
	s.ensureNodeAuthSeed(inbound)
	return nil
}

func defaultAwgInlineClients(inbound *model.Inbound, existing, clients []model.Client) error {
	if inbound.Protocol != model.AWG || len(clients) == 0 {
		return nil
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil || settings == nil {
		return nil
	}
	ic, ok := settings["clients"].([]any)
	if !ok {
		return nil
	}
	if err := defaultAwgClients(existing, clients, ic, awgSettingsAddress(inbound.Settings), awgSettingsVersion(inbound.Settings)); err != nil {
		return err
	}
	settings["clients"] = ic
	if bs, err := json.Marshal(settings); err == nil {
		inbound.Settings = string(bs)
	}
	return nil
}

func (s *InboundService) migrateAwgSettingsOnUpdate(inbound, oldInbound *model.Inbound) error {
	if inbound.Protocol != model.AWG {
		return nil
	}
	if err := validateAwgSettingsForSave(inbound.Settings, inbound.Tag); err != nil {
		return err
	}
	if oldInbound.Protocol == model.AWG {
		inbound.Settings = migrateAwgClientSubnets(
			awgSettingsAddress(oldInbound.Settings),
			awgSettingsAddress(inbound.Settings),
			inbound.Settings,
		)
	}
	oldAddr := awgSettingsAddress(oldInbound.Settings)
	newAddr := awgSettingsAddress(inbound.Settings)
	subnetChanged := true
	if oldP, oErr := netip.ParsePrefix(strings.TrimSpace(oldAddr)); oErr == nil {
		if newP, nErr := netip.ParsePrefix(strings.TrimSpace(newAddr)); nErr == nil {
			subnetChanged = oldP.Masked().String() != newP.Masked().String()
		}
	}
	if subnetChanged {
		if err := s.checkAwgSubnetConflict(newAddr, inbound.Id, inbound.NodeID); err != nil {
			return err
		}
	}
	return nil
}
