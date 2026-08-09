// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// TunnelJob reconciles the external tunnel sidecars (NaiveProxy caddy) with
// their stored configs: a crashed core is revived, a disabled one is kept
// down. Mirrors AwgJob/MtprotoJob.
type TunnelJob struct {
	tunnelService service.TunnelService
}

// NewTunnelJob creates a new tunnel reconcile job instance.
func NewTunnelJob() *TunnelJob {
	return new(TunnelJob)
}

// Run converges every tunnel core once.
func (j *TunnelJob) Run() {
	j.tunnelService.Reconcile()
}
