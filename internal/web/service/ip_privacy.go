// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Two privacy controls apply to every client address the panel shows:
//
//   - the Xray log "Mask Address" option (quarter, half, full) masks the IPs
//     the panel displays the same way Xray masks its own logs;
//   - a client's "Never log IPs" flag keeps its addresses out of the panel
//     entirely: nothing is recorded, stored or shown for it.

const ipPrivacyCacheTTL = 10 * time.Second

var ipPrivacy = struct {
	sync.Mutex
	at       time.Time
	mask     string
	neverLog map[string]bool
}{}

func loadIPPrivacy() (string, map[string]bool) {
	ipPrivacy.Lock()
	defer ipPrivacy.Unlock()
	if time.Since(ipPrivacy.at) < ipPrivacyCacheTTL && ipPrivacy.neverLog != nil {
		return ipPrivacy.mask, ipPrivacy.neverLog
	}
	ipPrivacy.mask = xrayLogMaskAddress()
	neverLog := map[string]bool{}
	if db := database.GetDB(); db != nil {
		var emails []string
		if err := db.Model(&model.ClientRecord{}).Where("never_log_ips = ?", true).Pluck("email", &emails).Error; err == nil {
			for _, e := range emails {
				neverLog[e] = true
			}
		}
	}
	ipPrivacy.neverLog = neverLog
	ipPrivacy.at = time.Now()
	return ipPrivacy.mask, ipPrivacy.neverLog
}

// resetIPPrivacyCache makes the next lookup re-read the settings.
func resetIPPrivacyCache() {
	ipPrivacy.Lock()
	ipPrivacy.at = time.Time{}
	ipPrivacy.Unlock()
}

// xrayLogMaskAddress returns the "log.maskAddress" value of the Xray template
// ("quarter", "half", "full" or "").
func xrayLogMaskAddress() string {
	tmpl, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil || tmpl == "" {
		return ""
	}
	var cfg struct {
		Log struct {
			MaskAddress string `json:"maskAddress"`
		} `json:"log"`
	}
	if json.Unmarshal([]byte(tmpl), &cfg) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(cfg.Log.MaskAddress))
}

// ClientNeverLogsIPs reports whether email has "Never log IPs" set.
func ClientNeverLogsIPs(email string) bool {
	_, neverLog := loadIPPrivacy()
	return neverLog[email]
}

// NeverLogIPEmails returns the clients with "Never log IPs" set.
func NeverLogIPEmails() map[string]bool {
	_, neverLog := loadIPPrivacy()
	return neverLog
}

// subNeverLogsIPs reports whether any client on the subscription has "Never
// log IPs" set; a device's address belongs to all of them.
func subNeverLogsIPs(subID string) bool {
	if strings.TrimSpace(subID) == "" {
		return false
	}
	var n int64
	database.GetDB().Model(&model.ClientRecord{}).
		Where("sub_id = ? AND never_log_ips = ?", subID, true).Count(&n)
	return n > 0
}

// ForgetClientIPs drops every address stored for email: the live list, the
// IP-limit log and its devices' last IPs.
func ForgetClientIPs(email string) {
	email = strings.TrimSpace(email)
	if email == "" {
		return
	}
	liveIPs.Lock()
	delete(liveIPs.byEmail, email)
	liveIPs.Unlock()
	db := database.GetDB()
	if err := db.Where("client_email = ?", email).Delete(&model.InboundClientIps{}).Error; err != nil {
		logger.Warning("never log IPs: clear IP log of", email, "failed:", err)
	}
	var subID string
	db.Model(&model.ClientRecord{}).Where("email = ?", email).Pluck("sub_id", &subID)
	if subID = strings.TrimSpace(subID); subID != "" {
		if err := db.Model(&model.ClientHwid{}).Where("sub_id = ?", subID).UpdateColumn("last_ip", "").Error; err != nil {
			logger.Warning("never log IPs: clear device IPs of", email, "failed:", err)
		}
	}
}

// LabelClientIP returns ip as the panel may show it under the Xray "Mask
// Address" setting, with its provider. Quarter and half masking keep the
// country and network but drop the city; full masking shows nothing.
func LabelClientIP(ip string) (string, IPProvider) {
	if strings.TrimSpace(ip) == "" {
		return "", IPProvider{}
	}
	mask, _ := loadIPPrivacy()
	shown := MaskIP(ip, mask)
	if shown == "" {
		return "", IPProvider{}
	}
	info := LookupIPProvider(ip)
	if shown != ip {
		info.City = ""
	}
	return shown, info
}

// MaskIP masks ip like Xray's log maskAddress: "quarter" hides the last
// IPv4 octet (the last 80 bits of IPv6), "half" the last two octets (the
// last 96 bits), "full" everything (empty result).
func MaskIP(ip, mode string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	switch mode {
	case "full":
		return ""
	case "quarter", "half":
	default:
		return ip
	}
	if addr.Is4() {
		b := addr.As4()
		if mode == "quarter" {
			return fmt.Sprintf("%d.%d.%d.*", b[0], b[1], b[2])
		}
		return fmt.Sprintf("%d.%d.*.*", b[0], b[1])
	}
	bits := 48
	if mode == "half" {
		bits = 32
	}
	prefix, err := addr.Prefix(bits)
	if err != nil {
		return ""
	}
	return prefix.String()
}
