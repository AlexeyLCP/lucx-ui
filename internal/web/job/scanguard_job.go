// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ScanguardJob keeps the scanner guard current: it re-applies rules that
// disappeared (reboot, iptables flush by fail2ban/docker) and refreshes the
// blocklists on the configured cadence (@every 30m tick, re-fetch at most
// once per service.ScanguardService interval).
type ScanguardJob struct {
	scanguardService *service.ScanguardService
}

// NewScanguardJob creates a new scanner guard reconcile job instance.
func NewScanguardJob() *ScanguardJob {
	return &ScanguardJob{scanguardService: &service.ScanguardService{}}
}

func (j *ScanguardJob) Run() {
	j.scanguardService.Reconcile()
}
