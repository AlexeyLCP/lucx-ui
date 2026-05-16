// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telegram

import (
	"encoding/json"
	"os"
	"sync"
)

const langFilePath = "/etc/lucx-ui/lucx_tg_langs.json"

// LangStore provides persistent per-user language preferences.
// Language choices survive panel restarts via JSON file storage.
type LangStore struct {
	mu   sync.RWMutex
	langs map[int64]string
}

// NewLangStore creates or loads the language store from disk.
func NewLangStore() *LangStore {
	s := &LangStore{langs: make(map[int64]string)}
	data, err := os.ReadFile(langFilePath)
	if err == nil {
		json.Unmarshal(data, &s.langs)
	}
	return s
}

// Get returns the language code for a chat ID, or empty string if not set.
func (s *LangStore) Get(chatID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.langs[chatID]
}

// Set stores a language preference and persists to disk.
func (s *LangStore) Set(chatID int64, lang string) {
	s.mu.Lock()
	s.langs[chatID] = lang
	s.mu.Unlock()
	s.save()
}

func (s *LangStore) save() {
	s.mu.RLock()
	data, err := json.Marshal(s.langs)
	s.mu.RUnlock()
	if err != nil {
		return
	}
	os.WriteFile(langFilePath, data, 0644)
}
