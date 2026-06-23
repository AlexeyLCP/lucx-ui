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
	"os/exec"
)

// awgQuick runs `awg-quick <verb> <conf>` and returns its combined output.
func awgQuick(verb, confPath string) ([]byte, error) {
	return exec.Command("awg-quick", verb, confPath).CombinedOutput()
}

// logAWG prints a formatted log line with [LUCX-AWG] prefix.
func logAWG(format string, args ...interface{}) {
	fmt.Printf("[LUCX-AWG] "+format+"\n", args...)
}

// getStringFromSettings extracts a string value from a JSON settings blob.
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

// getIntFromSettings extracts an int value from a JSON settings blob.
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

// pickFreeTunSubnet returns an available subnet for a TUN child interface.
func pickFreeTunSubnet(awgId int) string {
	return fmt.Sprintf("172.19.%d.%d/30", awgId/64, (awgId%64)*4)
}

// appendToFile appends a line to a file, creating it if needed.
func appendToFile(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}
