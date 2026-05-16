// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database/model"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// SendAWGClientConfig generates a full AmneziaWG .conf file and sends it
// as a Telegram Document to the specified chat.
func SendAWGClientConfig(
	bot *telego.Bot,
	chatID int64,
	client model.Client,
	inbound model.Inbound,
	serverIP string,
	statsText string,
) error {
	configText := buildAWGConfigText(client, inbound, serverIP)
	fileName := fmt.Sprintf("%s_awg.conf", sanitizeFileName(client.Email))

	doc := tu.Document(
		tu.ID(chatID),
		tu.FileFromBytes([]byte(configText), fileName),
	)
	doc.Caption = statsText
	doc.ParseMode = "HTML"

	_, err := bot.SendDocument(context.Background(), doc)
	return err
}

// buildAWGConfigText generates a complete AmneziaWG config with obfuscation.
func buildAWGConfigText(client model.Client, inbound model.Inbound, serverIP string) string {
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)

	mtu := getInt(settings, "mtu", 1320)
	jc := getInt(settings, "jc", 8)
	jmin := getInt(settings, "jmin", 50)
	jmax := getInt(settings, "jmax", 500)
	s1 := getInt(settings, "s1", 50)
	s2 := getInt(settings, "s2", 80)
	s3 := getInt(settings, "s3", 30)
	s4 := getInt(settings, "s4", 15)
	h1 := getString(settings, "h1", "88830977-466888999")
	h2 := getString(settings, "h2", "577571549-1039919960")
	h3 := getString(settings, "h3", "1167874883-1558472606")
	h4 := getString(settings, "h4", "1739740840-2061202155")

	var b strings.Builder
	fmt.Fprintf(&b, "# %s — LucX-UI AWG Client\n\n", client.Email)
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = <CLIENT_PRIVATE_KEY>\n")
	fmt.Fprintf(&b, "Address = 10.100.0.2/32\n")
	fmt.Fprintf(&b, "DNS = 1.1.1.1, 1.0.0.1\n")
	fmt.Fprintf(&b, "MTU = %d\n\n", mtu)
	fmt.Fprintf(&b, "Jc = %d\n", jc)
	fmt.Fprintf(&b, "Jmin = %d\n", jmin)
	fmt.Fprintf(&b, "Jmax = %d\n", jmax)
	fmt.Fprintf(&b, "S1 = %d\n", s1)
	fmt.Fprintf(&b, "S2 = %d\n", s2)
	fmt.Fprintf(&b, "S3 = %d\n", s3)
	fmt.Fprintf(&b, "S4 = %d\n\n", s4)
	fmt.Fprintf(&b, "H1 = %s\n", h1)
	fmt.Fprintf(&b, "H2 = %s\n", h2)
	fmt.Fprintf(&b, "H3 = %s\n", h3)
	fmt.Fprintf(&b, "H4 = %s\n\n", h4)

	if cpsI1 := getString(settings, "i1", ""); cpsI1 != "" {
		fmt.Fprintf(&b, "I1 = <b 0x%s>\n", cpsI1)
		fmt.Fprintf(&b, "I2 = <b 0x%s>\n", getString(settings, "i2", ""))
		fmt.Fprintf(&b, "I3 = <b 0x%s>\n", getString(settings, "i3", ""))
		fmt.Fprintf(&b, "I4 = <b 0x%s>\n", getString(settings, "i4", ""))
		fmt.Fprintf(&b, "I5 = <b 0x%s>\n\n", getString(settings, "i5", ""))
	}

	fmt.Fprintf(&b, "[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", client.ID)
	fmt.Fprintf(&b, "PresharedKey = %s\n", client.Password)
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", serverIP, inbound.Port)
	fmt.Fprintf(&b, "AllowedIPs = 0.0.0.0/0, ::/0\n")
	fmt.Fprintf(&b, "PersistentKeepalive = 25\n")
	return b.String()
}

func getInt(m map[string]interface{}, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func getString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func sanitizeFileName(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, s)
}
