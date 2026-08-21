// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package model

// SidecarOutbound is a client-mode tunnel sidecar (Naive / mieru / TrustTunnel)
// that this panel dials as a SOCKS upstream. Mirrors AwgOutbound (Tag / Remark /
// Enable / Settings JSON) but the data plane is a userspace client process
// exposing SOCKS on loopback, injected as an Xray socks outbound — not a
// kernel iface + freedom/sockopt.interface.
type SidecarOutbound struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Protocol  string `json:"protocol" form:"protocol" gorm:"not null"`
	Tag       string `json:"tag" form:"tag" gorm:"uniqueIndex;not null"`
	Remark    string `json:"remark" form:"remark"`
	Enable    bool   `json:"enable" form:"enable" gorm:"default:true"`
	Settings  string `json:"settings" form:"settings"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
