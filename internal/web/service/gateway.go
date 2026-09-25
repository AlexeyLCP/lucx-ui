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
)

type GatewayApplyRequest struct {
	Selected   []int             `json:"selected"`
	Steal      []int             `json:"steal"`
	PublicHost string            `json:"publicHost"`
	UFW        bool              `json:"ufw"`
	HidePanel  bool              `json:"hidePanel"`
	HideNaive  []int             `json:"hideNaive"`
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
	steal := map[int]bool{}
	for _, id := range req.Steal {
		steal[id] = true
	}
	if len(selected) == 0 {
		return common.NewError("gateway: nothing selected")
	}
	db := database.GetDB()
	for idStr, sni := range req.SNI {
		id, err := strconv.Atoi(idStr)
		if err != nil || !selected[id] {
			continue
		}
		ib := byID[id]
		if ib == nil {
			continue
		}
		tunnel.SetInboundSNI(ib, sni)
		if err := db.Model(ib).Select("settings", "stream_settings").Updates(ib).Error; err != nil {
			return err
		}
	}
	if err := applyNaiveMoves(byID, tunnel.PlanNaivePublic(gw.Port, others)); err != nil {
		return err
	}
	rows := tunnel.BuildPreview(gw.Port, host, others, bindIP)
	if err := hideNaiveOnSite(byID, rows, selected, req.HideNaive); err != nil {
		return err
	}
	if c := tunnel.SNIClash(rows, selected); c != "" {
		return common.NewError("gateway: duplicate SNI", c)
	}
	for _, o := range others {
		// Anything left public on TCP :443 wins the bind race with the
		// gateway — one of them then fails to listen.
		if o == nil || !o.Enable || selected[o.Id] || o.Port != gw.Port ||
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
	var snap []tunnel.GatewaySnapshotRow
	for _, row := range rows {
		if !selected[row.InboundID] || row.Class == tunnel.ClassSkip {
			continue
		}
		ib := byID[row.InboundID]
		if ib == nil {
			continue
		}
		sr := tunnel.GatewaySnapshotRow{InboundID: ib.Id, Listen: ib.Listen, Port: ib.Port, StreamSettings: ib.StreamSettings}
		ib.Listen = row.NewListen
		ib.Port = row.NewPort
		// Cover selected → Reality dest is the local decoy, same as the nginx
		// script. The checkbox is not required.
		if row.StealDest != "" && (steal[row.InboundID] || coverSelected(rows, selected)) {
			ib.StreamSettings = tunnel.SetRealityDest(ib.StreamSettings, row.StealDest)
		}
		if !row.NoProxy && tunnel.XrayAcceptsProxyProtocol(ib.Protocol) {
			ib.StreamSettings = tunnel.SetAcceptProxyProtocol(ib.StreamSettings, true)
		}
		if err := db.Model(ib).Select("listen", "port", "stream_settings").Updates(ib).Error; err != nil {
			return err
		}
		if row.HostAddress != "" && row.Class != tunnel.ClassSkip {
			h := model.Host{
				GroupId:   random.NumLower(16),
				InboundId: ib.Id,
				Remark:    gatewayHostRemark,
				Address:   row.HostAddress,
				Port:      row.HostPort,
				Security:  "same",
			}
			if err := db.Create(&h).Error; err != nil {
				return err
			}
			sr.HostID = h.Id
		}
		snap = append(snap, sr)
	}
	cfg.PublicHost = host
	cfg.BindIP = bindIP
	cfg.Snapshot = snap
	cfg.Routes = tunnel.RoutesFromPreview(rows, selected)
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
		ib.Listen = sr.Listen
		ib.Port = sr.Port
		if sr.StreamSettings != "" {
			ib.StreamSettings = sr.StreamSettings
		}
		_ = db.Model(ib).Select("listen", "port", "stream_settings").Updates(ib).Error
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

// BindAppliedRealityDest points masked REALITY dest at the Cover loopback,
// matching the nginx script's fallback. No client-link change. Returns true
// if Xray must reload.
func (s *InboundService) BindAppliedRealityDest() bool {
	all, err := s.GetAllInbounds()
	if err != nil {
		return false
	}
	db := database.GetDB()
	changed := false
	for _, gw := range all {
		cfg, ok := tunnel.GatewayConfigFromInbound(gw)
		if !ok || !cfg.Applied() {
			continue
		}
		byID := indexInbounds(all)
		var coverPort, tproxyPort int
		for _, sr := range cfg.Snapshot {
			ib := byID[sr.InboundID]
			if ib == nil || ib.Port <= 0 {
				continue
			}
			if ib.Protocol == model.Cover {
				coverPort = ib.Port
			}
			if ib.Protocol == model.Tproxy && tproxyPort == 0 {
				tproxyPort = ib.Port
			}
		}
		if coverPort == 0 {
			coverPort = tproxyPort
		}
		if coverPort <= 0 {
			continue
		}
		dest := fmt.Sprintf("127.0.0.1:%d", coverPort)
		for _, sr := range cfg.Snapshot {
			ib := byID[sr.InboundID]
			if ib == nil {
				continue
			}
			if !tunnel.IsRealityStream(ib.StreamSettings) {
				continue
			}
			if tunnel.RealityDest(ib.StreamSettings) == dest {
				continue
			}
			ib.StreamSettings = tunnel.SetRealityDest(ib.StreamSettings, dest)
			if err := db.Model(ib).Select("stream_settings").Updates(ib).Error; err != nil {
				logger.Warning("gateway: reality dest:", err)
				continue
			}
			changed = true
		}
	}
	return changed
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
