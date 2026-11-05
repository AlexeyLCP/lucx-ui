// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const Gateway Name = "gateway"

const (
	gatewayDefaultPort     = 443
	gatewayDropBackend     = "127.0.0.1:1"
	gatewayPassthroughBase = 1443
	gatewayCaddyBase       = 8443
)

const (
	ClassPassthrough = "passthrough"
	ClassCaddy       = "caddy"
	ClassSkip        = "skip"
)

// GatewayRoute is one SNI → backend for Caddy L4. NoProxy marks backends that
// cannot parse PROXY v1. Chan names an in-process l4chan listener (unified
// mode): `l4http` hands the conn to a site block of the same Caddyfile.
type GatewayRoute struct {
	SNI     string `json:"sni"`
	Dest    string `json:"dest,omitempty"`
	Chan    string `json:"chan,omitempty"`
	NoProxy bool   `json:"noProxy,omitempty"`
}

// GatewaySnapshotRow is enough to undo one inbound after Apply.
// Settings is omitted on snapshots taken before lucx.269; revert must not
// blank settings when the field is empty.
type GatewaySnapshotRow struct {
	InboundID       int    `json:"inboundId"`
	Listen          string `json:"listen"`
	Port            int    `json:"port"`
	StreamSettings  string `json:"streamSettings,omitempty"`
	Settings        string `json:"settings,omitempty"`
	HostID          int    `json:"hostId,omitempty"`
	DisabledHostIDs []int  `json:"disabledHostIds,omitempty"`
	// AppliedSettings / AppliedStreamSettings are the inbound's JSON as Apply
	// left it. Revert restores only what Apply changed and keeps edits made
	// while masked (see MergeRevertJSON); empty on older snapshots.
	AppliedSettings       string `json:"appliedSettings,omitempty"`
	AppliedStreamSettings string `json:"appliedStreamSettings,omitempty"`
}

// RevertUpdates is the column map Apply's undo must write. A struct Updates
// drops the empty listen and leaves the inbound on 127.0.0.1 after Revert.
// cur is the inbound as it is now (nil = plain snapshot restore): keys the
// operator changed while the gateway was applied survive the revert.
func RevertUpdates(sr GatewaySnapshotRow, cur ...*model.Inbound) map[string]any {
	stream, settings := sr.StreamSettings, sr.Settings
	if len(cur) > 0 && cur[0] != nil {
		if sr.AppliedStreamSettings != "" {
			stream = MergeRevertJSON(sr.StreamSettings, sr.AppliedStreamSettings, cur[0].StreamSettings)
		}
		if sr.Settings != "" && sr.AppliedSettings != "" {
			settings = MergeRevertJSON(sr.Settings, sr.AppliedSettings, cur[0].Settings)
		}
	}
	u := map[string]any{
		"listen":          sr.Listen,
		"port":            sr.Port,
		"stream_settings": stream,
	}
	if settings != "" {
		u["settings"] = settings
	}
	return u
}

// MergeRevertJSON undoes Apply's changes without losing later edits: it
// starts from the pre-Apply JSON (orig) and takes from cur every key the
// operator changed after Apply (cur differs from applied), recursing into
// objects; keys removed since Apply are removed. Anything that isn't a JSON
// object falls back to orig, the old behaviour.
func MergeRevertJSON(orig, applied, cur string) string {
	var o, a, c map[string]any
	if json.Unmarshal([]byte(orig), &o) != nil || json.Unmarshal([]byte(applied), &a) != nil ||
		json.Unmarshal([]byte(cur), &c) != nil || o == nil || a == nil || c == nil {
		return orig
	}
	b, err := json.Marshal(mergeRevert(o, a, c))
	if err != nil {
		return orig
	}
	return string(b)
}

func mergeRevert(o, a, c map[string]any) map[string]any {
	for k, cv := range c {
		av, inApplied := a[k]
		if inApplied && reflect.DeepEqual(av, cv) {
			continue // untouched since Apply: keep the pre-Apply value
		}
		om, oIsMap := o[k].(map[string]any)
		am, aIsMap := av.(map[string]any)
		cm, cIsMap := cv.(map[string]any)
		if oIsMap && aIsMap && cIsMap {
			o[k] = mergeRevert(om, am, cm)
			continue
		}
		o[k] = cv
	}
	for k := range a {
		if _, still := c[k]; !still {
			delete(o, k)
		}
	}
	return o
}

// GatewayConfig lives in inbound settings. Snapshot empty = mask not applied.
type GatewayConfig struct {
	Remark     string `json:"remark"`
	Enabled    bool   `json:"enabled"`
	PublicHost string `json:"publicHost"`
	BindIP     string `json:"bindIP,omitempty"`
	// BindIPs: every IP the gateway listens on (multi-IP hosts keep each
	// masked inbound on its own IP). BindIP stays the first for old readers.
	BindIPs []string `json:"bindIPs,omitempty"`
	// HTTPBinds are extra IPs that get only the gateway's :80 block (HTTPS
	// redirect + ACME relay) — the panel's own IP, whose :443 the panel keeps.
	HTTPBinds []string             `json:"httpBinds,omitempty"`
	Routes    []GatewayRoute       `json:"routes"`
	Snapshot  []GatewaySnapshotRow `json:"snapshot"`
	Fallback  string               `json:"fallback,omitempty"`
	// Unified: caddy-class routes terminate inside this gateway's Caddy
	// process (site blocks + l4chan) instead of loopback sidecars. Set on
	// new applies only; old configs keep the per-process layout.
	Unified      bool         `json:"unified,omitempty"`
	UFW          bool         `json:"ufw,omitempty"`
	UFWWasActive bool         `json:"ufwWasActive,omitempty"`
	HidePanel    bool         `json:"hidePanel,omitempty"`
	PanelRoutes  []CoverRoute `json:"panelRoutes,omitempty"`
}

// Binds returns the gateway's listen IPs; empty = all interfaces.
func (c GatewayConfig) Binds() []string {
	if len(c.BindIPs) > 0 {
		return CleanBinds(c.BindIPs)
	}
	return CleanBinds([]string{c.BindIP})
}

// CleanBinds trims, drops empties and duplicates, keeps order.
func CleanBinds(ips []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out
}

// gatewayListen renders the layer4 server addresses: one IP:port per bind,
// or :port on all interfaces when there is none.
func gatewayListen(port int, bindIPs []string) string {
	binds := CleanBinds(bindIPs)
	if len(binds) == 0 {
		return ":" + strconv.Itoa(port)
	}
	addrs := make([]string, len(binds))
	for i, ip := range binds {
		addrs[i] = net.JoinHostPort(ip, strconv.Itoa(port))
	}
	return strings.Join(addrs, " ")
}

func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{}
}

func (c GatewayConfig) Merge() GatewayConfig {
	c.PublicHost = strings.ToLower(strings.TrimSpace(c.PublicHost))
	if c.Routes == nil {
		c.Routes = []GatewayRoute{}
	}
	if c.Snapshot == nil {
		c.Snapshot = []GatewaySnapshotRow{}
	}
	return c
}

func (c GatewayConfig) Applied() bool {
	return len(c.Snapshot) > 0
}

func sniMapKey(sni string) string {
	sni = strings.ToLower(strings.TrimSpace(sni))
	sni = strings.ReplaceAll(sni, `"`, "")
	sni = strings.ReplaceAll(sni, " ", "")
	return sni
}

// LocalIPv4 is the IPv4 of the default route. Empty if unknown.
func LocalIPv4() string {
	d := &net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(context.Background(), "udp4", "1.1.1.1:53")
	if err != nil {
		return ""
	}
	defer c.Close()
	host, _, err := net.SplitHostPort(c.LocalAddr().String())
	if err != nil {
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return ""
	}
	return ip.String()
}

// RenderGatewayCaddyfile is Caddy L4 TLS SNI mux. Empty fallback = drop.
// bindIPs set → listen IP:port on each so loopback:port stays free for backends.
func RenderGatewayCaddyfile(listenPort int, routes []GatewayRoute, fallback string, bindIPs ...string) string {
	if listenPort <= 0 {
		listenPort = gatewayDefaultPort
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = gatewayDropBackend
	}
	listen := gatewayListen(listenPort, bindIPs)
	var b strings.Builder
	b.WriteString("{\n\tadmin off\n\tlayer4 {\n\t\t")
	b.WriteString(listen)
	b.WriteString(" {\n")
	// Legacy mode proxies every route over loopback — strip Chan so routes
	// built by a newer preview never emit l4http into a file with no sites.
	legacy := make([]GatewayRoute, len(routes))
	for i, r := range routes {
		r.Chan = ""
		legacy[i] = r
	}
	writeL4Mux(&b, legacy, GatewayRoute{Dest: fallback})
	b.WriteString("\t\t}\n\t}\n}\n")
	return b.String()
}

// RenderUnifiedGatewayCaddyfile renders the same SNI mux plus the absorbed
// caddy-class services as site blocks in one Caddyfile: Chan routes hand the
// conn to a named in-process listener via `l4http` (caddylucx module in the
// merged binary), so TLS terminates in the HTTP app with the real client IP.
func RenderUnifiedGatewayCaddyfile(listenPort int, routes []GatewayRoute, fallbackChan string, bindIPs, sites []string, globals ...string) string {
	return renderUnifiedGateway(listenPort, routes, fallbackChan, bindIPs, bindIPs, sites, globals...)
}

// renderUnifiedGateway is RenderUnifiedGatewayCaddyfile with its own set of
// :80 binds (the gateway IPs plus HTTPBinds).
func renderUnifiedGateway(listenPort int, routes []GatewayRoute, fallbackChan string, bindIPs, httpBinds, sites []string, globals ...string) string {
	if listenPort <= 0 {
		listenPort = gatewayDefaultPort
	}
	listen := gatewayListen(listenPort, bindIPs)
	var b strings.Builder
	b.WriteString("{\n\tadmin off\n\tauto_https off\n\tgrace_period 1s\n\tlog {\n\t\tlevel WARN\n\t}\n\tservers {\n\t\tprotocols h1 h2\n\t}\n")
	for _, g := range globals {
		b.WriteString(g)
	}
	b.WriteString("\tlayer4 {\n\t\t")
	b.WriteString(listen)
	b.WriteString(" {\n")
	writeL4Mux(&b, routes, GatewayRoute{Chan: fallbackChan})
	b.WriteString("\t\t}\n\t}\n}\n")
	hasSite := false
	for _, s := range sites {
		if s = strings.TrimSpace(s); s != "" {
			b.WriteString(s + "\n")
			hasSite = true
		}
	}
	// The sites own :80 on the same IPs: redirect to HTTPS and relay ACME
	// HTTP-01 to acme.sh, so certificates keep renewing behind the gateway.
	if hasSite && listenPort == gatewayDefaultPort {
		writeHTTPRedirectSite(&b, "http://", strings.Join(httpBinds, " "))
	}
	return b.String()
}

// writeL4Mux emits matching_timeout + route blocks shared by both renders.
// Empty fallback is omitted: dialing 127.0.0.1:1 logs connection refused per hit.
func writeL4Mux(b *strings.Builder, routes []GatewayRoute, fallback GatewayRoute) {
	b.WriteString("\t\t\tmatching_timeout 15s\n")
	seen := map[string]bool{}
	i := 0
	for _, r := range routes {
		k := sniMapKey(r.SNI)
		if k == "" || (strings.TrimSpace(r.Dest) == "" && r.Chan == "") || seen[k] {
			continue
		}
		seen[k] = true
		tag := "sni" + strconv.Itoa(i)
		i++
		b.WriteString("\t\t\t@" + tag + " tls sni " + k + "\n")
		b.WriteString("\t\t\troute @" + tag + " {\n")
		writeL4Handler(b, r)
		b.WriteString("\t\t\t}\n")
	}
	if dropFallback(fallback) {
		return
	}
	b.WriteString("\t\t\troute {\n")
	writeL4Handler(b, fallback)
	b.WriteString("\t\t\t}\n")
}

// dropFallback is the blackhole catch-all. A real dest or l4http chan stays.
func dropFallback(r GatewayRoute) bool {
	if strings.TrimSpace(r.Chan) != "" {
		return false
	}
	d := strings.TrimSpace(r.Dest)
	return d == "" || d == gatewayDropBackend
}

func dropDeadChans(routes []GatewayRoute, live map[string]bool) []GatewayRoute {
	out := make([]GatewayRoute, 0, len(routes))
	for _, r := range routes {
		if r.Chan != "" && !live[r.Chan] {
			continue
		}
		out = append(out, r)
	}
	return out
}

func writeL4Handler(b *strings.Builder, r GatewayRoute) {
	if r.Chan != "" {
		b.WriteString("\t\t\t\tl4http " + r.Chan + "\n")
		return
	}
	d := strings.TrimSpace(r.Dest)
	if d == "" {
		d = gatewayDropBackend
	}
	if r.NoProxy {
		b.WriteString("\t\t\t\tproxy " + d + "\n")
	} else {
		b.WriteString("\t\t\t\tproxy " + d + " {\n\t\t\t\t\tproxy_protocol v1\n\t\t\t\t}\n")
	}
}

func gatewayLoopbackDest(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}
