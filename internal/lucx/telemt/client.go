// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telemt

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type TelemtClient struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func GenerateSecret() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "ee" + hex.EncodeToString(b)
}

func GenerateProxyLink(host string, port int, secret string) string {
	return fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", host, port, secret)
}
