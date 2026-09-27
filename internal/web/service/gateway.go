// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"gorm.io/gorm"
)

type GatewayApplyRequest struct {
	Selected   []int             `json:"selected"`
	Steal      []int             `json:"steal"`
	PublicHost string            `json:"publicHost"`
	UFW        bool              `json:"ufw"`
	HidePanel  bool              `json:"hidePanel"`
	HideNaive  []int             `json:"hideNaive"`
	Inside     []int             `json:"inside"`
	Paths      map[string]string `json:"paths"`
	SNI        map[string]string `json:"sni"`
}

type GatewayPreviewResult struct {
	Applied    bool                `json:"applied"`
	PublicHost string              `json:"publicHost"`
	BindIP     string              `json:"bindIP"`
	Rows       []tunnel.PreviewRow `json:"rows"`
	MaskedIds  []int               `json:"maskedIds,omitempty"`
	UFW        bool                `json:"ufw"`
	UFWAllow   []string            `json:"ufwAllow,omitempty"`
	HidePanel  bool                `json:"hidePanel"`
	WebPort    int                 `json:"webPort,omitempty"`
	WebTLS     bool                `json:"webTLS,omitempty"`
}

func (s *InboundService) GatewayPreview(gatewayID int, publicHost string) (*GatewayPreviewResult, error) {
	gw, others, err := s.gatewayAndOthers(gatewayID)
	if err != nil {
		return nil, err
	}
	cfg, _ := tunnel.GatewayConfigFromInbound(gw)
	if publicHost == "" {
		publicHost = tunnel.ResolveGatewayPublicHost("", cfg.PublicHost, panelPublicHost(), others)
	}
	bindIP := cfg.BindIP
	if bindIP == "" {
		bindIP = tunnel.LocalIPv4()
	}
	var masked []int
	for _, s := range cfg.Snapshot {
		masked = append(masked, s.InboundID)
	}
	rows := tunnel.BuildPreview(gw.Port, publicHost, others, bindIP)
	web, sub := gatewayExtraPorts()
	cert, _ := (&SettingService{}).GetCertFile()
	return &GatewayPreviewResult{
		Applied:    cfg.Applied(),
		PublicHost: publicHost,
		BindIP:     bindIP,
		Rows:       rows,
		MaskedIds:  masked,
		UFW:        cfg.UFW,
		UFWAllow:   tunnel.GatewayUFWAllow(web, sub, tunnel.SSHDPort(), rows, ufwDefaultSelected(rows), cfg.HidePanel),
		HidePanel:  cfg.HidePanel,
		WebPort:    web,
		WebTLS:     strings.TrimSpace(cert) != "",
	}, nil
}

func (s *InboundService) GatewayApply(gatewayID int, req GatewayApplyRequest) error {
	gw, others, err := s.gatewayAndOthers(gatewayID)
	if err != nil {
		return err
	}
	cfg, _ := tunnel.GatewayConfigFromInbound(gw)
	if cfg.Applied() {
		return common.NewError("gateway: revert first")
	}
	byID := map[int]*model.Inbound{}
	for _, o := range others {
		byID[o.Id] = o
	}
	host := tunnel.ResolveGatewayPublicHost(req.PublicHost, cfg.PublicHost, panelPublicHost(), others)
	bindIP := tunnel.LocalIPv4()
	selected := map[int]bool{}
	for _, id := range req.Selected {
		selected[id] = true
	}
	inside := map[int]bool{}
	for _, id := range req.Inside {
		ib := byID[id]
		if ib != nil && ib.Protocol == model.Naive {
			req.HideNaive = append(req.HideNaive, id)
			continue
		}
		inside[id] = true
	}
	if len(selected) == 0 && len(inside) == 0 && len(req.HideNaive) == 0 {
		return common.NewError("gateway: nothing selected")
	}
	db := database.GetDB()
	// Snapshot originals before any SNI edit. REALITY serverNames are never
	// written (SetInboundSNI ignores them); other SNI edits must still revert.
	snap := map[int]*tunnel.GatewaySnapshotRow{}
	remember := func(ib *model.Inbound) {
		if ib == nil || snap[ib.Id] != nil {
			return
		}
		snap[ib.Id] = &tunnel.GatewaySnapshotRow{
			InboundID: ib.Id, Listen: ib.Listen, Port: ib.Port,
			StreamSettings: ib.StreamSettings, Settings: ib.Settings,
		}
	}
	for id := range selected {
		remember(byID[id])
	}
	for id := range inside {
		remember(byID[id])
	}
	for _, id := range req.HideNaive {
		remember(byID[id])
	}
	for idStr, sni := range req.SNI {
		id, err := strconv.Atoi(idStr)
		if err != nil || !selected[id] || inside[id] {
			continue
		}
		tunnel.SetInboundSNI(byID[id], sni)
	}
	if err := applyNaiveMoves(byID, tunnel.PlanNaivePublic(gw.Port, others)); err != nil {
		return err
	}
	rows := tunnel.BuildPreview(gw.Port, host, others, bindIP)
	if len(inside) > 0 {
		if _, cover := selectedSiteHost(rows, selected); !cover {
			return common.NewError("gateway: pick Cover to hide a protocol inside the site")
		}
	}
	if err := hideNaiveOnSite(byID, rows, selected, req.HideNaive); err != nil {
		return err
	}
	routeSelected := map[int]bool{}
	for id, on := range selected {
		if on && !inside[id] {
			routeSelected[id] = true
		}
	}
	if c := tunnel.SNIClash(rows, routeSelected); c != "" {
		return common.NewError("gateway: duplicate SNI", c)
	}
	for _, o := range others {
		// Anything left public on TCP :443 wins the bind race with the
		// gateway — one of them then fails to listen.
		if o == nil || !o.Enable || selected[o.Id] || inside[o.Id] || o.Port != gw.Port ||
			tunnel.IsLoopbackListen(o.Listen) || !tunnel.InboundUsesTCP(o) {
			continue
		}
		return common.NewErrorf("gateway: inbound %q still occupies TCP :%d — select it or move it off", inboundLabel(o), o.Port)
	}
	cert, key := gatewayPanelCertPair()
	for _, row := range rows {
		ib := byID[row.InboundID]
		if !selected[row.InboundID] || row.Class != tunnel.ClassCaddy ||
			ib == nil || ib.Protocol != model.Naive {
			continue
		}
		ncfg, _ := tunnel.ConfigFromInbound(ib)
		if !ncfg.UseAcme {
			continue
		}
		// Auto TLS has no public :80/:443 to renew on once masked — point it
		// at the panel cert when it covers the domain, else fail loudly.
		if err := tunnel.ValidateCertFiles(cert, key, ncfg.Domain); err != nil {
			return common.NewErrorf("gateway: %q uses Auto TLS which can't renew behind masking — set cert/key paths (panel cert doesn't cover %q)", ib.Remark, ncfg.Domain)
		}
		tunnel.SetNaiveCert(ib, cert, key)
		if err := db.Model(ib).Select("settings").Updates(ib).Error; err != nil {
			return err
		}
	}
	if req.HidePanel {
		front := false
		for _, row := range rows {
			if !selected[row.InboundID] {
				continue
			}
			if row.Protocol == string(model.Cover) || row.Protocol == string(model.Tproxy) {
				front = true
				break
			}
		}
		if !front {
			return common.NewError("gateway: hide panel needs Cover or WEB proxy selected")
		}
		routes := panelCoverRoutes()
		if len(routes) == 0 {
			return common.NewError("gateway: set a panel base path (not /) before hiding the panel")
		}
		cfg.HidePanel = true
		cfg.PanelRoutes = routes
	}
	siteHost, _ := selectedSiteHost(rows, selected)
	insidePath, err := applyInside(byID, rows, inside, req.Paths, siteHost)
	if err != nil {
		return err
	}
	var snapRows []tunnel.GatewaySnapshotRow
	for _, row := range rows {
		ib := byID[row.InboundID]
		sr := snap[row.InboundID]
		if ib == nil || sr == nil {
			continue
		}
		move := (selected[row.InboundID] && row.Class != tunnel.ClassSkip && !inside[row.InboundID]) || inside[row.InboundID]
		if !move && !hideNaiveID(req.HideNaive, row.InboundID) {
			delete(snap, row.InboundID)
			continue
		}
		if move {
			tunnel.MoveListen(ib, row.NewListen, row.NewPort)
			if !inside[row.InboundID] && !row.NoProxy && tunnel.XrayAcceptsProxyProtocol(ib.Protocol) {
				ib.StreamSettings = tunnel.SetAcceptProxyProtocol(ib.StreamSettings, true)
			}
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Updates(map[string]any{
			"listen": ib.Listen, "port": ib.Port,
			"stream_settings": ib.StreamSettings, "settings": ib.Settings,
		}).Error; err != nil {
			return err
		}
		disabled, err := disableInboundHosts(db, ib.Id)
		if err != nil {
			return err
		}
		sr.DisabledHostIDs = disabled
		if (row.HostAddress != "" || inside[row.InboundID]) && (move || hideNaiveID(req.HideNaive, row.InboundID)) {
			h := gatewayHost(ib.Id, row, inside[row.InboundID], siteHost, insidePath[row.InboundID])
			if err := db.Create(&h).Error; err != nil {
				return err
			}
			sr.HostID = h.Id
		}
		snapRows = append(snapRows, *sr)
	}
	cfg.PublicHost = host
	cfg.BindIP = bindIP
	cfg.Snapshot = snapRows
	cfg.Routes = tunnel.RoutesFromPreview(rows, routeSelected)
	cfg.Fallback = tunnel.CoverFallback(rows, selected)
	cfg.Unified = true
	cfg.Enabled = true
	cfg.UFW = req.UFW
	if req.UFW {
		cfg.UFWWasActive = tunnel.UFWActive()
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	gw.Settings = string(body)
	gw.Enable = true
	gw.Listen = bindIP
	if gw.Port <= 0 {
		gw.Port = 443
	}
	if err := db.Model(gw).Select("settings", "enable", "port", "listen").Updates(gw).Error; err != nil {
		return err
	}
	s.ensureGatewayRuntime(gw, others)
	if req.UFW {
		web, sub := gatewayExtraPorts()
		if err := tunnel.ApplyUFW(tunnel.GatewayUFWAllow(web, sub, tunnel.SSHDPort(), rows, selected, req.HidePanel)); err != nil {
			return common.NewError("gateway applied, ufw:", err)
		}
	}
	return nil
}

func (s *InboundService) GatewayRevert(gatewayID int) error {
	gw, others, err := s.gatewayAndOthers(gatewayID)
	if err != nil {
		return err
	}
	cfg, _ := tunnel.GatewayConfigFromInbound(gw)
	if !cfg.Applied() {
		return common.NewError("gateway: nothing to revert")
	}
	if cfg.UFW {
		_ = tunnel.RevertUFW(cfg.UFWWasActive)
	}
	tunnel.GetManager().Remove(tunnel.GatewayKey(gw.Id))
	db := database.GetDB()
	byID := map[int]*model.Inbound{}
	for _, o := range others {
		byID[o.Id] = o
	}
	for _, sr := range cfg.Snapshot {
		if sr.HostID > 0 {
			_ = db.Delete(&model.Host{}, sr.HostID).Error
		}
		ib := byID[sr.InboundID]
		if ib == nil {
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Updates(tunnel.RevertUpdates(sr)).Error; err != nil {
			return err
		}
		if len(sr.DisabledHostIDs) > 0 {
			_ = db.Model(&model.Host{}).Where("id IN ?", sr.DisabledHostIDs).Updates(map[string]any{"is_disabled": false}).Error
		}
	}
	cfg.Snapshot = nil
	cfg.Routes = nil
	cfg.Fallback = ""
	cfg.Unified = false
	cfg.BindIP = ""
	cfg.UFW = false
	cfg.UFWWasActive = false
	cfg.HidePanel = false
	cfg.PanelRoutes = nil
	cfg.Enabled = false
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	gw.Settings = string(body)
	gw.Enable = false
	gw.Listen = ""
	if err := db.Model(gw).Select("settings", "enable", "listen").Updates(gw).Error; err != nil {
		return err
	}
	s.ensureGatewayRuntime(gw, others)
	s.sweepOrphanGatewayHosts()
	_ = (&XrayService{inboundService: *s}).RestartXray(true)
	return nil
}

func gatewayPanelCertPair() (cert, key string) {
	st := SettingService{}
	cert, _ = st.GetCertFile()
	key, _ = st.GetKeyFile()
	return
}

func loopbackDest(tls bool, port int) string {
	if port <= 0 {
		return ""
	}
	if tls {
		return fmt.Sprintf("https://127.0.0.1:%d", port)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func panelCoverRoutes() []tunnel.CoverRoute {
	st := SettingService{}
	var out []tunnel.CoverRoute
	add := func(path, dest string) {
		path = strings.TrimSpace(path)
		if path == "" || path == "/" || dest == "" {
			return
		}
		out = append(out, tunnel.CoverRoute{Path: strings.TrimSuffix(path, "/"), Dest: dest})
	}
	base, _ := st.GetBasePath()
	web, _ := st.GetPort()
	cert, _ := st.GetCertFile()
	add(base, loopbackDest(strings.TrimSpace(cert) != "", web))
	if on, err := st.GetSubEnable(); err == nil && on {
		subPort, _ := st.GetSubPort()
		subCert, _ := st.GetSubCertFile()
		dest := loopbackDest(strings.TrimSpace(subCert) != "", subPort)
		p, _ := st.GetSubPath()
		add(p, dest)
		if jsonOn, _ := st.GetSubJsonEnable(); jsonOn {
			jp, _ := st.GetSubJsonPath()
			add(jp, dest)
		}
		if clashOn, _ := st.GetSubClashEnable(); clashOn {
			cp, _ := st.GetSubClashPath()
			add(cp, dest)
		}
		if awgOn, _ := st.GetSubAwgEnable(); awgOn {
			ap, _ := st.GetSubAwgPath()
			add(ap, dest)
		}
	}
	return out
}

func gatewayExtraPorts() (web, sub int) {
	st := SettingService{}
	web, _ = st.GetPort()
	if on, err := st.GetSubEnable(); err == nil && on {
		sub, _ = st.GetSubPort()
	}
	return
}

func ufwDefaultSelected(rows []tunnel.PreviewRow) map[int]bool {
	m := map[int]bool{}
	for _, r := range rows {
		if r.Class != tunnel.ClassSkip {
			m[r.InboundID] = true
		}
	}
	return m
}

const gatewayHostRemark = "gateway"

func panelPublicHost() string {
	st := SettingService{}
	if d, err := st.GetSubDomain(); err == nil && strings.TrimSpace(d) != "" {
		return d
	}
	d, _ := st.GetWebDomain()
	return d
}

func inboundLabel(ib *model.Inbound) string {
	if ib == nil {
		return ""
	}
	if s := strings.TrimSpace(ib.Remark); s != "" {
		return s
	}
	return fmt.Sprintf("%s #%d", ib.Protocol, ib.Id)
}

func coverSelected(rows []tunnel.PreviewRow, selected map[int]bool) bool {
	for _, row := range rows {
		if selected[row.InboundID] && (row.Protocol == string(model.Cover) || row.Protocol == string(model.Tproxy)) {
			return true
		}
	}
	return false
}

// BindAppliedRealityDest used to rewrite REALITY dest to the Cover loopback
// on every reconcile. That stole the Microsoft handshake and survived Revert
// when the snapshot was taken after the write. It is a no-op: dest stays
// whatever the operator set.
func (s *InboundService) BindAppliedRealityDest() bool {
	return false
}

func hideNaiveID(ids []int, id int) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func disableInboundHosts(db *gorm.DB, inboundID int) ([]int, error) {
	var hosts []model.Host
	if err := db.Where("inbound_id = ? AND is_disabled = ?", inboundID, false).Find(&hosts).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(hosts))
	for _, h := range hosts {
		if err := db.Model(&model.Host{}).Where("id = ?", h.Id).Updates(map[string]any{"is_disabled": true}).Error; err != nil {
			return nil, err
		}
		ids = append(ids, h.Id)
	}
	return ids, nil
}

func gatewayHost(id int, row tunnel.PreviewRow, inside bool, site, path string) model.Host {
	h := model.Host{
		GroupId:   random.NumLower(16),
		InboundId: id,
		Remark:    gatewayHostRemark,
		Address:   row.HostAddress,
		Port:      row.HostPort,
		Security:  "same",
	}
	if h.Port <= 0 {
		h.Port = 443
	}
	if !inside {
		return h
	}
	h.Address = site
	h.Port = 443
	h.Security = "tls"
	h.Sni = site
	h.Path = path
	return h
}

// applyInside parks WS/HTTPUpgrade behind the Cover path and records the path
// used for the subscription host. Cover settings are mutated in memory; the
// caller saves them with the rest of the snapshot.
func applyInside(byID map[int]*model.Inbound, rows []tunnel.PreviewRow, inside map[int]bool, paths map[string]string, site string) (map[int]string, error) {
	out := map[int]string{}
	if len(inside) == 0 {
		return out, nil
	}
	var cover *model.Inbound
	for _, row := range rows {
		if row.Protocol == string(model.Cover) {
			if ib := byID[row.InboundID]; ib != nil {
				cover = ib
				break
			}
		}
	}
	if cover == nil {
		return nil, common.NewError("gateway: pick Cover to hide a protocol inside the site")
	}
	for id := range inside {
		ib := byID[id]
		row := previewRow(rows, id)
		if ib == nil || row.InboundID == 0 || !row.CanInside {
			return nil, common.NewErrorf("gateway: inbound %d cannot hide inside the site", id)
		}
		path := ""
		if paths != nil {
			path = strings.TrimSpace(paths[strconv.Itoa(id)])
		}
		if path == "" || path == "/" {
			path = "/" + random.NumLower(12)
		}
		dest := fmt.Sprintf("127.0.0.1:%d", row.NewPort)
		next, err := tunnel.AppendCoverRoute(cover.Settings, path, dest)
		if err != nil {
			return nil, err
		}
		cover.Settings = next
		ib.StreamSettings = tunnel.SetPlainPath(ib.StreamSettings, path)
		out[id] = path
	}
	return out, nil
}

func previewRow(rows []tunnel.PreviewRow, id int) tunnel.PreviewRow {
	for _, row := range rows {
		if row.InboundID == id {
			return row
		}
	}
	return tunnel.PreviewRow{}
}

func hideNaiveOnSite(byID map[int]*model.Inbound, rows []tunnel.PreviewRow, selected map[int]bool, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	host, cover := selectedSiteHost(rows, selected)
	if host == "" {
		return common.NewError("gateway: pick Cover or WEB proxy to hide Naive behind 443")
	}
	db := database.GetDB()
	for _, id := range ids {
		ib := byID[id]
		if ib == nil || ib.Protocol != model.Naive {
			continue
		}
		tunnel.HideNaiveOnSite(ib, host, cover)
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", ib.Settings).Error; err != nil {
			return err
		}
	}
	return nil
}

func selectedSiteHost(rows []tunnel.PreviewRow, selected map[int]bool) (host string, cover bool) {
	var tproxy string
	for _, r := range rows {
		if selected != nil && !selected[r.InboundID] {
			continue
		}
		switch r.Protocol {
		case string(model.Cover):
			if r.SNI != "" {
				return r.SNI, true
			}
		case string(model.Tproxy):
			if r.SNI != "" && tproxy == "" {
				tproxy = r.SNI
			}
		}
	}
	return tproxy, false
}

func applyNaiveMoves(byID map[int]*model.Inbound, moves []tunnel.NaiveMove) error {
	if len(moves) == 0 {
		return nil
	}
	cert, key := gatewayPanelCertPair()
	db := database.GetDB()
	for _, m := range moves {
		ib := byID[m.ID]
		if ib == nil {
			continue
		}
		ncfg, _ := tunnel.ConfigFromInbound(ib)
		if ncfg.UseAcme {
			if err := tunnel.ValidateCertFiles(cert, key, ncfg.Domain); err != nil {
				return common.NewErrorf("gateway: %q uses Auto TLS which needs port 443 — set cert/key (panel cert doesn't cover %q)", inboundLabel(ib), ncfg.Domain)
			}
			tunnel.SetNaiveCert(ib, cert, key)
		}
		m.Apply(ib)
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Updates(map[string]any{
			"listen":   ib.Listen,
			"port":     ib.Port,
			"settings": ib.Settings,
		}).Error; err != nil {
			return err
		}
		_ = db.Where("inbound_id = ? AND remark = ?", ib.Id, gatewayHostRemark).Delete(&model.Host{}).Error
	}
	return nil
}

// ReleaseMaskedNaive undoes a previous Apply that hid naive on loopback.
// Called from reconcile so an update fixes it without a console.
func (s *InboundService) ReleaseMaskedNaive() {
	all, err := s.GetAllInbounds()
	if err != nil {
		return
	}
	db := database.GetDB()
	for _, gw := range all {
		if gw == nil || gw.Protocol != model.Gateway {
			continue
		}
		cfg, ok := tunnel.GatewayConfigFromInbound(gw)
		if !ok || !cfg.Applied() {
			continue
		}
		next, moves, changed := tunnel.ReleaseMaskedNaive(cfg, all, gw.Port)
		if !changed {
			continue
		}
		if err := applyNaiveMoves(indexInbounds(all), moves); err != nil {
			logger.Warning("gateway: release naive:", err)
			continue
		}
		if next.UFW {
			for _, m := range moves {
				if err := tunnel.AllowUFW(m.Port); err != nil {
					logger.Warning("gateway: ufw allow naive:", err)
				}
			}
		}
		body, err := json.Marshal(next)
		if err != nil {
			continue
		}
		gw.Settings = string(body)
		if err := db.Model(gw).Select("settings").Updates(gw).Error; err != nil {
			logger.Warning("gateway: save after naive release:", err)
		}
	}
}

func indexInbounds(all []*model.Inbound) map[int]*model.Inbound {
	byID := map[int]*model.Inbound{}
	for _, o := range all {
		if o != nil {
			byID[o.Id] = o
		}
	}
	return byID
}

func (s *InboundService) sweepOrphanGatewayHosts() {
	all, err := s.GetAllInbounds()
	if err != nil {
		return
	}
	for _, ib := range all {
		cfg, ok := tunnel.GatewayConfigFromInbound(ib)
		if ok && cfg.Applied() {
			return
		}
	}
	_ = database.GetDB().Where("remark = ?", gatewayHostRemark).Delete(&model.Host{}).Error
}

func (s *InboundService) EnsureGatewayInbound(userId int) (*model.Inbound, error) {
	all, err := s.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, ib := range all {
		if ib != nil && ib.Protocol == model.Gateway && ib.NodeID == nil {
			return ib, nil
		}
	}
	ib := &model.Inbound{
		UserId:         userId,
		Remark:         "SNI gateway",
		Enable:         false,
		Port:           443,
		Protocol:       model.Gateway,
		Listen:         "",
		Settings:       "{}",
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	created, _, err := s.AddInbound(ib)
	return created, err
}

func (s *InboundService) DisableGatewayMask(id int) (bool, error) {
	gw, err := s.GetInbound(id)
	if err != nil {
		return false, err
	}
	if gw.Protocol != model.Gateway {
		return false, nil
	}
	cfg, ok := tunnel.GatewayConfigFromInbound(gw)
	if !ok || !cfg.Applied() {
		return false, nil
	}
	return true, s.GatewayRevert(id)
}

func (s *InboundService) gatewayAndOthers(id int) (*model.Inbound, []*model.Inbound, error) {
	all, err := s.GetAllInbounds()
	if err != nil {
		return nil, nil, err
	}
	var gw *model.Inbound
	var others []*model.Inbound
	for _, ib := range all {
		if ib == nil || ib.NodeID != nil {
			continue
		}
		if ib.Id == id {
			gw = ib
			continue
		}
		others = append(others, ib)
	}
	if gw == nil || gw.Protocol != model.Gateway {
		return nil, nil, common.NewError("gateway inbound not found")
	}
	return gw, others, nil
}

func (s *InboundService) ensureGatewayRuntime(gw *model.Inbound, others []*model.Inbound) {
	secret, _ := (&SettingService{}).GetSecret()
	cert, key := gatewayPanelCertPair()
	if inst, ok := tunnel.GatewayInstanceFromInbound(gw, others, secret, cert, key); ok {
		_ = tunnel.GetManager().Ensure(inst)
	}
	(&TunnelService{inboundService: *s}).Reconcile()
}
