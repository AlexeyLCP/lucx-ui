// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package nodetype

import (
	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type HelloObj struct {
	Version        string   `json:"version" example:"3.6.0-lucx.114"`
	Features       []string `json:"features" example:"[\"awg\",\"mtproto\",\"naive\",\"olcrtc\",\"qwdtt\",\"mieru\",\"trusttunnel\",\"cluster\"]"`
	AWGVersion     string   `json:"awgVersion" example:"v3.0.20260730"`
	MTProtoVersion string   `json:"mtprotoVersion" example:""`
	ModuleLoaded   bool     `json:"moduleLoaded" example:"true"`
	ModuleAwg3     bool     `json:"moduleAwg3" example:"true"`
	ModuleAwg31    bool     `json:"moduleAwg31" example:"true"`
}

func LocalHello() HelloObj {
	hs := awg.CollectHostStatus()
	features := make([]string, len(DefaultLucXFeatures))
	copy(features, DefaultLucXFeatures)
	return HelloObj{
		Version:      config.GetPanelVersion(),
		Features:     features,
		AWGVersion:   hs.Version,
		ModuleLoaded: hs.ModuleLoaded,
		ModuleAwg3:   hs.ModuleAwg3,
		ModuleAwg31:  hs.ModuleAwg31,
	}
}
