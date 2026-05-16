// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telegram

import (
	"context"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// SendLanguageMenu sends an inline keyboard for language selection.
// Called from the /lang command handler.
func SendLanguageMenu(bot *telego.Bot, chatID int64) error {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("English").WithCallbackData("lucx_lang:en-US"),
			tu.InlineKeyboardButton("Русский").WithCallbackData("lucx_lang:ru-RU"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("فارسی").WithCallbackData("lucx_lang:fa-IR"),
			tu.InlineKeyboardButton("中文").WithCallbackData("lucx_lang:zh-CN"),
		),
	)

	msg := tu.Message(tu.ID(chatID), "🌐 Choose language / Выберите язык:")
	msg.ReplyMarkup = keyboard
	_, err := bot.SendMessage(context.Background(), msg)
	return err
}

// HandleLanguageCallback processes a lucx_lang:* callback data string.
// Returns the language code (e.g., "ru-RU") and true if this was a language callback.
func HandleLanguageCallback(callbackData string) (lang string, isLangCallback bool) {
	if len(callbackData) < 11 || callbackData[:10] != "lucx_lang:" {
		return "", false
	}
	return callbackData[10:], true
}
