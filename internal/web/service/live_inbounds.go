// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// liveInboundGraceMs keeps an inbound listed for a client this long after the
// last sign of use, so the clients list doesn't flicker between polls.
const liveInboundGraceMs = 30_000

// LiveInbound is one inbound a client is using right now. Weight orders a
// client's inbounds: bytes moved for metered sidecars, socket count for Xray.
type LiveInbound struct {
	Tag      string `json:"tag"`
	Remark   string `json:"remark"`
	Protocol string `json:"protocol"`
	Weight   int64  `json:"-"`
}

type liveInboundEntry struct {
	lastSeen int64
	weight   int64
}

var liveInbounds = struct {
	sync.Mutex
	byEmail map[string]map[string]liveInboundEntry
}{byEmail: map[string]map[string]liveInboundEntry{}}

// RecordLiveInbound notes that email used the inbound tag at nowMs. A sidecar
// collector calls it with the bytes it attributed to that client.
func RecordLiveInbound(email, tag string, weight, nowMs int64) {
	email, tag = strings.TrimSpace(email), strings.TrimSpace(tag)
	if email == "" || tag == "" {
		return
	}
	liveInbounds.Lock()
	defer liveInbounds.Unlock()
	tags := liveInbounds.byEmail[email]
	if tags == nil {
		tags = map[string]liveInboundEntry{}
		liveInbounds.byEmail[email] = tags
	}
	tags[tag] = liveInboundEntry{lastSeen: nowMs, weight: weight}
}

// LiveInboundsByEmail returns, per client email, the inbounds used within the
// grace window, busiest first. Expired entries are dropped.
func (s *InboundService) LiveInboundsByEmail() map[string][]LiveInbound {
	meta := liveInboundMeta()
	now := time.Now().UnixMilli()
	liveInbounds.Lock()
	defer liveInbounds.Unlock()
	out := make(map[string][]LiveInbound, len(liveInbounds.byEmail))
	for email, tags := range liveInbounds.byEmail {
		for tag, e := range tags {
			if now-e.lastSeen > liveInboundGraceMs {
				delete(tags, tag)
				continue
			}
			m, ok := meta[tag]
			if !ok {
				continue
			}
			m.Weight = e.weight
			out[email] = append(out[email], m)
		}
		if len(tags) == 0 {
			delete(liveInbounds.byEmail, email)
			continue
		}
		sort.SliceStable(out[email], func(i, j int) bool {
			a, b := out[email][i], out[email][j]
			if a.Weight != b.Weight {
				return a.Weight > b.Weight
			}
			return a.Remark < b.Remark
		})
	}
	return out
}

func liveInboundMeta() map[string]LiveInbound {
	var rows []model.Inbound
	if err := database.GetDB().Select("tag", "remark", "protocol").Find(&rows).Error; err != nil {
		return nil
	}
	meta := make(map[string]LiveInbound, len(rows))
	for _, ib := range rows {
		remark := strings.TrimSpace(ib.Remark)
		if remark == "" {
			remark = ib.Tag
		}
		meta[ib.Tag] = LiveInbound{Tag: ib.Tag, Remark: remark, Protocol: string(ib.Protocol)}
	}
	return meta
}

// RecordXrayLiveInbounds attributes Xray's online users to inbounds. Xray's
// per-user counters span every inbound a client is attached to, so the inbound
// is found from the connection itself: each online client IP is matched to the
// server address:port it holds an established TCP socket with. UDP inbounds
// (hysteria) have no socket table, so a client IP with no TCP match is credited
// to the client's UDP inbounds instead.
func (s *InboundService) RecordXrayLiveInbounds(users []xray.OnlineUser) {
	if len(users) == 0 {
		return
	}
	sockets, err := readEstablishedTCP()
	if err != nil {
		return
	}
	endpoints, udpTags := xrayInboundEndpoints()
	attached := emailInboundTags(users)
	now := time.Now().UnixMilli()
	for _, u := range users {
		mine := attached[u.Email]
		if len(mine) == 0 {
			continue
		}
		var unmatched []string
		for _, ip := range u.IPs {
			found := false
			for local, n := range sockets[normalizeIP(ip.IP)] {
				tag := endpoints.lookup(local)
				if tag == "" || !mine[tag] {
					continue
				}
				RecordLiveInbound(u.Email, tag, int64(n), now)
				RecordLiveIP(u.Email, ip.IP, tag, now)
				found = true
			}
			if !found {
				unmatched = append(unmatched, ip.IP)
			}
		}
		// An online IP with no TCP socket of its own is on a UDP inbound.
		for tag := range udpTags {
			if !mine[tag] || len(unmatched) == 0 {
				continue
			}
			RecordLiveInbound(u.Email, tag, 0, now)
			for _, ip := range unmatched {
				RecordLiveIP(u.Email, ip, tag, now)
			}
		}
		if len(unmatched) > 0 && !hasAny(udpTags, mine) {
			for _, ip := range unmatched {
				RecordLiveIP(u.Email, ip, "", now)
			}
		}
	}
	recordSidecarIPs(sockets, now)
}

// recordSidecarIPs credits the client IPs connected to a sidecar inbound
// (TrustTunnel, Mieru, AnyTLS…) to its user when exactly one client is live on
// it. Sidecars don't report per-connection users, so with several live
// clients the IPs stay unattributed rather than guessed.
func recordSidecarIPs(sockets map[string]map[string]int, now int64) {
	eps := sidecarEndpoints()
	if len(eps) == 0 {
		return
	}
	for remote, a := range attributeSidecarIPs(sockets, eps, liveEmailsByTag(now)) {
		RecordLiveIP(a.email, remote, a.tag, now)
	}
}

func hasAny(set, in map[string]bool) bool {
	for k := range set {
		if in[k] {
			return true
		}
	}
	return false
}

type sidecarIP struct{ email, tag string }

// attributeSidecarIPs maps each remote IP connected to a sidecar endpoint to
// that inbound's user, but only when the inbound has exactly one live user.
func attributeSidecarIPs(sockets map[string]map[string]int, eps endpointMap, liveByTag map[string][]string) map[string]sidecarIP {
	out := map[string]sidecarIP{}
	for remote, locals := range sockets {
		for local := range locals {
			tag := eps.lookup(local)
			if emails := liveByTag[tag]; tag != "" && len(emails) == 1 {
				out[remote] = sidecarIP{email: emails[0], tag: tag}
			}
		}
	}
	return out
}

// sidecarEndpoints maps the public address:port of each sidecar inbound.
func sidecarEndpoints() endpointMap {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil
	}
	var hosts []model.Host
	_ = db.Where("is_disabled = ?", false).Find(&hosts).Error
	hostsByInbound := map[int][]model.Host{}
	for _, h := range hosts {
		hostsByInbound[h.InboundId] = append(hostsByInbound[h.InboundId], h)
	}
	eps := endpointMap{}
	for _, ib := range inbounds {
		if !lucxRuntimeSidecar(ib.Protocol) || ib.Port <= 0 || ib.Tag == "" {
			continue
		}
		// An AnyTLS server with per-user stats reports its users' IPs itself.
		if _, ok := tunnel.ReadAnytlsStats(ib.Id); ok && ib.Protocol == model.Anytls {
			continue
		}
		listen := strings.TrimSpace(ib.Listen)
		port := strconv.Itoa(ib.Port)
		switch {
		case tunnel.IsLoopbackListen(listen):
			// LUCX-HOOK: behind the SNI gateway the sidecar only sees the
			// gateway on loopback. Its Host row keeps the public IP:port the
			// client really connects to, and the gateway's socket there carries
			// the real client IP — same as xrayInboundEndpoints.
			for _, h := range hostsByInbound[ib.Id] {
				if ip := net.ParseIP(strings.TrimSpace(h.Address)); ip != nil && h.Port > 0 {
					eps[net.JoinHostPort(ip.String(), strconv.Itoa(h.Port))] = ib.Tag
				}
			}
			// END LUCX-HOOK
		case listen == "" || listen == "0.0.0.0" || listen == "::":
			eps["*:"+port] = ib.Tag
		default:
			if ip := net.ParseIP(listen); ip != nil {
				eps[net.JoinHostPort(ip.String(), port)] = ib.Tag
			}
		}
	}
	return eps
}

func liveEmailsByTag(now int64) map[string][]string {
	liveInbounds.Lock()
	defer liveInbounds.Unlock()
	out := map[string][]string{}
	for email, tags := range liveInbounds.byEmail {
		for tag, e := range tags {
			if now-e.lastSeen <= liveInboundGraceMs {
				out[tag] = append(out[tag], email)
			}
		}
	}
	return out
}

// LiveIP is a client address seen recently, with its location/provider and
// the inbounds (remarks) it is connected to.
type LiveIP struct {
	IP       string   `json:"ip"`
	LastSeen int64    `json:"lastSeen"`
	Inbounds []string `json:"inbounds,omitempty"`
	IPProvider
	tags []string
}

type liveIPEntry struct {
	lastSeen int64
	tags     map[string]int64
}

var liveIPs = struct {
	sync.Mutex
	byEmail map[string]map[string]*liveIPEntry
}{byEmail: map[string]map[string]*liveIPEntry{}}

// RecordLiveIP notes that email was connected from ip, on inbound tag when
// known, at nowMs. Loopback and unspecified addresses (internal bridges) are
// ignored.
func RecordLiveIP(email, ip, tag string, nowMs int64) {
	email, tag = strings.TrimSpace(email), strings.TrimSpace(tag)
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if email == "" || parsed == nil || parsed.IsLoopback() || parsed.IsUnspecified() || ClientNeverLogsIPs(email) {
		return
	}
	ip = normalizeIP(ip)
	liveIPs.Lock()
	defer liveIPs.Unlock()
	if liveIPs.byEmail[email] == nil {
		liveIPs.byEmail[email] = map[string]*liveIPEntry{}
	}
	e := liveIPs.byEmail[email][ip]
	if e == nil {
		e = &liveIPEntry{tags: map[string]int64{}}
		liveIPs.byEmail[email][ip] = e
	}
	e.lastSeen = nowMs
	if tag != "" {
		e.tags[tag] = nowMs
	}
}

func liveIPsSnapshot(now int64) map[string][]LiveIP {
	liveIPs.Lock()
	defer liveIPs.Unlock()
	out := map[string][]LiveIP{}
	for email, ips := range liveIPs.byEmail {
		for ip, e := range ips {
			if now-e.lastSeen > liveInboundGraceMs {
				delete(ips, ip)
				continue
			}
			l := LiveIP{IP: ip, LastSeen: e.lastSeen}
			for tag, seen := range e.tags {
				if now-seen <= liveInboundGraceMs {
					l.tags = append(l.tags, tag)
				}
			}
			sort.Strings(l.tags)
			out[email] = append(out[email], l)
		}
		if len(ips) == 0 {
			delete(liveIPs.byEmail, email)
		}
	}
	return out
}

// ClientDevices is the clients-list "Devices" cell: registered HWID devices
// against the effective limit (0 = unlimited) and the addresses live now.
type ClientDevices struct {
	Count int      `json:"count"`
	Limit int      `json:"limit"`
	IPs   []LiveIP `json:"ips,omitempty"`
}

// ClientDevicesByEmail returns the Devices cell for every client. The limit
// mirrors effectiveHwidLimitForSubID: the highest limit among the enabled
// clients sharing a subscription.
func (s *InboundService) ClientDevicesByEmail() map[string]ClientDevices {
	db := database.GetDB()
	type clientRow struct {
		Email string
		SubID string
	}
	var clients []clientRow
	if err := db.Table("clients").Select("email, sub_id").Find(&clients).Error; err != nil {
		return nil
	}
	type subAgg struct {
		SubID string
		N     int
	}
	var limits, counts []subAgg
	_ = db.Table("clients").Select("sub_id, COALESCE(MAX(limit_hwid), 0) AS n").
		Where("enable = ?", true).Group("sub_id").Scan(&limits).Error
	_ = db.Table("client_hwids").Select("sub_id, COUNT(*) AS n").Group("sub_id").Scan(&counts).Error
	limitBySub := make(map[string]int, len(limits))
	for _, l := range limits {
		limitBySub[l.SubID] = l.N
	}
	countBySub := make(map[string]int, len(counts))
	for _, c := range counts {
		countBySub[c.SubID] = c.N
	}
	ips := liveIPsSnapshot(time.Now().UnixMilli())
	meta := liveInboundMeta()
	out := make(map[string]ClientDevices, len(clients))
	for _, c := range clients {
		d := ClientDevices{}
		if sub := strings.TrimSpace(c.SubID); sub != "" {
			d.Count, d.Limit = countBySub[sub], limitBySub[sub]
		}
		for _, ip := range ips[c.Email] {
			shown, info := LabelClientIP(ip.IP)
			if shown == "" {
				continue // "Mask Address: full"
			}
			ip.IP, ip.IPProvider = shown, info
			for _, tag := range ip.tags {
				if m, ok := meta[tag]; ok {
					ip.Inbounds = append(ip.Inbounds, m.Remark)
				}
			}
			d.IPs = append(d.IPs, ip)
		}
		sort.Slice(d.IPs, func(i, j int) bool { return d.IPs[i].LastSeen > d.IPs[j].LastSeen })
		out[c.Email] = d
	}
	return out
}

// endpointMap resolves a local "ip:port" to the Xray inbound serving it.
// Wildcard listens are stored under "*:port".
type endpointMap map[string]string

func (m endpointMap) lookup(local string) string {
	if tag, ok := m[local]; ok {
		return tag
	}
	if _, port, err := net.SplitHostPort(local); err == nil {
		return m["*:"+port]
	}
	return ""
}

// xrayInboundEndpoints maps each enabled Xray inbound to the address clients
// dial. A public listen is used as is; a loopback listen (fronted by a gateway
// or web server) is reached through its Host overrides' address:port.
func xrayInboundEndpoints() (endpointMap, map[string]bool) {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil, nil
	}
	var hosts []model.Host
	_ = db.Where("is_disabled = ?", false).Find(&hosts).Error
	hostsByInbound := map[int][]model.Host{}
	for _, h := range hosts {
		hostsByInbound[h.InboundId] = append(hostsByInbound[h.InboundId], h)
	}
	eps := endpointMap{}
	udp := map[string]bool{}
	for i := range inbounds {
		ib := &inbounds[i]
		if lucxRuntimeSidecar(ib.Protocol) || ib.Port <= 0 || ib.Tag == "" {
			continue
		}
		if !tunnel.InboundUsesTCP(ib) {
			udp[ib.Tag] = true
			continue
		}
		port := strconv.Itoa(ib.Port)
		listen := strings.TrimSpace(ib.Listen)
		switch {
		case tunnel.IsLoopbackListen(listen):
			for _, h := range hostsByInbound[ib.Id] {
				if ip := net.ParseIP(strings.TrimSpace(h.Address)); ip != nil && h.Port > 0 {
					eps[net.JoinHostPort(ip.String(), strconv.Itoa(h.Port))] = ib.Tag
				}
			}
		case listen == "" || listen == "0.0.0.0" || listen == "::":
			eps["*:"+port] = ib.Tag
		default:
			if ip := net.ParseIP(listen); ip != nil {
				eps[net.JoinHostPort(ip.String(), port)] = ib.Tag
			}
		}
	}
	return eps, udp
}

// emailInboundTags returns, for the given online users, the tags of the
// inbounds each email is attached to.
func emailInboundTags(users []xray.OnlineUser) map[string]map[string]bool {
	emails := make([]string, 0, len(users))
	for _, u := range users {
		emails = append(emails, u.Email)
	}
	type row struct {
		Email string
		Tag   string
	}
	var rows []row
	_ = database.GetDB().Table("clients").
		Select("clients.email AS email, inbounds.tag AS tag").
		Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
		Joins("JOIN inbounds ON inbounds.id = client_inbounds.inbound_id").
		Where("clients.email IN ?", emails).
		Scan(&rows).Error
	out := map[string]map[string]bool{}
	for _, r := range rows {
		if out[r.Email] == nil {
			out[r.Email] = map[string]bool{}
		}
		out[r.Email][r.Tag] = true
	}
	return out
}

// readEstablishedTCP returns remote IP -> local "ip:port" -> socket count for
// every established TCP connection (IPv4 and IPv6).
func readEstablishedTCP() (map[string]map[string]int, error) {
	out := map[string]map[string]int{}
	var firstErr error
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		parseProcNetTCP(f, out)
		f.Close()
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// parseProcNetTCP adds the established sockets of one /proc/net/tcp{,6} table.
func parseProcNetTCP(r io.Reader, out map[string]map[string]int) {
	sc := bufio.NewScanner(r)
	sc.Scan() // header
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || f[3] != "01" { // 01 = ESTABLISHED
			continue
		}
		local, err1 := procAddr(f[1])
		remote, err2 := procAddr(f[2])
		if err1 != nil || err2 != nil {
			continue
		}
		rip, _, _ := net.SplitHostPort(remote)
		rip = normalizeIP(rip)
		if out[rip] == nil {
			out[rip] = map[string]int{}
		}
		out[rip][local]++
	}
}

// procAddr decodes "0100007F:01BB" (little-endian 32-bit words) to "127.0.0.1:443".
func procAddr(s string) (string, error) {
	host, port, ok := strings.Cut(s, ":")
	if !ok {
		return "", fmt.Errorf("bad address %q", s)
	}
	raw, err := hex.DecodeString(host)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return "", fmt.Errorf("bad address %q", s)
	}
	for i := 0; i+4 <= len(raw); i += 4 {
		raw[i], raw[i+1], raw[i+2], raw[i+3] = raw[i+3], raw[i+2], raw[i+1], raw[i]
	}
	p, err := strconv.ParseUint(port, 16, 16)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(normalizeIP(net.IP(raw).String()), strconv.Itoa(int(p))), nil
}

// normalizeIP folds IPv4-mapped IPv6 ("::ffff:1.2.3.4") to plain IPv4.
func normalizeIP(s string) string {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil {
		return strings.TrimSpace(s)
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}
