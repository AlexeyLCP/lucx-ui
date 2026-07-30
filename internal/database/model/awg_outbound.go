// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package model

// AwgOutbound is a client-mode AmneziaWG connection to an upstream VPN server.
// Mirrors the Inbound shape (Tag/Remark/Enable/Settings JSON) but represents an
// egress: the panel brings up a kernel interface awgo-{Id} and injects a
// freedom outbound (bound to that interface) into the Xray config so routing
// rules can send traffic through the upstream VPN.
type AwgOutbound struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Tag       string `json:"tag" form:"tag" gorm:"uniqueIndex;not null"`
	Remark    string `json:"remark" form:"remark"`
	Enable    bool   `json:"enable" form:"enable" gorm:"default:true"`
	Settings  string `json:"settings" form:"settings"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
