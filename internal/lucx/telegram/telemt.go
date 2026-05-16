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

	"github.com/mhsanaei/3x-ui/v3/database/model"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// SendTelemtClientData sends client statistics with a "Connect" inline button
// containing a tg://proxy deep link.
func SendTelemtClientData(
	bot *telego.Bot,
	chatID int64,
	client model.Client,
	inbound model.Inbound,
	serverIP string,
	statsText string,
) error {
	// Extract client secret from inbound settings
	secret := extractTelemtSecret(client, inbound)

	// Build tg://proxy deep link
	proxyLink := fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", serverIP, inbound.Port, secret)

	// Create inline keyboard with "Connect" button
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔗 Connect").WithURL(proxyLink),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📋 Copy Link").WithCallbackData("copy_proxy_link_"+secret),
		),
	)

	msg := tu.Message(tu.ID(chatID), statsText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := bot.SendMessage(context.Background(), msg)
	return err
}

// extractTelemtSecret extracts the ee-prefixed secret for a client.
func extractTelemtSecret(client model.Client, inbound model.Inbound) string {
	// Try from client password first
	if client.Password != "" {
		return client.Password
	}

	// Try from inbound settings
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err == nil {
		if clients, ok := settings["clients"].([]interface{}); ok {
			for _, c := range clients {
				if cm, ok := c.(map[string]interface{}); ok {
					if email, _ := cm["email"].(string); email == client.Email {
						if secret, _ := cm["secret"].(string); secret != "" {
							return secret
						}
						if pass, _ := cm["password"].(string); pass != "" {
							return pass
						}
					}
				}
			}
		}
	}
	return ""
}

// GetServerIP returns a display-friendly server address from the inbound.
func GetServerIP(inbound model.Inbound) string {
	if inbound.Listen != "" && inbound.Listen != "0.0.0.0" {
		return inbound.Listen
	}
	return ""
}
