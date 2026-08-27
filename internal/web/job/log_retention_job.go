// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"os"
	"path/filepath"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// LogRetentionJob deletes log files in the panel log folder that are older
// than the operator-configured retention period (panel setting
// logRetentionDays, 0 = disabled). Active log files that the panel, Xray or
// fail2ban keep open (3xui.log, 3xipl*.log, the configured Xray access/error
// logs) are never deleted — only rotated segments, old crash reports and
// other stale files are removed.
type LogRetentionJob struct {
	settingService service.SettingService
}

// NewLogRetentionJob creates a new log retention job instance.
func NewLogRetentionJob() *LogRetentionJob {
	return new(LogRetentionJob)
}

// Run is the cron.Job interface method.
func (j *LogRetentionJob) Run() {
	days, err := j.settingService.GetLogRetentionDays()
	if err != nil {
		logger.Warning("[LUCX-LOGS] failed to read logRetentionDays:", err)
		return
	}
	if days <= 0 {
		return
	}
	protected := protectedLogPaths()
	removed, err := pruneLogFilesOlderThan(config.GetLogFolder(), days, time.Now(), protected)
	if err != nil {
		logger.Warning("[LUCX-LOGS] retention sweep failed:", err)
		return
	}
	if removed > 0 {
		logger.Infof("[LUCX-LOGS] deleted %d log file(s) older than %d day(s)", removed, days)
	}
}

// protectedLogPaths collects the files that must survive the sweep: the
// active panel log, the IP-limit chain (fail2ban tails 3xipl.log), and the
// currently configured Xray access/error logs.
func protectedLogPaths() map[string]bool {
	protected := map[string]bool{}
	protect := func(path string, err error) {
		if err != nil || path == "" || path == "none" {
			return
		}
		if abs, err := filepath.Abs(filepath.Clean(path)); err == nil {
			protected[abs] = true
		}
	}
	protect(filepath.Join(config.GetLogFolder(), "3xui.log"), nil)
	protect(xray.GetIPLimitLogPath(), nil)
	protect(xray.GetIPLimitBannedLogPath(), nil)
	protect(xray.GetIPLimitBannedPrevLogPath(), nil)
	protect(xray.GetAccessLogPath())
	protect(xray.GetErrorLogPath())
	return protected
}

// pruneLogFilesOlderThan removes regular files directly inside dir whose
// modification time is older than now minus days. Subdirectories and
// protected paths are skipped. Returns the number of deleted files.
func pruneLogFilesOlderThan(dir string, days int, now time.Time, protected map[string]bool) (int, error) {
	cutoff := now.AddDate(0, 0, -days)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if abs, err := filepath.Abs(filepath.Clean(path)); err == nil && protected[abs] {
			continue
		}
		if err := os.Remove(path); err != nil {
			logger.Warning("[LUCX-LOGS] failed to delete old log file:", path, "-", err)
			continue
		}
		removed++
	}
	return removed, nil
}
