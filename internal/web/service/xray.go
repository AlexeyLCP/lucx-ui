package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	// LUCX-HOOK: AWG — restore policy routing synchronously after Xray restarts.
	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	// NaiveProxy tunnel SOCKS egress (injectTunnelEgress).
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	// END LUCX-HOOK

	"go.uber.org/atomic"
)

var (
	lock              sync.Mutex
	isNeedXrayRestart atomic.Bool // Indicates that restart was requested for Xray
	isManuallyStopped atomic.Bool // Indicates that Xray was stopped manually from the panel
	xrayState         xrayLifecycle
)

type xrayLifecycle struct {
	mu      sync.RWMutex
	process *xray.Process
	result  string
}

func (s *xrayLifecycle) snapshot() (*xray.Process, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.process, s.result
}

func (s *xrayLifecycle) replace(process *xray.Process) {
	s.mu.Lock()
	s.process = process
	s.result = ""
	s.mu.Unlock()
}

func (s *xrayLifecycle) storeResult(process *xray.Process, result string) {
	s.mu.Lock()
	if s.process == process && s.result == "" {
		s.result = result
	}
	s.mu.Unlock()
}

func currentXrayProcess() *xray.Process {
	process, _ := xrayState.snapshot()
	return process
}

// XrayService provides business logic for Xray process management.
// It handles starting, stopping, restarting Xray, and managing its configuration.
type XrayService struct {
	inboundService InboundService
	settingService SettingService
	nodeService    NodeService
	xrayAPI        xray.XrayAPI
}

// IsXrayRunning checks if the Xray process is currently running.
func (s *XrayService) IsXrayRunning() bool {
	process := currentXrayProcess()
	return process != nil && process.IsRunning()
}

// XrayProcess returns the current Xray process instance (may be nil when Xray
// is not running). It exposes the lifecycle snapshot to callers outside this
// package (e.g. the tgbot subpackage).
func XrayProcess() *xray.Process {
	return currentXrayProcess()
}

// GetXrayErr returns the error from the Xray process, if any.
func (s *XrayService) GetXrayErr() error {
	process := currentXrayProcess()
	if process == nil {
		return nil
	}

	err := process.GetErr()
	if err == nil {
		return nil
	}

	if runtime.GOOS == "windows" && err.Error() == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return nil
	}

	return err
}

// GetXrayResult returns the result string from the Xray process.
func (s *XrayService) GetXrayResult() string {
	process, cachedResult := xrayState.snapshot()
	if cachedResult != "" {
		return cachedResult
	}
	if process == nil || process.IsRunning() {
		return ""
	}

	result := process.GetResult()

	if runtime.GOOS == "windows" && result == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return ""
	}

	xrayState.storeResult(process, result)
	return result
}

// GetXrayVersion returns the version of the running Xray process.
func (s *XrayService) GetXrayVersion() string {
	process := currentXrayProcess()
	if process == nil {
		return "Unknown"
	}
	return process.GetXrayVersion()
}

// RemoveIndex removes an element at the specified index from a slice.
// Returns a new slice with the element removed.
func RemoveIndex(s []any, index int) []any {
	return append(s[:index], s[index+1:]...)
}

// GetXrayConfig retrieves and builds the Xray configuration from settings and inbounds.
func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}
	xrayConfig.LogConfig = resolveXrayLogPaths(xrayConfig.LogConfig)
	xrayConfig.API = ensureAPIServices(xrayConfig.API)
	xrayConfig.Policy = ensureStatsPolicy(xrayConfig.Policy)
	xrayConfig.RouterConfig = stripDisabledRules(xrayConfig.RouterConfig)
	// Template outbounds authored before the xray-core #6258 XHTTP rename may
	// still carry sessionPlacement/sessionKey; lift them too (same reason as
	// the per-inbound lift below).
	xrayConfig.OutboundConfigs = liftOutboundsXhttpSessionIDKeys(xrayConfig.OutboundConfigs)

	_, _, _ = s.inboundService.AddTraffic(nil, nil)

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		if inbound.NodeID != nil {
			continue
		}
		if inbound.Protocol == model.MTProto || inbound.Protocol == model.AmneziaWG {
			continue
		}
		// LUCX-HOOK: AWG is a kernel-interface sidecar; skip it when building
		// the Xray config, mirroring the MTProto exclusion above.
		if inbound.Protocol == model.AWG {
			continue
		}
		// Tunnel sidecars as inbounds — not Xray protocols.
		if inbound.Protocol == model.Naive || inbound.Protocol == model.Olcrtc || inbound.Protocol == model.Qwdtt ||
			inbound.Protocol == model.Mieru || inbound.Protocol == model.TrustTunnel || inbound.Protocol == model.Anytls {
			continue
		}
		// END LUCX-HOOK
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)

		dbClients, listErr := s.inboundService.clientService.ListForInbound(nil, inbound.Id)
		if listErr != nil {
			return nil, listErr
		}

		clientStats := inbound.ClientStats
		enableMap := make(map[string]bool, len(clientStats))
		for _, clientTraffic := range clientStats {
			enableMap[clientTraffic.Email] = clientTraffic.Enable
		}

		finalClients := make([]any, 0, len(dbClients))
		var wgPeers []any
		for i := range dbClients {
			c := dbClients[i]
			if enable, exists := enableMap[c.Email]; exists && !enable {
				logger.Infof("Remove Inbound User %s due to expiration or traffic limit", c.Email)
				continue
			}
			if !c.Enable {
				continue
			}
			flow := c.Flow
			if flow == "xtls-rprx-vision-udp443" {
				flow = "xtls-rprx-vision"
			}
			if inbound.DisableFlow {
				flow = ""
			}
			entry := map[string]any{"email": c.Email}
			switch inbound.Protocol {
			case model.VLESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if flow != "" {
					entry["flow"] = flow
				}
				if c.Reverse != nil {
					entry["reverse"] = c.Reverse
				}
			case model.VMESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if c.Security != "" {
					entry["security"] = c.Security
				}
			case model.Trojan:
				if c.Password != "" {
					entry["password"] = c.Password
				}
				if flow != "" {
					entry["flow"] = flow
				}
			case model.Shadowsocks:
				if c.Password != "" {
					entry["password"] = c.Password
				}
			case model.Hysteria:
				if c.Auth != "" {
					entry["auth"] = c.Auth
				}
			case model.WireGuard:
				wgPeers = append(wgPeers, model.WireguardPeerFromClient(c))
				continue
			}
			finalClients = append(finalClients, entry)
		}

		var mutated bool
		if inbound.Protocol == model.WireGuard {
			delete(settings, "clients")
			if wgPeers == nil {
				wgPeers = []any{}
			}
			settings["peers"] = wgPeers
			mutated = true
		} else {
			_, hadClients := settings["clients"]
			mutated = hadClients || len(finalClients) > 0
			if mutated {
				settings["clients"] = finalClients
			}
		}

		if inboundCanHostFallbacks(inbound) {
			fallbacks, fbErr := s.inboundService.fallbackService.BuildFallbacksJSON(nil, inbound.Id)
			if fbErr != nil {
				return nil, fbErr
			}
			if len(fallbacks) > 0 {
				generic := make([]any, 0, len(fallbacks))
				for _, f := range fallbacks {
					generic = append(generic, f)
				}
				settings["fallbacks"] = generic
				mutated = true
			}
		}

		if mutated {
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.Settings = string(modifiedSettings)
		}

		if len(inbound.StreamSettings) > 0 {
			// Unmarshal stream JSON
			var stream map[string]any
			_ = json.Unmarshal([]byte(inbound.StreamSettings), &stream)

			// Remove the "settings" field under "tlsSettings" and "realitySettings"
			tlsSettings, ok1 := stream["tlsSettings"].(map[string]any)
			realitySettings, ok2 := stream["realitySettings"].(map[string]any)
			if ok1 || ok2 {
				if ok1 {
					delete(tlsSettings, "settings")
				} else if ok2 {
					delete(realitySettings, "settings")
				}
			}

			delete(stream, "externalProxy")

			// finalmask.tcp + REALITY panics Xray-core on the first connection
			// (XTLS/Xray-core#6453). AddInbound/UpdateInbound reject this
			// combination at save time, but a row saved before that guard
			// existed (upgrade, node sync, restored backup, direct DB edit)
			// would still crash Xray on the next restart without this — drop
			// it here too, the same way liftXhttpSessionIDKeys and
			// HealShadowsocksClientMethods heal other legacy data in place.
			if len(finalMaskRealityTcpMasks(stream)) > 0 {
				logger.Warningf("Inbound %q: dropping finalmask, incompatible with REALITY security (crashes Xray-core, see XTLS/Xray-core#6453)", inbound.Tag)
				delete(stream, "finalmask")
			}

			dropEmptyRandPackets(stream["finalmask"])

			if dropped := stripIncompleteXmcMasks(stream); dropped > 0 {
				logger.Warningf("Inbound %q: dropping %d XMC finalmask mask(s) without complete Minecraft profiles — reconfigure them to restore the obfuscation (see XTLS/Xray-core#6487)", inbound.Tag, dropped)
			}

			// xray-core v26.6.22 (#6258) renamed the XHTTP session keys and
			// kept no fallback. Lift legacy sessionPlacement/sessionKey onto the
			// new names here so inbounds stored before the rename keep working
			// without the admin re-saving them.
			liftXhttpSessionIDKeys(stream)

			newStream, err := json.MarshalIndent(stream, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.StreamSettings = string(newStream)
		}

		if inbound.Protocol == model.Shadowsocks {
			if healed, ok := model.HealShadowsocksClientMethods(inbound.Settings); ok {
				inbound.Settings = healed
			}
		}

		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}

	// Merge subscription-derived outbounds (if any) into the final outbounds array.
	// These are additive: each subscription is placed before or after the template
	// outbounds based on its Prepend flag, ordered by Priority. Tags assigned by the
	// subscription service are kept stable across refreshes so that balancers and
	// routing rules continue to work.
	subSvc := &OutboundSubscriptionService{}
	if prepend, appendList, err := subSvc.activeOutboundsSplit(); err == nil && (len(prepend) > 0 || len(appendList) > 0) {
		mergeSubscriptionOutbounds(xrayConfig, prepend, appendList)
	}

	// LUCX-HOOK: AWG outbound freedom tags (awgo-N) must exist BEFORE SOCKS/TUN
	// egress injectors look up outboundTag. Previously they ran after TrustTunnel
	// / mieru / naive / mtproto injectors, so a selected AWG outbound like "SW"
	// was "not found" and injectSocksEgress skipped the whole bridge (VladufQa
	// lucx.133: TrustTunnel socks5 → :39431 with nothing listening).
	if awgOuts, err := (&AwgOutboundService{}).GetOutbounds(); err == nil {
		injectAwgOutbounds(xrayConfig, awgOuts)
	} else {
		logger.Warning("awg outbound: read enabled outbounds failed:", err)
	}
	if scOuts, err := (&SidecarOutboundService{}).GetOutbounds(); err == nil {
		injectSidecarOutbounds(xrayConfig, scOuts)
	} else {
		logger.Warning("sidecar outbound: read enabled outbounds failed:", err)
	}
	// END LUCX-HOOK

	// Route opted-in local mtproto inbounds through the core's router. Each one
	// gets a loopback SOCKS bridge — tagged with the inbound's own tag so it is
	// matchable in routing rules — that its mtg sidecar dials Telegram through.
	// Done after the subscription merge so a selected subscription outbound (or
	// balancer) is a valid rule target.
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.MTProto || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		injectMtprotoEgress(xrayConfig, inbound)
	}
	// LUCX-HOOK: Route opted-in local AWG inbounds through the core's router.
	// Each one gets a TUN inbound — tagged with the inbound's own tag so it is
	// matchable in routing rules — that the AWG kernel interface's decrypted
	// packets flow into. Mirrors the mtproto egress bridge but uses a TUN
	// inbound (IP packets from the kernel) instead of a SOCKS loopback (TCP
	// connections from a userspace sidecar). Done after the subscription merge
	// so a selected subscription outbound (or balancer) is a valid rule target.
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.AWG || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		if awg.KernelAvailable() {
			injectAwgEgress(xrayConfig, inbound)
		}
	}
	// qWDTT routeThroughXray: TUN bridge (kernel IP from wdtt0 → Xray), same idea as AWG.
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.Qwdtt || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		injectQwdttEgress(xrayConfig, inbound)
	}
	// olcRTC routeThroughXray: SOCKS bridge (binary socks: YAML → loopback SOCKS).
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.Olcrtc || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		injectOlcrtcEgress(xrayConfig, inbound)
	}
	// mieru routeThroughXray: SOCKS bridge (mita egress.proxies SOCKS5 → loopback SOCKS).
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.Mieru || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		injectMieruEgress(xrayConfig, inbound)
	}
	// TrustTunnel routeThroughXray: SOCKS bridge ([forward_protocol.socks5]).
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.TrustTunnel || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		injectTrustTunnelEgress(xrayConfig, inbound)
	}
	// NaiveProxy: prefer inbound rows (mtproto pattern — tag = inbound.Tag).
	// Fall back to legacy global lucxTunnel_naive settings until migrated.
	naiveInboundSeen := false
	for i := range inbounds {
		inbound := inbounds[i]
		if inbound.Protocol != model.Naive || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		naiveInboundSeen = true
		injectNaiveInboundEgress(xrayConfig, inbound)
	}
	if !naiveInboundSeen {
		if naiveCfg, err := (&TunnelService{}).LoadNaiveConfig(); err == nil {
			injectTunnelEgress(xrayConfig, naiveCfg)
		} else {
			logger.Warning("tunnel egress: load naive config failed:", err)
		}
	}
	// END LUCX-HOOK

	// Every AmneziaWG inbound is embedded (internal/amneziawgnet: amneziawg-go
	// over a gVisor netstack, no kernel module) and relays every peer's
	// decapsulated traffic into its own loopback SOCKS5 inbound, always on —
	// unlike mtproto's bridge above, there's no opt-in gate here: once
	// traffic is decapsulated in gVisor, Xray's own freedom outbound is the
	// only way it reaches the real internet at all, not an optional extra
	// hop. Whether it goes anywhere beyond Xray's default routing is up to
	// whatever rules the admin adds through the stock Routing page, exactly
	// like routing any other protocol.
	injectAmneziawgnetSocks(xrayConfig, inbounds)

	// Restores each opted-in peer's own distinct public IPv6 source identity
	// for its outbound connections — a peer that has an IPv6 address in its
	// AllowedIPs, on an inbound with IPv6Enabled, gets its own freedom
	// outbound bound to that exact address via sendThrough.
	// internal/amneziawgnet's own Manager is responsible for actually
	// aliasing that address onto the host (see v6alias.go) so the kernel
	// lets Xray bind an egress socket to it at all; this call only builds
	// the Xray-side outbound/routing-rule half.
	injectAmneziawgV6Egress(xrayConfig, inbounds)

	// Wire the panel's own HTTP traffic through the configured outbound, after
	// the subscription merge so subscription outbound tags are valid targets.
	if egressTag, err := s.settingService.GetPanelOutbound(); err != nil {
		logger.Warning("read panelOutbound setting failed:", err)
	} else if egressTag != "" {
		injectPanelEgress(xrayConfig, egressTag)
	}

	nodes, err := s.nodeService.GetAll()
	if err != nil {
		logger.Warning("read nodes for egress injection failed:", err)
	} else {
		injectNodeEgresses(xrayConfig, nodes)
	}

	return xrayConfig, nil
}

// PanelEgressInboundTag is the tag of the loopback SOCKS inbound injected into
// the generated config when a panel outbound is configured. The panel's own
// HTTP clients dial through it to egress via the chosen outbound.
const PanelEgressInboundTag = "panel-egress"

// panelEgressBasePort is the first port tried for the egress bridge; ports
// already taken by other inbounds in the generated config are skipped.
const panelEgressBasePort = 62790

// injectPanelEgress appends a loopback SOCKS inbound and routing rule only when
// outboundTag resolves in the final outbound or balancer set. Otherwise the
// entire injection is skipped. Generated state is hot-appliable and never
// modifies the stored template or restarts the core.
func injectPanelEgress(cfg *xray.Config, outboundTag string) {
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == PanelEgressInboundTag {
			logger.Warning("panel egress: inbound tag [", PanelEgressInboundTag, "] already exists, skipping injection")
			return
		}
	}

	// The rule must exist before the inbound takes traffic, otherwise the
	// bridge would silently egress through the default outbound instead.
	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			logger.Warning("panel egress: routing section is unparsable, skipping injection:", err)
			return
		}
	}
	if !routingTargetExists(routing, cfg.OutboundConfigs, outboundTag) {
		logger.Warning("panel egress: target tag [", outboundTag, "] not found, skipping injection")
		return
	}
	rules, _ := routing["rules"].([]any)
	rule := map[string]any{
		"type":       "field",
		"inboundTag": []any{PanelEgressInboundTag},
	}
	// The configured tag may name a routing balancer instead of a concrete
	// outbound. A field rule can target either, so emit the matching key —
	// balancerTag load-balances the panel's own traffic across the balancer's
	// outbounds, while a plain outbound tag keeps the original behavior.
	if routingTagIsBalancer(routing, outboundTag) {
		rule["balancerTag"] = outboundTag
	} else {
		rule["outboundTag"] = outboundTag
	}
	routing["rules"] = append([]any{rule}, rules...)
	newRouting, err := json.Marshal(routing)
	if err != nil {
		logger.Warning("panel egress: failed to rebuild routing section, skipping injection:", err)
		return
	}
	cfg.RouterConfig = json_util.RawMessage(newRouting)

	used := make(map[int]struct{}, len(cfg.InboundConfigs))
	for i := range cfg.InboundConfigs {
		used[cfg.InboundConfigs[i].Port] = struct{}{}
	}
	port := panelEgressBasePort
	for {
		if _, taken := used[port]; !taken {
			break
		}
		port++
	}

	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     port,
		Protocol: "socks",
		Settings: json_util.RawMessage(`{"auth":"noauth","udp":false}`),
		Tag:      PanelEgressInboundTag,
	})
}

func outboundTagExists(outbounds json_util.RawMessage, tag string) bool {
	var parsed []struct {
		Tag string `json:"tag"`
	}
	if tag == "" || json.Unmarshal(outbounds, &parsed) != nil {
		return false
	}
	for _, outbound := range parsed {
		if outbound.Tag == tag {
			return true
		}
	}
	return false
}

func routingTargetExists(routing map[string]any, outbounds json_util.RawMessage, tag string) bool {
	return routingTagIsBalancer(routing, tag) || outboundTagExists(outbounds, tag)
}

// NodeEgressInboundTag returns the loopback SOCKS inbound tag for a given node.
func NodeEgressInboundTag(nodeID int) string {
	return fmt.Sprintf("node-egress-%d", nodeID)
}

// nodeEgressBasePort is the first port tried for node egress bridges.
const nodeEgressBasePort = 62800

// injectNodeEgresses appends a loopback SOCKS inbound per enabled node that has
// an OutboundTag, and prepends a routing rule sending that inbound's traffic to
// the selected outbound tag. These bridges are hot-appliable.
func injectNodeEgresses(cfg *xray.Config, nodes []*model.Node) {
	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			logger.Warning("node egress: routing section is unparsable, skipping injection:", err)
			return
		}
	}

	used := make(map[int]struct{}, len(cfg.InboundConfigs))
	usedTags := make(map[string]struct{}, len(cfg.InboundConfigs))
	for i := range cfg.InboundConfigs {
		used[cfg.InboundConfigs[i].Port] = struct{}{}
		usedTags[cfg.InboundConfigs[i].Tag] = struct{}{}
	}

	rules, _ := routing["rules"].([]any)
	newRules := make([]any, 0)

	for _, n := range nodes {
		if !n.Enable || n.OutboundTag == "" {
			continue
		}
		if !routingTargetExists(routing, cfg.OutboundConfigs, n.OutboundTag) {
			logger.Warning("node egress: target tag [", n.OutboundTag, "] not found, skipping node [", n.Id, "]")
			continue
		}
		tag := NodeEgressInboundTag(n.Id)
		if _, exists := usedTags[tag]; exists {
			logger.Warning("node egress: inbound tag [", tag, "] already exists, skipping")
			continue
		}
		usedTags[tag] = struct{}{}

		rule := map[string]any{
			"type":       "field",
			"inboundTag": []any{tag},
		}
		if routingTagIsBalancer(routing, n.OutboundTag) {
			rule["balancerTag"] = n.OutboundTag
		} else {
			rule["outboundTag"] = n.OutboundTag
		}
		newRules = append(newRules, rule)

		port := nodeEgressBasePort + n.Id
		for {
			if _, taken := used[port]; !taken {
				break
			}
			port++
		}
		used[port] = struct{}{}

		cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
			Listen:   json_util.RawMessage(`"127.0.0.1"`),
			Port:     port,
			Protocol: "socks",
			Settings: json_util.RawMessage(`{"auth":"noauth","udp":false}`),
			Tag:      tag,
		})
	}

	if len(newRules) == 0 {
		return
	}
	routing["rules"] = append(newRules, rules...)
	newRouting, err := json.Marshal(routing)
	if err != nil {
		logger.Warning("node egress: failed to rebuild routing section, skipping injection:", err)
		return
	}
	cfg.RouterConfig = json_util.RawMessage(newRouting)
}

// routingTagIsBalancer reports whether tag names a balancer in the parsed
// routing section. The panel-egress rule targets a balancer via balancerTag and
// a concrete outbound via outboundTag, so the caller picks the key from this.
func routingTagIsBalancer(routing map[string]any, tag string) bool {
	if tag == "" {
		return false
	}
	balancers, ok := routing["balancers"].([]any)
	if !ok {
		return false
	}
	for _, b := range balancers {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := bm["tag"].(string); ok && t == tag {
			return true
		}
	}
	return false
}

// mtprotoEgressSocksSettings is the loopback SOCKS server a routed mtproto
// inbound exposes for its mtg sidecar to dial Telegram through. mtg makes plain
// TCP connections, so UDP is left off (matching the panel egress bridge).
// Shared with injectTunnelEgress (NaiveProxy also dials plain TCP via SOCKS5).
const mtprotoEgressSocksSettings = `{"auth":"noauth","udp":false}`

// LUCX-HOOK: injectNaiveInboundEgress wires one routed Naive inbound into Xray as a
// loopback SOCKS bridge tagged with the inbound's own tag (mtproto pattern).
func injectNaiveInboundEgress(cfg *xray.Config, inbound *model.Inbound) {
	var parsed struct {
		RouteThroughXray bool   `json:"routeThroughXray"`
		RouteXrayPort    int    `json:"routeXrayPort"`
		OutboundTag      string `json:"outboundTag"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return
	}
	if !parsed.RouteThroughXray || parsed.RouteXrayPort <= 0 || inbound.Tag == "" {
		return
	}
	injectSocksEgress(cfg, inbound.Tag, parsed.RouteXrayPort, parsed.OutboundTag, "naive egress", true)
}

// END LUCX-HOOK

// LUCX-HOOK: injectTunnelEgress wires the legacy global NaiveProxy tunnel
// core (settings lucxTunnel_naive) into Xray. Kept for pre-migration hosts.
func injectTunnelEgress(cfg *xray.Config, naive tunnel.NaiveConfig) {
	if !naive.Enabled || !naive.RouteThroughXray || naive.RouteXrayPort <= 0 {
		return
	}
	injectSocksEgress(cfg, tunnel.NaiveEgressTag, naive.RouteXrayPort, naive.OutboundTag, "tunnel egress", true)
}

func injectSocksEgress(cfg *xray.Config, tag string, port int, outboundTag, logPrefix string, auth bool) {
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == tag {
			logger.Warning(logPrefix, ": inbound tag [", tag, "] already present in generated config, skipping bridge")
			return
		}
	}
	if outboundTag != "" {
		routing := map[string]any{}
		parseOK := true
		if len(cfg.RouterConfig) > 0 {
			if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
				logger.Warning(logPrefix, ": routing section is unparsable, skipping rule:", err)
				parseOK = false
			}
		}
		if parseOK && !routingTargetExists(routing, cfg.OutboundConfigs, outboundTag) {
			logger.Warning(logPrefix, ": target tag [", outboundTag, "] not found, injecting SOCKS without force-route")
			parseOK = false
		}
		if parseOK {
			rules, _ := routing["rules"].([]any)
			rule := map[string]any{
				"type":       "field",
				"inboundTag": []any{tag},
			}
			if routingTagIsBalancer(routing, outboundTag) {
				rule["balancerTag"] = outboundTag
			} else {
				rule["outboundTag"] = outboundTag
			}
			routing["rules"] = append([]any{rule}, rules...)
			newRouting, err := json.Marshal(routing)
			if err != nil {
				logger.Warning(logPrefix, ": failed to rebuild routing section, skipping rule:", err)
			} else {
				cfg.RouterConfig = json_util.RawMessage(newRouting)
			}
		}
	}
	settings := json_util.RawMessage(mtprotoEgressSocksSettings)
	if auth {
		user, pass := tunnel.SocksBridgeAuth()
		if raw, err := json.Marshal(map[string]any{
			"auth": "password",
			"udp":  false,
			"accounts": []map[string]string{
				{"user": user, "pass": pass},
			},
		}); err == nil {
			settings = json_util.RawMessage(raw)
		}
	}
	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     port,
		Protocol: "socks",
		Settings: settings,
		Sniffing: json_util.RawMessage(awgEgressTunSniffing),
		Tag:      tag,
	})
}

// END LUCX-HOOK

// injectMtprotoEgress wires one routed mtproto inbound into the generated
// config after any selected outbound resolves in the final target set. Invalid
// selected targets or routing data skip the entire injection; without a selected
// outbound, the bridge retains default-route behavior. Generated state remains
// hot-appliable, leaves the stored template untouched, and never forces a full
// Xray restart. Mirrors injectPanelEgress.
func injectMtprotoEgress(cfg *xray.Config, inbound *model.Inbound) {
	var parsed struct {
		RouteThroughXray bool   `json:"routeThroughXray"`
		RouteXrayPort    int    `json:"routeXrayPort"`
		OutboundTag      string `json:"outboundTag"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return
	}
	if !parsed.RouteThroughXray || parsed.RouteXrayPort <= 0 || inbound.Tag == "" {
		return
	}
	tag := inbound.Tag
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == tag {
			logger.Warning("mtproto egress: inbound tag [", tag, "] already present in generated config, skipping bridge")
			return
		}
	}

	if parsed.OutboundTag != "" {
		routing := map[string]any{}
		parseOK := true
		if len(cfg.RouterConfig) > 0 {
			if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
				logger.Warning("mtproto egress: routing section is unparsable, skipping rule:", err)
				parseOK = false
			}
		}
		if parseOK && !routingTargetExists(routing, cfg.OutboundConfigs, parsed.OutboundTag) {
			logger.Warning("mtproto egress: target tag [", parsed.OutboundTag, "] not found, injecting SOCKS without force-route")
			parseOK = false
		}
		if parseOK {
			rules, _ := routing["rules"].([]any)
			rule := map[string]any{
				"type":       "field",
				"inboundTag": []any{tag},
			}
			if routingTagIsBalancer(routing, parsed.OutboundTag) {
				rule["balancerTag"] = parsed.OutboundTag
			} else {
				rule["outboundTag"] = parsed.OutboundTag
			}
			routing["rules"] = append([]any{rule}, rules...)
			newRouting, err := json.Marshal(routing)
			if err != nil {
				logger.Warning("mtproto egress: failed to rebuild routing section, skipping rule:", err)
			} else {
				cfg.RouterConfig = json_util.RawMessage(newRouting)
			}
		}
	}

	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     parsed.RouteXrayPort,
		Protocol: "socks",
		Settings: json_util.RawMessage(mtprotoEgressSocksSettings),
		Tag:      tag,
	})
}

// LUCX-HOOK: awgEgressTunSettings is the TUN inbound an AWG kernel interface
// flows its decrypted IP packets into so Xray routing rules apply. Unlike the
// mtproto SOCKS bridge (TCP from userspace), AWG produces raw IP packets from
// the kernel module, so a TUN inbound is the correct sink. The TUN device name
// is derived from the AWG interface name (awgN → tunN); the gateway comes from
// awgTunGateway, not from the inbound's tunnel address.
const awgEgressTunSettingsFmt = `{"name":"%s","mtu":%d,"gateway":["%s"],"userLevel":0}`

// awgTunGateway returns the address Xray assigns to tunN. It must lie outside
// the AWG tunnel subnet — sharing it would give the kernel two connected
// routes for the same prefix and pull the reply path toward tunN — and must
// differ between routed inbounds, whose TUN devices coexist. A per-id /30
// under 10.254/16 satisfies both against the 10.200.0.0/24-style defaults the
// panel hands out.
func awgTunGateway(id int) string {
	if id >= 1 && id < 254 {
		return fmt.Sprintf("10.254.%d.1/30", id)
	}
	return fmt.Sprintf("10.252.%d.1/30", (id%253)+1)
}

// injectAwgEgress wires one routed AWG inbound into the generated config: it
// appends a TUN inbound (tagged with the inbound's own tag) and, when an
// outbound is selected, prepends a routing rule sending that tag to it. Both
// live only in the generated config — the stored template is untouched — so
// every change to a routed AWG inbound must force a config regen (see
// awgRoutesThroughXray in §inbound.go); the kernel-side sidecar push alone
// never reaches Xray. Mirrors injectMtprotoEgress but uses a TUN inbound
// instead of a loopback SOCKS bridge, because AWG traffic is raw IP from a
// kernel module, not TCP from a userspace daemon.
func injectAwgEgress(cfg *xray.Config, inbound *model.Inbound) {
	var parsed struct {
		RouteThroughXray bool   `json:"routeThroughXray"`
		OutboundTag      string `json:"outboundTag"`
		MTU              int    `json:"mtu"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return
	}
	if !parsed.RouteThroughXray || inbound.Tag == "" {
		return
	}
	tag := inbound.Tag
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == tag {
			logger.Warning("awg egress: inbound tag [", tag, "] already present in generated config, skipping bridge")
			return
		}
	}

	// When outboundTag is set, prepend a routing rule that sends all traffic
	// from this AWG TUN inbound to that specific outbound (e.g. "warp",
	// "vless-out", a balancer). When outboundTag is empty, no rule is added —
	// TUN traffic falls through to Xray's general routing pipeline, so
	// sniffing/domain/IP/balancer rules apply normally (the "Xray kettle").
	// This mirrors mtproto: an empty outboundTag means "let Xray decide".
	if parsed.OutboundTag != "" {
		routing := map[string]any{}
		parseOK := true
		if len(cfg.RouterConfig) > 0 {
			if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
				logger.Warning("awg egress: routing section is unparsable, skipping rule:", err)
				parseOK = false
			}
		}
		if parseOK && !routingTargetExists(routing, cfg.OutboundConfigs, parsed.OutboundTag) {
			logger.Warning("awg egress: target tag [", parsed.OutboundTag, "] not found, injecting TUN without force-route")
			parseOK = false
		}
		if parseOK {
			rules, _ := routing["rules"].([]any)
			rule := map[string]any{
				"type":       "field",
				"inboundTag": []any{tag},
			}
			if routingTagIsBalancer(routing, parsed.OutboundTag) {
				rule["balancerTag"] = parsed.OutboundTag
			} else {
				rule["outboundTag"] = parsed.OutboundTag
			}
			routing["rules"] = append([]any{rule}, rules...)
			if newRouting, err := json.Marshal(routing); err == nil {
				cfg.RouterConfig = json_util.RawMessage(newRouting)
			} else {
				logger.Warning("awg egress: failed to rebuild routing section, skipping rule:", err)
			}
		}
	}

	mtu := parsed.MTU
	if mtu == 0 {
		mtu = 1320
	}
	tunName := fmt.Sprintf("tun%d", inbound.Id)
	tunSettings := fmt.Sprintf(awgEgressTunSettingsFmt, tunName, mtu, awgTunGateway(inbound.Id))
	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Protocol: "tun",
		Settings: json_util.RawMessage(tunSettings),
		Sniffing: json_util.RawMessage(awgEgressTunSniffing),
		Tag:      tag,
	})
}

// awgEgressTunSniffing lets the router match domain rules against AWG flows:
// without it the TUN inbound only ever exposes destination IPs, so geosite/
// domain rules silently never fire for routed AWG traffic. routeOnly keeps
// the sniffed SNI/Host a routing-time hint — the dial target stays the IP the
// client resolved, so egress behavior is unchanged.
const awgEgressTunSniffing = `{"enabled":true,"destOverride":["http","tls","quic"],"routeOnly":true}`

// injectOlcrtcEgress wires a routed olcRTC inbound as a loopback SOCKS bridge
// (mirrors injectNaiveInboundEgress). The olcrtc YAML socks: block dials it.
func injectOlcrtcEgress(cfg *xray.Config, inbound *model.Inbound) {
	cfgO, ok := tunnel.OlcrtcConfigFromInbound(inbound)
	if !ok || !cfgO.RouteThroughXray || cfgO.RouteXrayPort <= 0 || inbound.Tag == "" {
		return
	}
	injectSocksEgress(cfg, inbound.Tag, cfgO.RouteXrayPort, cfgO.OutboundTag, "olcrtc egress", false)
}

// injectMieruEgress wires a routed mieru inbound as a loopback SOCKS bridge.
// mita's native egress.proxies SOCKS5 block dials it. mita resolves domains
// itself (SOCKS CONNECT is an IP); sniffing on the bridge extracts SNI so
// geosite/domain rules still match (same as AWG TUN).
func injectMieruEgress(cfg *xray.Config, inbound *model.Inbound) {
	cfgM, ok := tunnel.MieruConfigFromInbound(inbound)
	if !ok || !cfgM.RouteThroughXray || cfgM.RouteXrayPort <= 0 || inbound.Tag == "" {
		return
	}
	injectSocksEgress(cfg, inbound.Tag, cfgM.RouteXrayPort, cfgM.OutboundTag, "mieru egress", false)
}

// injectTrustTunnelEgress wires a routed TrustTunnel inbound as a loopback
// SOCKS bridge. The endpoint's [forward_protocol.socks5] dials it. Like
// mieru, the endpoint resolves destinations itself; sniffing on the bridge
// extracts SNI so geosite/domain rules still match.
func injectTrustTunnelEgress(cfg *xray.Config, inbound *model.Inbound) {
	cfgT, ok := tunnel.TrustTunnelConfigFromInbound(inbound)
	if !ok || !cfgT.RouteThroughXray || cfgT.RouteXrayPort <= 0 || inbound.Tag == "" {
		return
	}
	injectSocksEgress(cfg, inbound.Tag, cfgT.RouteXrayPort, cfgT.OutboundTag, "trusttunnel egress", false)
}

// injectQwdttEgress wires a routed qWDTT inbound into Xray as a TUN bridge
// (mirrors injectAwgEgress). Kernel ifaces wdtt0/wdttraw0 are policy-routed
// into this TUN by tunnel.ensureQwdttXrayRouting.
func injectQwdttEgress(cfg *xray.Config, inbound *model.Inbound) {
	cfgQ, ok := tunnel.QwdttConfigFromInbound(inbound)
	if !ok || !cfgQ.RouteThroughXray || inbound.Tag == "" {
		return
	}
	tag := inbound.Tag
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == tag {
			logger.Warning("qwdtt egress: inbound tag [", tag, "] already present, skipping bridge")
			return
		}
	}
	if cfgQ.OutboundTag != "" {
		routing := map[string]any{}
		parseOK := true
		if len(cfg.RouterConfig) > 0 {
			if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
				logger.Warning("qwdtt egress: routing unparsable, skipping rule:", err)
				parseOK = false
			}
		}
		if parseOK && !routingTargetExists(routing, cfg.OutboundConfigs, cfgQ.OutboundTag) {
			logger.Warning("qwdtt egress: target tag [", cfgQ.OutboundTag, "] not found, injecting TUN without force-route")
			parseOK = false
		}
		if parseOK {
			rules, _ := routing["rules"].([]any)
			rule := map[string]any{
				"type":       "field",
				"inboundTag": []any{tag},
			}
			if routingTagIsBalancer(routing, cfgQ.OutboundTag) {
				rule["balancerTag"] = cfgQ.OutboundTag
			} else {
				rule["outboundTag"] = cfgQ.OutboundTag
			}
			routing["rules"] = append([]any{rule}, rules...)
			if newRouting, err := json.Marshal(routing); err == nil {
				cfg.RouterConfig = json_util.RawMessage(newRouting)
			}
		}
	}
	tunName := tunnel.QwdttTunName(inbound.Id)
	mtu := 1280
	tunSettings := fmt.Sprintf(awgEgressTunSettingsFmt, tunName, mtu, tunnel.QwdttTunGateway(inbound.Id))
	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Protocol: "tun",
		Settings: json_util.RawMessage(tunSettings),
		Sniffing: json_util.RawMessage(awgEgressTunSniffing),
		Tag:      tag,
	})
}

// LUCX-HOOK: AWG outbound — inject one freedom outbound per enabled
// awg_outbounds row so routing rules can send traffic through the upstream
// VPN. Each outbound is bound to its kernel interface via sockopt.interface
// (always awgo-{Id}, never the editable Tag) and optionally source-bound via
// sendThrough (tunnel IP with the CIDR mask stripped — Xray rejects masked
// IPs). Called after the regular outbounds are merged and before SOCKS/TUN
// egress injectors look up outboundTag.
//
// OutboundConfigs is a json_util.RawMessage (a JSON array blob), not a typed
// slice, so the existing array is unmarshaled into []any, appended to, and
// re-marshaled — mirroring mergeSubscriptionOutbounds. A corrupt template is
// left untouched (the user will see the error on Xray start / next save),
// never silently dropped.
// awgOutboundIsUp answers the one question in config generation that can change
// between builds. Recorded state, never a live probe: a shell call here would
// make the generated config — and so the decision to restart Xray — depend on
// the probe's mood. A var so a test can drive it without a kernel module.
var awgOutboundIsUp = func(ifname string) bool {
	if !awg.KernelAvailable() {
		return true
	}
	return awg.GetManager().ClientIfaceUp(ifname)
}

func injectAwgOutbounds(cfg *xray.Config, outbounds []*model.AwgOutbound) {
	for _, o := range outbounds {
		if !o.Enable {
			// Keep the tag so routing/balancer rules still match. A missing
			// tag makes Xray fall through to the default outbound (direct) —
			// YouTube-via-AWG kept working after the operator disabled the
			// row (VladufQa). Blackhole is fail-closed.
			if err := appendAwgOutbound(cfg, blackholeOutbound(o.Tag)); err != nil {
				logger.Warning("awg outbound: failed to inject blackhole for disabled tag", o.Tag, ":", err)
			}
			continue
		}
		ci, ok := awg.ClientInstanceFromOutbound(o)
		if !ok {
			continue
		}
		if !awgOutboundIsUp(ci.Ifname) {
			if err := appendAwgOutbound(cfg, blackholeOutbound(o.Tag)); err != nil {
				logger.Warning("awg outbound: failed to inject blackhole for down iface", ci.Ifname, ":", err)
			}
			continue
		}
		settings := map[string]any{
			"domainStrategy": "UseIP",
		}
		streamSettings := map[string]any{
			"sockopt": map[string]any{
				"interface": ci.Ifname,
			},
		}
		outbound := map[string]any{
			"protocol":       "freedom",
			"tag":            o.Tag,
			"settings":       settings,
			"streamSettings": streamSettings,
		}
		// Xray reads sendThrough from the outbound, not from a protocol's
		// settings, and drops the unknown key there without a word.
		if ip := strings.SplitN(ci.Settings.Address, "/", 2)[0]; ip != "" {
			outbound["sendThrough"] = ip
		}
		if err := appendAwgOutbound(cfg, outbound); err != nil {
			logger.Warning("awg outbound: failed to inject freedom outbound for tag", o.Tag, ":", err)
		}
	}
}

func injectSidecarOutbounds(cfg *xray.Config, outbounds []*model.SidecarOutbound) {
	for _, o := range outbounds {
		if o == nil {
			continue
		}
		if !o.Enable {
			if err := appendAwgOutbound(cfg, blackholeOutbound(o.Tag)); err != nil {
				logger.Warning("sidecar outbound: failed to inject blackhole for disabled tag", o.Tag, ":", err)
			}
			continue
		}
		s, ok := tunnel.ParseSidecarSettings(o)
		if !ok {
			continue
		}
		if err := appendAwgOutbound(cfg, map[string]any{
			"protocol": "socks",
			"tag":      o.Tag,
			"settings": map[string]any{
				"servers": []any{
					map[string]any{
						"address": "127.0.0.1",
						"port":    s.SocksPort,
					},
				},
			},
		}); err != nil {
			logger.Warning("sidecar outbound: failed to inject socks outbound for tag", o.Tag, ":", err)
		}
	}
}

// appendAwgOutbound unmarshals cfg.OutboundConfigs (a JSON array blob) into
// []any, appends one outbound object, and re-marshals it back. A nil/empty
// OutboundConfigs becomes a single-element array. A corrupt template aborts
// the append and leaves the field untouched, mirroring the safety contract of
// mergeSubscriptionOutbounds.
func blackholeOutbound(tag string) map[string]any {
	return map[string]any{
		"protocol": "blackhole",
		"tag":      tag,
		"settings": map[string]any{},
	}
}

func appendAwgOutbound(cfg *xray.Config, ob map[string]any) error {
	var existing []any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &existing); err != nil {
			// Corrupt template outbounds — do not touch the field at all.
			return err
		}
	}
	existing = append(existing, ob)
	combined, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	cfg.OutboundConfigs = json_util.RawMessage(combined)
	return nil
}

// END LUCX-HOOK

// amneziawgEgressSniffingSettings matches this fork's normal per-inbound
// default (see default.json's "mixed" inbound). Without this, domain-based
// Routing rules can never match this relay: the peer resolved DNS
// itself, through the tunnel, before ever sending a packet — by the time the
// embedded forwarder recovers the decapsulated traffic, the destination is
// already a bare IP, with no domain name attached at the network layer at
// all. Sniffing recovers it from the payload itself (TLS SNI / HTTP Host /
// QUIC) the same way it already does for every other inbound; without it,
// only tag/IP/network-based rules can ever match this traffic, and any
// domain rule above it in the list is silently unreachable.
const amneziawgEgressSniffingSettings = `{"enabled":true,"destOverride":["http","tls","quic","fakedns"]}`

// injectAmneziawgnetSocks gives every enabled AmneziaWG inbound with at
// least one qualifying peer its own loopback SOCKS5 inbound for the
// embedded (amneziawg-go) relay path (internal/amneziawgnet) -- always on,
// since there is no alternative datapath once traffic is decapsulated in
// gVisor: Xray's own freedom outbound is how it reaches the real internet at
// all (see internal/amneziawgnet/relay.go's doc comment, Finding 3 of the
// migration plan). Tagged with the inbound's own real tag: it's already
// selectable in the panel's stock Routing page (InboundService.GetInboundTags
// is protocol-blind), and per-inbound traffic totals
// (internal/web/service/inbound_traffic.go's addClientTraffic) match by
// exact tag -- reusing it isn't a style choice.
func injectAmneziawgnetSocks(cfg *xray.Config, inbounds []*model.Inbound) {
	existingTags := make(map[string]struct{}, len(cfg.InboundConfigs))
	for i := range cfg.InboundConfigs {
		existingTags[cfg.InboundConfigs[i].Tag] = struct{}{}
	}

	for _, inbound := range inbounds {
		if !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		var inst amneziawg.Instance
		var ok bool
		switch inbound.Protocol {
		case model.AmneziaWG:
			inst, ok = amneziawg.InstanceFromInbound(inbound)
		// LUCX-HOOK: sans-kernel AWG inbound uses the same gVisor SOCKS path
		case model.AWG:
			if awg.KernelAvailable() {
				continue
			}
			src, srcOK := awg.InstanceFromInbound(inbound)
			if !srcOK {
				continue
			}
			inst, ok = kernelAwgToEmbedded(src)
		// END LUCX-HOOK
		default:
			continue
		}
		if !ok {
			continue
		}
		if _, taken := existingTags[inbound.Tag]; taken {
			logger.Warning("amneziawgnet socks: inbound tag [", inbound.Tag, "] already present in generated config, skipping its relay inbound")
			continue
		}

		emails := make([]string, 0, len(inst.Peers))
		for _, p := range inst.Peers {
			if p.Email != "" {
				emails = append(emails, p.Email)
			}
		}
		if len(emails) == 0 {
			continue
		}

		settings, err := amneziawgnet.SocksInboundSettings(emails, amneziawgnet.SocksPassword())
		if err != nil {
			logger.Warning("amneziawgnet socks: building settings for inbound [", inbound.Tag, "]: ", err)
			continue
		}

		existingTags[inbound.Tag] = struct{}{}
		cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
			Listen:   json_util.RawMessage(`"127.0.0.1"`),
			Port:     amneziawgnet.SOCKSPortForInbound(inbound.Id),
			Protocol: "socks",
			Settings: json_util.RawMessage(settings),
			Sniffing: json_util.RawMessage(amneziawgEgressSniffingSettings),
			Tag:      inbound.Tag,
		})
	}
}

// amneziawgV6EgressTag returns the stable, globally-unique freedom outbound
// tag for one peer's IPv6 source-identity egress. Stable across config
// regenerations (a pure function of two stable identifiers), so
// internal/xray/hot_diff.go's tag-keyed outbound/routing diffing recognizes
// "unchanged" rather than remove+recreate on every poll. The inbound.Id
// prefix is defense in depth, not load-bearing on its own: email is already
// enforced globally unique across the whole panel's client table
// (model.ClientRecord.Email has a gorm uniqueIndex) — kept anyway since it
// costs nothing and makes the tag self-describing, matching
// NodeEgressInboundTag's own style.
func amneziawgV6EgressTag(inboundID int, email string) string {
	return fmt.Sprintf("amneziawg-v6-%d-%s", inboundID, email)
}

// injectAmneziawgV6Egress gives every enabled, non-node-hosted AmneziaWG
// peer with an IPv6 AllowedIPs entry its own single-purpose freedom
// outbound, bound via sendThrough to that exact address, plus a routing
// rule sending only that peer's own traffic through it — restoring the
// per-client public IPv6 identity the hard cutover temporarily dropped
// (Phase 3.5 of the migration plan). Scoped to outbound source identity
// only: it depends on internal/amneziawgnet's own alias mechanism actually
// giving the host that address at the OS level (see v6alias.go's
// V6AliasesActive, the exact same gate this function uses below) — without
// that, sendThrough fails to bind and every connection through it errors
// outright (freedom.go's dial failure); there is no fallback outbound.
//
// The routing rule matches both inboundTag and user: SocksInboundSettings
// (used by injectAmneziawgnetSocks above) already authenticates each
// connection as the peer's own email via stock SOCKS5 auth, and a stock
// Xray SOCKS5 inbound sets that connection's stats/routing identity from
// the authenticated username — so "user" reliably isolates exactly one
// peer's traffic, the same building block Finding 3 of the migration plan
// already established for per-client stats.
//
// Modeled on injectNodeEgresses (the established N-per-slice inbound+rule
// precedent, not injectAmneziawgnetSocks itself, which only ever emits a
// single inbound and never touches outbounds/routing) and
// mergeSubscriptionOutbounds's unmarshal-append-remarshal pattern for
// cfg.OutboundConfigs. Synthetic rules are prepended ahead of whatever's
// already in the routing rules array, the same pattern injectNodeEgresses/
// injectMtprotoEgress already use for their own always-must-win infra
// rules — this never touches the admin's own saved Routing-page rule
// order.
func injectAmneziawgV6Egress(cfg *xray.Config, inbounds []*model.Inbound) {
	// Protocol is checked alongside Tag, not just Tag alone: a tag collision
	// with some unrelated (non-socks) inbound must not be mistaken for this
	// instance's own relay having been created.
	liveInboundTags := make(map[string]struct{}, len(cfg.InboundConfigs))
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Protocol == "socks" {
			liveInboundTags[cfg.InboundConfigs[i].Tag] = struct{}{}
		}
	}

	var existingOutbounds []any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &existingOutbounds); err != nil {
			logger.Warning("amneziawg v6 egress: outbounds section is unparsable, skipping injection:", err)
			return
		}
	}
	usedOutboundTags := make(map[string]struct{}, len(existingOutbounds))
	for _, o := range existingOutbounds {
		if m, ok := o.(map[string]any); ok {
			if t, ok := m["tag"].(string); ok {
				usedOutboundTags[t] = struct{}{}
			}
		}
	}

	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			logger.Warning("amneziawg v6 egress: routing section is unparsable, skipping injection:", err)
			return
		}
	}
	rules, _ := routing["rules"].([]any)
	newRules := make([]any, 0)
	newOutbounds := make([]any, 0)

	for _, inbound := range inbounds {
		if inbound.Protocol != model.AmneziaWG || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		if _, live := liveInboundTags[inbound.Tag]; !live {
			// The relay inbound itself wasn't created this pass (e.g. a tag
			// collision inside injectAmneziawgnetSocks) -- no SOCKS5 inbound
			// exists for hot_diff.go's inboundTag match to ever fire against.
			continue
		}
		inst, ok := amneziawg.InstanceFromInbound(inbound)
		if !ok || !amneziawgnet.V6AliasesActive(inst) {
			continue
		}
		for _, p := range inst.Peers {
			if p.Email == "" {
				continue
			}
			v6 := amneziawg.FirstIPv6(p.AllowedIPs)
			if v6 == "" {
				continue
			}
			tag := amneziawgV6EgressTag(inbound.Id, p.Email)
			if _, taken := usedOutboundTags[tag]; taken {
				logger.Warning("amneziawg v6 egress: outbound tag [", tag, "] already exists, skipping peer [", p.Email, "]")
				continue
			}
			usedOutboundTags[tag] = struct{}{}
			newOutbounds = append(newOutbounds, map[string]any{
				"tag":         tag,
				"protocol":    "freedom",
				"sendThrough": v6,
				"settings":    map[string]any{},
			})
			newRules = append(newRules, map[string]any{
				"type":        "field",
				"inboundTag":  []any{inbound.Tag},
				"user":        []any{p.Email},
				"outboundTag": tag,
			})
		}
	}

	if len(newOutbounds) == 0 {
		return
	}

	merged := make([]any, 0, len(existingOutbounds))
	merged = append(merged, existingOutbounds...)
	merged = append(merged, newOutbounds...)
	combined, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		logger.Warning("amneziawg v6 egress: failed to rebuild outbounds section, skipping injection:", err)
		return
	}
	cfg.OutboundConfigs = json_util.RawMessage(combined)

	routing["rules"] = append(newRules, rules...)
	newRouting, err := json.Marshal(routing)
	if err != nil {
		logger.Warning("amneziawg v6 egress: failed to rebuild routing section, skipping injection:", err)
		return
	}
	cfg.RouterConfig = json_util.RawMessage(newRouting)
}

// mergeSubscriptionOutbounds appends the subscription outbounds to the
// OutboundConfigs array of the xray config. It works on the already-unmarshaled
// template so that manually configured outbounds are never overwritten.
//
// Safety: if we cannot parse the template's outbounds array, we leave
// OutboundConfigs exactly as it came from the template (we do not inject
// subscription outbounds). This prevents us from accidentally dropping the
// user's manually configured outbounds when the template is in a weird state.
func mergeSubscriptionOutbounds(cfg *xray.Config, prepend, appendList []any) {
	if len(prepend) == 0 && len(appendList) == 0 {
		return
	}
	var templateOutbounds []any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &templateOutbounds); err != nil {
			// Corrupt template outbounds — do not touch the field at all.
			// The user will see problems on Xray start / next save.
			return
		}
	}
	var merged []any
	merged = append(merged, prepend...)
	merged = append(merged, templateOutbounds...)
	merged = append(merged, appendList...)
	combined, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return
	}
	cfg.OutboundConfigs = json_util.RawMessage(combined)
}

// ensureAPIServices guarantees the gRPC services the panel depends on are
// listed in the generated config's api block: HandlerService and StatsService
// have always been required for inbound/user management and traffic polling,
// and RoutingService enables hot routing reload on templates saved before it
// was added to the default template. The stored template itself is not
// modified — only the generated runtime config.
func ensureAPIServices(api json_util.RawMessage) json_util.RawMessage {
	if len(api) == 0 {
		// No api block means the panel's API integration is deliberately
		// disabled; don't resurrect it behind the user's back.
		return api
	}
	var parsed map[string]any
	if err := json.Unmarshal(api, &parsed); err != nil {
		return api
	}
	services, _ := parsed["services"].([]any)
	have := make(map[string]bool, len(services))
	for _, svc := range services {
		if name, ok := svc.(string); ok {
			have[name] = true
		}
	}
	added := false
	for _, name := range []string{"HandlerService", "StatsService", "RoutingService"} {
		if !have[name] {
			services = append(services, name)
			added = true
		}
	}
	if !added {
		return api
	}
	parsed["services"] = services
	out, err := json.Marshal(parsed)
	if err != nil {
		return api
	}
	return out
}

// ensureStatsPolicy guarantees every policy level in the generated config has
// statsUserOnline enabled, so the core tracks per-email online IPs for the
// panel's online view and access-log-free IP limiting. Generated clients carry
// no explicit level, so level "0" is created when absent. The flag is panel
// infrastructure and is forced on even over an explicit false in the template,
// same as the api services above. An entirely missing or unparsable policy
// block is left alone; the stored template itself is never modified — only the
// generated runtime config.
func ensureStatsPolicy(policy json_util.RawMessage) json_util.RawMessage {
	if len(policy) == 0 {
		return policy
	}
	var parsed map[string]any
	if err := json.Unmarshal(policy, &parsed); err != nil {
		return policy
	}
	levels, _ := parsed["levels"].(map[string]any)
	if levels == nil {
		levels = make(map[string]any)
	}
	if _, ok := levels["0"]; !ok {
		levels["0"] = map[string]any{}
	}
	changed := false
	for _, raw := range levels {
		level, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if enabled, ok := level["statsUserOnline"].(bool); !ok || !enabled {
			level["statsUserOnline"] = true
			changed = true
		}
	}
	if !changed {
		return policy
	}
	parsed["levels"] = levels
	out, err := json.Marshal(parsed)
	if err != nil {
		return policy
	}
	return out
}

func resolveXrayLogPaths(logCfg json_util.RawMessage) json_util.RawMessage {
	if len(logCfg) == 0 {
		return logCfg
	}
	var parsed map[string]any
	if err := json.Unmarshal(logCfg, &parsed); err != nil {
		return logCfg
	}
	changed := false
	for _, key := range []string{"access", "error"} {
		v, ok := parsed[key].(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || strings.EqualFold(trimmed, "none") {
			continue
		}
		base := path.Base(filepath.ToSlash(trimmed))
		if base == "" || base == "." || base == ".." || base == "/" {
			continue
		}
		confined := filepath.Join(config.GetLogFolder(), base)
		if confined == trimmed {
			continue
		}
		parsed[key] = confined
		changed = true
	}
	if !changed {
		return logCfg
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		return logCfg
	}
	return out
}

// stripDisabledRules removes routing rules marked `enabled: false` from the
// generated runtime config and strips the panel-only `enabled` key from the
// rest, since xray-core has no such field. The internal api rule is always
// kept (see isApiRule) so traffic stats can't be toggled off. The stored
// template is untouched — only the generated config is filtered.
func stripDisabledRules(routerCfg json_util.RawMessage) json_util.RawMessage {
	if len(routerCfg) == 0 {
		return routerCfg
	}
	var parsed map[string]any
	if err := json.Unmarshal(routerCfg, &parsed); err != nil {
		return routerCfg
	}
	rules, ok := parsed["rules"].([]any)
	if !ok || len(rules) == 0 {
		return routerCfg
	}

	var activeRules []any
	changed := false
	for _, rawRule := range rules {
		rule, ok := rawRule.(map[string]any)
		if !ok {
			activeRules = append(activeRules, rawRule)
			continue
		}

		if enabledRaw, exists := rule["enabled"]; exists {
			// The internal api rule carries traffic stats and must never be
			// dropped, even if it was somehow marked disabled.
			enabled, ok := enabledRaw.(bool)
			if ok && !enabled && !isApiRule(rule) {
				changed = true
				continue
			}
			delete(rule, "enabled")
			changed = true
		}
		activeRules = append(activeRules, rule)
	}

	if !changed {
		return routerCfg
	}

	parsed["rules"] = activeRules
	out, err := json.Marshal(parsed)
	if err != nil {
		return routerCfg
	}
	return out
}

// GetXrayTraffic fetches the current traffic statistics from the running Xray process.
func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		err := errors.New("xray is not running")
		logger.Debug("Attempted to fetch Xray traffic, but Xray is not running:", err)
		return nil, nil, err
	}
	apiPort := process.GetAPIPort()
	if err := s.xrayAPI.Init(apiPort); err != nil {
		logger.Debug("Failed to initialize Xray API:", err)
		return nil, nil, err
	}
	defer s.xrayAPI.Close()

	traffic, clientTraffic, err := s.xrayAPI.GetTraffic()
	if err != nil {
		logger.Debug("Failed to fetch Xray traffic:", err)
		return nil, nil, err
	}
	return traffic, clientTraffic, nil
}

// GetOnlineUsers returns connection-based online users (email + source IPs)
// from the running core's online-stats API. ok=false means the API is not
// available — xray isn't running or the core predates the online-stats RPCs —
// and callers must use the legacy traffic-delta / access-log paths. The
// capability is probed lazily per process: an Unimplemented answer pins this
// core as unsupported until the next restart, while transient errors leave the
// capability undecided so a flaky poll can't lock in legacy mode.
func (s *XrayService) GetOnlineUsers() ([]xray.OnlineUser, bool, error) {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		return nil, false, nil
	}
	if process.OnlineAPISupport() == xray.OnlineAPIUnsupported {
		return nil, false, nil
	}
	if err := s.xrayAPI.Init(process.GetAPIPort()); err != nil {
		logger.Debug("Failed to initialize Xray API:", err)
		return nil, false, err
	}
	defer s.xrayAPI.Close()

	users, err := s.xrayAPI.GetOnlineUsers()
	if err != nil {
		if xray.IsUnimplementedErr(err) {
			process.SetOnlineAPISupport(xray.OnlineAPIUnsupported)
			logger.Info("xray core does not support the online-stats API; falling back to traffic-delta onlines and access-log IP limit")
			return nil, false, nil
		}
		logger.Debug("Failed to fetch Xray online users:", err)
		return nil, false, err
	}
	if process.OnlineAPISupport() == xray.OnlineAPIUnknown {
		process.SetOnlineAPISupport(xray.OnlineAPISupported)
		logger.Info("xray core supports the online-stats API; using connection-based onlines and access-log-free IP limit")
	}
	return users, true, nil
}

// BalancerStatus is the live view of one balancer for the panel UI. Running
// is false when the balancer isn't present in the running core (e.g. xray is
// stopped or the balancer hasn't been saved/applied yet).
type BalancerStatus struct {
	Tag      string   `json:"tag"`
	Running  bool     `json:"running"`
	Override string   `json:"override"`
	Selected []string `json:"selected"`
}

// GetBalancersStatus queries the running core for the live state of the
// given balancer tags. Per-tag failures are reported as Running=false rather
// than failing the whole call, so the UI can render saved-but-not-applied
// balancers alongside live ones.
func (s *XrayService) GetBalancersStatus(tags []string) ([]BalancerStatus, error) {
	statuses := make([]BalancerStatus, 0, len(tags))
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		for _, tag := range tags {
			statuses = append(statuses, BalancerStatus{Tag: tag})
		}
		return statuses, nil
	}
	if err := s.xrayAPI.Init(process.GetAPIPort()); err != nil {
		return nil, err
	}
	defer s.xrayAPI.Close()

	for _, tag := range tags {
		info, err := s.xrayAPI.GetBalancerInfo(tag)
		if err != nil {
			logger.Debug("get balancer info [", tag, "] failed:", err)
			statuses = append(statuses, BalancerStatus{Tag: tag})
			continue
		}
		statuses = append(statuses, BalancerStatus{
			Tag:      tag,
			Running:  true,
			Override: info.Override,
			Selected: info.Selected,
		})
	}
	return statuses, nil
}

// OverrideBalancer forces a balancer in the running core to use the given
// outbound tag; an empty target clears the override. When target names
// another balancer, the override resolves to the loopback outbound that
// routes traffic through the target balancer via the routing rules.
func (s *XrayService) OverrideBalancer(tag, target string) error {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		return errors.New("xray is not running")
	}
	if target != "" {
		resolved, err := s.resolveOverrideTarget(target)
		if err != nil {
			return err
		}
		if resolved != "" {
			target = resolved
		}
	}
	if err := s.xrayAPI.Init(process.GetAPIPort()); err != nil {
		return err
	}
	defer s.xrayAPI.Close()
	return s.xrayAPI.SetBalancerTarget(tag, target)
}

// resolveOverrideTarget checks if target names a balancer and, if so,
// returns the loopback outbound tag that routes to it through the
// routing rules. Returns empty if target is already a concrete outbound.
func (s *XrayService) resolveOverrideTarget(target string) (string, error) {
	template, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return "", err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(template), &cfg); err != nil {
		return "", err
	}
	routing, _ := cfg["routing"].(map[string]any)
	if routing == nil {
		return "", nil
	}
	rules, _ := routing["rules"].([]any)
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if rule["balancerTag"] != target {
			continue
		}
		inboundTags, ok := rule["inboundTag"].([]any)
		if !ok || len(inboundTags) == 0 {
			continue
		}
		if lbTag, ok := inboundTags[0].(string); ok && strings.HasPrefix(lbTag, "_bl_") {
			return lbTag, nil
		}
	}
	return "", nil
}

// TestRoute asks the running core which outbound its router picks for the
// described connection.
func (s *XrayService) TestRoute(req xray.RouteTestRequest) (*xray.RouteTestResult, error) {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		return nil, errors.New("xray is not running")
	}
	if err := s.xrayAPI.Init(process.GetAPIPort()); err != nil {
		return nil, err
	}
	defer s.xrayAPI.Close()
	return s.xrayAPI.TestRoute(req)
}

// RestartXray reconciles the running Xray process with the current desired
// config. When isForce is false it first tries to apply the changes through
// the Xray gRPC API without restarting the process (inbounds, outbounds and
// routing rules/balancers are hot-reloadable); only changes the core cannot
// take at runtime — or a force request — stop and restart the process.
func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart Xray, force:", isForce)
	if !isForce && isManuallyStopped.Load() {
		return nil
	}
	isManuallyStopped.Store(false)

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	process := currentXrayProcess()
	if process != nil && process.IsRunning() {
		configUnchanged := process.GetConfig().Equals(xrayConfig)
		if !isForce && configUnchanged && !isNeedXrayRestart.Load() {
			logger.Debug("It does not need to restart Xray")
			return nil
		}
		if !isForce && !configUnchanged && s.tryHotApply(process, xrayConfig) {
			logger.Info("Xray config changes applied through the core API, no restart needed")
			return nil
		}
		_ = process.Stop()
	}

	process = xray.NewProcess(xrayConfig)
	xrayState.replace(process)
	s.xrayAPI.StatsLastValues = nil
	err = process.Start()
	if err != nil {
		return err
	}

	// LUCX-HOOK: AWG — the Xray-owned tunN device dies with the process, taking
	// the per-inbound policy route (default dev tunN table 1000+N) with it. The
	// AWG reconcile cron would re-add it within 10 s, but that leaves a dead
	// window where routed AWG clients lose internet (VladufQa's "must re-pick
	// the outbound after restarting Xray" — re-saving the inbound merely
	// triggered a manual reconcile ahead of the cron). Restore it here,
	// synchronously with the restart, so there is no window.
	s.ensureAwgRouting()
	s.ensureQwdttRouting()
	// END LUCX-HOOK

	return nil
}

// LUCX-HOOK: ensureAwgRouting re-derives the desired AWG sidecar state from the
// DB and runs the reconcile-loop convergence (ensureXrayRouting + ensureNatRules)
// synchronously. Called right after Xray (re)starts so the tunN policy route is
// restored in the same instant the new tunN appears, not on the next 10 s tick.
// Best-effort: failures are logged, never fail the Xray start itself.
func (s *XrayService) ensureAwgRouting() {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("awg: post-restart routing restore, get inbounds failed:", err)
		return
	}
	var desired []awg.Instance
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := awg.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
		}
	}
	if len(desired) > 0 {
		awg.GetManager().Reconcile(desired)
	}
}

// ensureQwdttRouting restores wdtt0→tunN policy routes after Xray restart.
func (s *XrayService) ensureQwdttRouting() {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return
	}
	for _, ib := range inbounds {
		if ib.Protocol != model.Qwdtt || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := tunnel.QwdttInstanceFromInbound(ib); ok && inst.RouteThroughXray {
			tunnel.GetManager().EnsureQwdttRouting(inst)
		}
	}
}

// END LUCX-HOOK

// tryHotApply attempts to reconcile the running Xray instance with newCfg
// through the core gRPC API (HandlerService for inbounds/outbounds,
// RoutingService for rules/balancers). It returns true when the running
// instance now matches newCfg; on any failure it returns false and the
// caller falls back to a full process restart, which cleans up whatever was
// partially applied. Callers must hold the package-level lock.
func (s *XrayService) tryHotApply(process *xray.Process, newCfg *xray.Config) bool {
	oldCfg := process.GetConfig()
	diff, ok := xray.ComputeHotDiff(oldCfg, newCfg)
	if !ok {
		logger.Debug("hot apply: config change is not API-applicable, falling back to restart")
		return false
	}
	if diff.Empty() {
		process.SetConfig(newCfg)
		return true
	}

	apiPort := process.GetAPIPort()
	if apiPort <= 0 {
		return false
	}
	// A dedicated client: s.xrayAPI may be in use by traffic polling on other
	// service instances and is reset around restarts.
	hotAPI := xray.XrayAPI{}
	if err := hotAPI.Init(apiPort); err != nil {
		logger.Debug("hot apply: failed to init xray api:", err)
		return false
	}
	defer hotAPI.Close()

	// Removals first so changed handlers and port swaps never collide with
	// the additions that follow.
	for _, u := range diff.RemovedUsers {
		if err := hotAPI.RemoveUser(u.Tag, u.Email); err != nil && !xray.IsMissingHandlerErr(err) {
			logger.Info("hot apply: remove user [", u.Email, "] from [", u.Tag, "] failed:", err)
			return false
		}
	}
	for _, tag := range diff.RemovedInboundTags {
		if err := hotAPI.DelInbound(tag); err != nil && !xray.IsMissingHandlerErr(err) {
			logger.Info("hot apply: remove inbound [", tag, "] failed:", err)
			return false
		}
	}
	for _, tag := range diff.RemovedOutboundTags {
		if err := hotAPI.DelOutbound(tag); err != nil && !xray.IsMissingHandlerErr(err) {
			logger.Info("hot apply: remove outbound [", tag, "] failed:", err)
			return false
		}
	}
	for _, ob := range diff.AddedOutbounds {
		if err := addOutboundReconciling(&hotAPI, ob); err != nil {
			logger.Info("hot apply: add outbound failed:", err)
			return false
		}
	}
	for _, ib := range diff.AddedInbounds {
		if err := addInboundReconciling(&hotAPI, ib); err != nil {
			logger.Info("hot apply: add inbound failed:", err)
			return false
		}
	}
	for _, u := range diff.AddedUsers {
		if err := addUserReconciling(&hotAPI, u); err != nil {
			logger.Info("hot apply: add user [", u.Email, "] to [", u.Tag, "] failed:", err)
			return false
		}
	}
	if diff.RoutingConfig != nil {
		if err := hotAPI.ApplyRoutingConfig(diff.RoutingConfig); err != nil {
			logger.Info("hot apply: apply routing config failed:", err)
			return false
		}
	}

	process.SetConfig(newCfg)
	return true
}

// addUserReconciling adds a user, and on an email conflict (the user was
// already applied through the runtime API) replaces the existing user instead.
func addUserReconciling(api *xray.XrayAPI, u xray.UserOp) error {
	err := api.AddUser(u.Protocol, u.Tag, u.User)
	if err == nil || !xray.IsUserExistsErr(err) {
		return err
	}
	if delErr := api.RemoveUser(u.Tag, u.Email); delErr != nil && !xray.IsMissingHandlerErr(delErr) {
		return delErr
	}
	return api.AddUser(u.Protocol, u.Tag, u.User)
}

// addInboundReconciling adds an inbound, and on a tag conflict (the handler
// was already created through the runtime API while the stored snapshot was
// stale) replaces the existing handler instead.
func addInboundReconciling(api *xray.XrayAPI, inbound []byte) error {
	err := api.AddInbound(inbound)
	if err == nil || !xray.IsExistingTagErr(err) {
		return err
	}
	var meta struct {
		Tag string `json:"tag"`
	}
	if jsonErr := json.Unmarshal(inbound, &meta); jsonErr != nil || meta.Tag == "" {
		return err
	}
	if delErr := api.DelInbound(meta.Tag); delErr != nil && !xray.IsMissingHandlerErr(delErr) {
		return delErr
	}
	return api.AddInbound(inbound)
}

// addOutboundReconciling mirrors addInboundReconciling for outbounds.
func addOutboundReconciling(api *xray.XrayAPI, outbound []byte) error {
	err := api.AddOutbound(outbound)
	if err == nil || !xray.IsExistingTagErr(err) {
		return err
	}
	var meta struct {
		Tag string `json:"tag"`
	}
	if jsonErr := json.Unmarshal(outbound, &meta); jsonErr != nil || meta.Tag == "" {
		return err
	}
	if delErr := api.DelOutbound(meta.Tag); delErr != nil && !xray.IsMissingHandlerErr(delErr) {
		return delErr
	}
	return api.AddOutbound(outbound)
}

// StopXray stops the running Xray process.
func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	isManuallyStopped.Store(true)
	logger.Debug("Attempting to stop Xray...")
	process := currentXrayProcess()
	if process != nil && process.IsRunning() {
		return process.Stop()
	}
	return errors.New("xray is not running")
}

// SetToNeedRestart marks that Xray needs to be restarted.
func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

// GetXrayAPIPort returns the port the local xray process is listening on
// for its gRPC HandlerService, or 0 when xray isn't currently running.
// Exposed for the runtime package's LocalRuntime adapter without a
// service-package import cycle.
func (s *XrayService) GetXrayAPIPort() int {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		return 0
	}
	return process.GetAPIPort()
}

// IsNeedRestartAndSetFalse checks if restart is needed and resets the flag to false.
func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CompareAndSwap(true, false)
}

// ApplyPendingRestart consumes the need-restart flag and restarts Xray. If the
// restart fails (for example GetXrayConfig hits a transient DB error and leaves
// the old process running), it re-arms the flag so the next tick retries instead
// of silently dropping the pending config change.
func (s *XrayService) ApplyPendingRestart() {
	if !s.IsNeedRestartAndSetFalse() {
		return
	}
	if err := s.RestartXray(false); err != nil {
		logger.Error("restart xray failed:", err)
		s.SetToNeedRestart()
	}
}

// DidXrayCrash checks if Xray crashed by verifying it's not running and wasn't manually stopped.
func (s *XrayService) DidXrayCrash() bool {
	return !s.IsXrayRunning() && !isManuallyStopped.Load()
}

// liftXhttpSessionIDKeys renames the legacy XHTTP session keys
// (sessionPlacement/sessionKey) to the v26.6.22 #6258 names
// (sessionIDPlacement/sessionIDKey) inside a streamSettings map. xray-core kept
// no fallback for the old names, so a config stored before the rename would be
// silently ignored by the engine. Returns true if it changed anything.
func liftXhttpSessionIDKeys(stream map[string]any) bool {
	xhttp, ok := stream["xhttpSettings"].(map[string]any)
	if !ok {
		return false
	}
	changed := false
	for legacy, renamed := range map[string]string{
		"sessionPlacement": "sessionIDPlacement",
		"sessionKey":       "sessionIDKey",
	} {
		v, has := xhttp[legacy]
		if !has {
			continue
		}
		if _, exists := xhttp[renamed]; !exists {
			xhttp[renamed] = v
		}
		delete(xhttp, legacy)
		changed = true
	}
	return changed
}

// liftOutboundsXhttpSessionIDKeys applies liftXhttpSessionIDKeys to every
// outbound's streamSettings in the raw outbounds array. The original bytes are
// returned untouched when nothing needs lifting, so an unchanged config never
// looks modified to the hot-reload diff.
func liftOutboundsXhttpSessionIDKeys(raw json_util.RawMessage) json_util.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var outbounds []map[string]any
	if err := json.Unmarshal(raw, &outbounds); err != nil {
		return raw
	}
	changed := false
	for _, ob := range outbounds {
		if stream, ok := ob["streamSettings"].(map[string]any); ok {
			if liftXhttpSessionIDKeys(stream) {
				changed = true
			}
		}
	}
	if !changed {
		return raw
	}
	if rewritten, err := json.Marshal(outbounds); err == nil {
		return rewritten
	}
	return raw
}
