// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func logAWG(format string, args ...interface{}) {
	fmt.Printf("[LUCX-AWG] "+format+"\n", args...)
}

func getStringFromSettings(settings, key, defaultVal string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return defaultVal
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getIntFromSettings(settings, key string, defaultVal int) int {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return defaultVal
	}
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return defaultVal
}

func buildAWGConfig(awg *model.Inbound, params *AWGParams, data TemplateData, upPath, downPath string) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
ListenPort = %d
MTU = %d
Jc = %d
Jmin = %d
Jmax = %d
S1 = %d
S2 = %d
S3 = %d
S4 = %d
H1 = %s
H2 = %s
H3 = %s
H4 = %s
PostUp = %s
PostDown = %s
`, params.PrivateKey, data.AWGServerIP, awg.Port, params.MTU,
		params.Jc, params.Jmin, params.Jmax,
		params.S1, params.S2, params.S3, params.S4,
		params.H1, params.H2, params.H3, params.H4,
		upPath, downPath)
}

func pickFreeTunSubnet(awgId int) string {
	return fmt.Sprintf("172.19.%d.%d/30", awgId/64, (awgId%64)*4)
}

func appendToFile(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}
