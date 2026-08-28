// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"io"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const coreUploadMax = 200 << 20

func saveCoreUpload(c *gin.Context, dst string) error {
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}
	if file.Size > coreUploadMax {
		return common.NewError("upload exceeds 200 MB")
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := dst + ".upload"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(src, coreUploadMax+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if n > coreUploadMax {
		_ = os.Remove(tmp)
		return common.NewError("upload exceeds 200 MB")
	}
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// TunnelController exposes the external tunnel sidecars (NaiveProxy caddy):
// config CRUD, lifecycle, logs, Caddyfile preview/validation and binary
// management. Routes live under /panel/api/tunnel/* and share the panel API
// auth + CSRF middleware.
//
// xrayService is held so mutating handlers can force-regenerate the Xray
// config when the NaiveProxy SOCKS egress bridge changes (routeThroughXray
// toggle / outbound / enable flip). The bridge lives only in the generated
// config — same reason AwgOutboundController force-restarts.
type TunnelController struct {
	svc         *service.TunnelService
	xrayService service.XrayService
}

// NewTunnelController creates a new TunnelController bound to the given gin
// group and registers its routes.
func NewTunnelController(g *gin.RouterGroup) *TunnelController {
	a := &TunnelController{svc: &service.TunnelService{}}
	a.initRouter(g)
	return a
}

// restartXrayIfNeeded regenerates Xray config when the tunnel SOCKS bridge
// must be added/moved/dropped. Force-restart (not the deferred flag) so the
// bridge is live before the caller returns.
func (a *TunnelController) restartXrayIfNeeded(need bool) {
	if !need {
		return
	}
	if err := a.xrayService.RestartXray(true); err != nil {
		logger.Warning("tunnel: restart xray after bridge change failed:", err)
	}
}

func (a *TunnelController) initRouter(g *gin.RouterGroup) {
	naive := g.Group("/naive")
	naive.GET("/status", a.status)
	naive.GET("/config", a.getConfig)
	naive.POST("/config", a.saveConfig)
	naive.POST("/start", a.start)
	naive.POST("/stop", a.stop)
	naive.POST("/restart", a.restart)
	naive.GET("/logs", a.logs)
	naive.POST("/preview", a.preview)
	naive.POST("/validate", a.validate)
	naive.POST("/upload", a.uploadBinary)
	naive.POST("/download", a.downloadBinary)
	naive.POST("/deleteBinary", a.deleteBinary)

	olcrtc := g.Group("/olcrtc")
	olcrtc.GET("/status", a.olcrtcStatus)
	olcrtc.GET("/config", a.olcrtcGetConfig)
	olcrtc.POST("/config", a.olcrtcSaveConfig)
	olcrtc.POST("/start", a.olcrtcStart)
	olcrtc.POST("/stop", a.olcrtcStop)
	olcrtc.POST("/restart", a.olcrtcRestart)
	olcrtc.GET("/logs", a.olcrtcLogs)
	olcrtc.POST("/preview", a.olcrtcPreview)
	olcrtc.POST("/upload", a.olcrtcUploadBinary)
	olcrtc.POST("/download", a.olcrtcDownloadBinary)
	olcrtc.POST("/deleteBinary", a.olcrtcDeleteBinary)

	qwdtt := g.Group("/qwdtt")
	qwdtt.GET("/status", a.qwdttStatus)
	qwdtt.GET("/config", a.qwdttGetConfig)
	qwdtt.POST("/config", a.qwdttSaveConfig)
	qwdtt.POST("/start", a.qwdttStart)
	qwdtt.POST("/stop", a.qwdttStop)
	qwdtt.POST("/restart", a.qwdttRestart)
	qwdtt.GET("/logs", a.qwdttLogs)
	qwdtt.POST("/upload", a.qwdttUploadBinary)
	qwdtt.POST("/download", a.qwdttDownloadBinary)
	qwdtt.POST("/deleteBinary", a.qwdttDeleteBinary)

	// mieru is inbound-only (no legacy config/lifecycle): status, logs and
	// binary management for the Settings → Cores page.
	mieru := g.Group("/mieru")
	mieru.GET("/status", a.mieruStatus)
	mieru.GET("/logs", a.mieruLogs)
	mieru.POST("/upload", a.mieruUploadBinary)
	mieru.POST("/download", a.mieruDownloadBinary)
	mieru.POST("/deleteBinary", a.mieruDeleteBinary)

	trusttunnel := g.Group("/trusttunnel")
	trusttunnel.GET("/status", a.trustTunnelStatus)
	trusttunnel.GET("/logs", a.trustTunnelLogs)
	trusttunnel.POST("/upload", a.trustTunnelUploadBinary)
	trusttunnel.POST("/download", a.trustTunnelDownloadBinary)
	trusttunnel.POST("/deleteBinary", a.trustTunnelDeleteBinary)

	// anytls is inbound-only (no legacy config/lifecycle): status, logs and
	// binary management for the Settings → Cores page.
	anytls := g.Group("/anytls")
	anytls.GET("/status", a.anytlsStatus)
	anytls.GET("/logs", a.anytlsLogs)
	anytls.POST("/upload", a.anytlsUploadBinary)
	anytls.POST("/download", a.anytlsDownloadBinary)
	anytls.POST("/deleteBinary", a.anytlsDeleteBinary)
}

func (a *TunnelController) status(c *gin.Context) {
	st, err := a.svc.NaiveStatus()
	if err != nil {
		jsonMsg(c, "tunnel: status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) getConfig(c *gin.Context) {
	cfg, err := a.svc.LoadNaiveConfig()
	if err != nil {
		jsonMsg(c, "tunnel: config load failed", err)
		return
	}
	jsonObj(c, cfg, nil)
}

func (a *TunnelController) saveConfig(c *gin.Context) {
	cfg := tunnel.DefaultNaiveConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid config body", err)
		return
	}
	needRestart, err := a.svc.SaveNaiveConfig(cfg)
	if err != nil {
		jsonMsg(c, "tunnel: config save failed", err)
		return
	}
	a.restartXrayIfNeeded(needRestart)
	st, err := a.svc.NaiveStatus()
	if err != nil {
		jsonMsg(c, "tunnel: status after save failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) start(c *gin.Context) {
	needRestart, err := a.svc.StartNaive()
	if err == nil {
		a.restartXrayIfNeeded(needRestart)
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.started"), err)
}

func (a *TunnelController) stop(c *gin.Context) {
	needRestart, err := a.svc.StopNaive()
	if err == nil {
		a.restartXrayIfNeeded(needRestart)
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.stopped"), err)
}

func (a *TunnelController) restart(c *gin.Context) {
	needRestart, err := a.svc.RestartNaive()
	if err == nil {
		a.restartXrayIfNeeded(needRestart)
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.restarted"), err)
}

func (a *TunnelController) logs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.NaiveLogs(lines), nil)
}

func (a *TunnelController) preview(c *gin.Context) {
	cfg := tunnel.DefaultNaiveConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid preview body", err)
		return
	}
	text, err := a.svc.PreviewNaive(cfg)
	if err != nil {
		jsonMsg(c, "tunnel: preview failed", err)
		return
	}
	jsonObj(c, gin.H{"caddyfile": text}, nil)
}

func (a *TunnelController) validate(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid validate body", err)
		return
	}
	if err := a.svc.ValidateCaddyfile(body.Text); err != nil {
		jsonMsg(c, "tunnel: Caddyfile invalid", err)
		return
	}
	jsonObj(c, gin.H{"valid": true}, nil)
}

// uploadBinary replaces the core binary on disk (multipart field "file").
// The route is exempt from the global body limit in web.go because caddy
// builds are ~50 MB.
func (a *TunnelController) uploadBinary(c *gin.Context) {
	dst := tunnel.Naive.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		logger.Warning("tunnel: save uploaded binary failed:", err)
		jsonMsg(c, "tunnel: upload failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.uploaded"), nil)
}

// tunnelDownloadRequest is the body of every core binary download endpoint.
// SHA256 is optional: when present the service refuses to install a download
// whose digest does not match, which is the only defence against a mirror or
// a path that hands back something other than the release asset.
type tunnelDownloadRequest struct {
	URL    string `json:"url" example:"https://github.com/AlexeyLCP/lucx-ui/releases/download/v3.6.0-lucx.124/caddy-naive-linux-amd64"`
	SHA256 string `json:"sha256,omitempty" example:"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"`
}

func (a *TunnelController) downloadBinary(c *gin.Context) {
	var body tunnelDownloadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid download body", err)
		return
	}
	if err := a.svc.DownloadBinary(body.URL, body.SHA256); err != nil {
		jsonMsg(c, "tunnel: download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.downloaded"), nil)
}

func (a *TunnelController) deleteBinary(c *gin.Context) {
	if err := a.svc.DeleteBinary(); err != nil {
		jsonMsg(c, "tunnel: binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.deleted"), nil)
}

// --- olcRTC ---------------------------------------------------------------

func (a *TunnelController) olcrtcStatus(c *gin.Context) {
	st, err := a.svc.OlcrtcStatus()
	if err != nil {
		jsonMsg(c, "tunnel: olcrtc status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) olcrtcGetConfig(c *gin.Context) {
	cfg, err := a.svc.LoadOlcrtcConfig()
	if err != nil {
		jsonMsg(c, "tunnel: olcrtc config load failed", err)
		return
	}
	jsonObj(c, cfg, nil)
}

func (a *TunnelController) olcrtcSaveConfig(c *gin.Context) {
	cfg := tunnel.DefaultOlcrtcConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid olcrtc config body", err)
		return
	}
	if err := a.svc.SaveOlcrtcConfig(cfg); err != nil {
		jsonMsg(c, "tunnel: olcrtc config save failed", err)
		return
	}
	st, err := a.svc.OlcrtcStatus()
	if err != nil {
		jsonMsg(c, "tunnel: olcrtc status after save failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) olcrtcStart(c *gin.Context) {
	err := a.svc.StartOlcrtc()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.olcrtc.toasts.started"), err)
}

func (a *TunnelController) olcrtcStop(c *gin.Context) {
	err := a.svc.StopOlcrtc()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.olcrtc.toasts.stopped"), err)
}

func (a *TunnelController) olcrtcRestart(c *gin.Context) {
	err := a.svc.RestartOlcrtc()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.olcrtc.toasts.restarted"), err)
}

func (a *TunnelController) olcrtcLogs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.OlcrtcLogs(lines), nil)
}

func (a *TunnelController) olcrtcPreview(c *gin.Context) {
	cfg := tunnel.DefaultOlcrtcConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid olcrtc preview body", err)
		return
	}
	text, err := a.svc.PreviewOlcrtc(cfg)
	if err != nil {
		jsonMsg(c, "tunnel: olcrtc preview failed", err)
		return
	}
	jsonObj(c, gin.H{"yaml": text}, nil)
}

func (a *TunnelController) olcrtcUploadBinary(c *gin.Context) {
	dst := tunnel.Olcrtc.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		logger.Warning("tunnel: save uploaded olcrtc binary failed:", err)
		jsonMsg(c, "tunnel: olcrtc upload failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.olcrtc.toasts.uploaded"), nil)
}

func (a *TunnelController) olcrtcDownloadBinary(c *gin.Context) {
	var body tunnelDownloadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid olcrtc download body", err)
		return
	}
	if err := a.svc.DownloadOlcrtcBinary(body.URL, body.SHA256); err != nil {
		jsonMsg(c, "tunnel: olcrtc download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.olcrtc.toasts.downloaded"), nil)
}

func (a *TunnelController) olcrtcDeleteBinary(c *gin.Context) {
	if err := a.svc.DeleteOlcrtcBinary(); err != nil {
		jsonMsg(c, "tunnel: olcrtc binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.olcrtc.toasts.deleted"), nil)
}

// --- qWDTT ----------------------------------------------------------------

func (a *TunnelController) qwdttStatus(c *gin.Context) {
	st, err := a.svc.QwdttStatus()
	if err != nil {
		jsonMsg(c, "tunnel: qwdtt status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) qwdttGetConfig(c *gin.Context) {
	cfg, err := a.svc.LoadQwdttConfig()
	if err != nil {
		jsonMsg(c, "tunnel: qwdtt config load failed", err)
		return
	}
	jsonObj(c, cfg, nil)
}

func (a *TunnelController) qwdttSaveConfig(c *gin.Context) {
	cfg := tunnel.DefaultQwdttConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid qwdtt config body", err)
		return
	}
	if err := a.svc.SaveQwdttConfig(cfg); err != nil {
		jsonMsg(c, "tunnel: qwdtt config save failed", err)
		return
	}
	st, err := a.svc.QwdttStatus()
	if err != nil {
		jsonMsg(c, "tunnel: qwdtt status after save failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) qwdttStart(c *gin.Context) {
	err := a.svc.StartQwdtt()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.qwdtt.toasts.started"), err)
}

func (a *TunnelController) qwdttStop(c *gin.Context) {
	err := a.svc.StopQwdtt()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.qwdtt.toasts.stopped"), err)
}

func (a *TunnelController) qwdttRestart(c *gin.Context) {
	err := a.svc.RestartQwdtt()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.qwdtt.toasts.restarted"), err)
}

func (a *TunnelController) qwdttLogs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.QwdttLogs(lines), nil)
}

func (a *TunnelController) qwdttUploadBinary(c *gin.Context) {
	dst := tunnel.Qwdtt.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		logger.Warning("tunnel: save uploaded qwdtt binary failed:", err)
		jsonMsg(c, "tunnel: qwdtt upload failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.qwdtt.toasts.uploaded"), nil)
}

func (a *TunnelController) qwdttDownloadBinary(c *gin.Context) {
	var body tunnelDownloadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid qwdtt download body", err)
		return
	}
	if err := a.svc.DownloadQwdttBinary(body.URL, body.SHA256); err != nil {
		jsonMsg(c, "tunnel: qwdtt download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.qwdtt.toasts.downloaded"), nil)
}

func (a *TunnelController) qwdttDeleteBinary(c *gin.Context) {
	if err := a.svc.DeleteQwdttBinary(); err != nil {
		jsonMsg(c, "tunnel: qwdtt binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.qwdtt.toasts.deleted"), nil)
}

// --- mieru (inbound-only: status/logs/binary for the Cores page) ----------

func (a *TunnelController) mieruStatus(c *gin.Context) {
	st, err := a.svc.MieruStatus()
	if err != nil {
		jsonMsg(c, "tunnel: mieru status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) mieruLogs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.MieruLogs(lines), nil)
}

func (a *TunnelController) mieruUploadBinary(c *gin.Context) {
	dst := tunnel.Mieru.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		logger.Warning("tunnel: save uploaded mieru binary failed:", err)
		jsonMsg(c, "tunnel: mieru upload failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.mieru.toasts.uploaded"), nil)
}

func (a *TunnelController) mieruDownloadBinary(c *gin.Context) {
	var body tunnelDownloadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid mieru download body", err)
		return
	}
	if err := a.svc.DownloadMieruBinary(body.URL, body.SHA256); err != nil {
		jsonMsg(c, "tunnel: mieru download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.mieru.toasts.downloaded"), nil)
}

func (a *TunnelController) mieruDeleteBinary(c *gin.Context) {
	if err := a.svc.DeleteMieruBinary(); err != nil {
		jsonMsg(c, "tunnel: mieru binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.mieru.toasts.deleted"), nil)
}

// --- TrustTunnel (inbound-only: status/logs/binary for the Cores page) ----

func (a *TunnelController) trustTunnelStatus(c *gin.Context) {
	st, err := a.svc.TrustTunnelStatus()
	if err != nil {
		jsonMsg(c, "tunnel: trusttunnel status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) trustTunnelLogs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.TrustTunnelLogs(lines), nil)
}

func (a *TunnelController) trustTunnelUploadBinary(c *gin.Context) {
	dst := tunnel.TrustTunnel.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		logger.Warning("tunnel: save uploaded trusttunnel binary failed:", err)
		jsonMsg(c, "tunnel: trusttunnel upload failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.trusttunnel.toasts.uploaded"), nil)
}

func (a *TunnelController) trustTunnelDownloadBinary(c *gin.Context) {
	var body tunnelDownloadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid trusttunnel download body", err)
		return
	}
	if err := a.svc.DownloadTrustTunnelBinary(body.URL, body.SHA256); err != nil {
		jsonMsg(c, "tunnel: trusttunnel download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.trusttunnel.toasts.downloaded"), nil)
}

func (a *TunnelController) trustTunnelDeleteBinary(c *gin.Context) {
	if err := a.svc.DeleteTrustTunnelBinary(); err != nil {
		jsonMsg(c, "tunnel: trusttunnel binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.trusttunnel.toasts.deleted"), nil)
}

// --- AnyTLS (inbound-only: status/logs/binary for the Cores page) ----------

func (a *TunnelController) anytlsStatus(c *gin.Context) {
	st, err := a.svc.AnytlsStatus()
	if err != nil {
		jsonMsg(c, "tunnel: anytls status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) anytlsLogs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.AnytlsLogs(lines), nil)
}

func (a *TunnelController) anytlsUploadBinary(c *gin.Context) {
	dst := tunnel.Anytls.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		logger.Warning("tunnel: save uploaded anytls binary failed:", err)
		jsonMsg(c, "tunnel: anytls upload failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.anytls.toasts.uploaded"), nil)
}

func (a *TunnelController) anytlsDownloadBinary(c *gin.Context) {
	var body tunnelDownloadRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid anytls download body", err)
		return
	}
	if err := a.svc.DownloadAnytlsBinary(body.URL, body.SHA256); err != nil {
		jsonMsg(c, "tunnel: anytls download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.anytls.toasts.downloaded"), nil)
}

func (a *TunnelController) anytlsDeleteBinary(c *gin.Context) {
	if err := a.svc.DeleteAnytlsBinary(); err != nil {
		jsonMsg(c, "tunnel: anytls binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.anytls.toasts.deleted"), nil)
}
