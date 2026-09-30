// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package chat

import (
	"strings"
	"unicode/utf8"
)

const maxTitleRunes = 24

// SuggestTitle builds a provisional session title from the first user message.
// AI may replace it later via Slave chat.autotitle.
func SuggestTitle(text string) string {
	s := strings.TrimSpace(text)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "新对话"
	}
	if utf8.RuneCountInString(s) <= maxTitleRunes {
		return s
	}
	return string([]rune(s)[:maxTitleRunes]) + "…"
}

// SanitizeTitle cleans a user- or AI-provided title.
func SanitizeTitle(text string) string {
	s := strings.TrimSpace(text)
	s = strings.Trim(s, "\"'`「」『』")
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > maxTitleRunes {
		return string([]rune(s)[:maxTitleRunes]) + "…"
	}
	return s
}
