// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package chat

import (
	"strings"
	"time"
)

type turnAccum struct {
	chatID string
	buf    strings.Builder
	done   bool
}

// ObserveTaskEvent accumulates assistant.delta for chat-linked tasks and
// persists a summary when the task reaches done/error — even if the App left.
func (s *Store) ObserveTaskEvent(taskID, kind string, payload map[string]any) {
	if s == nil || s.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	tsk := s.tasks.Get(taskID)
	if tsk == nil || tsk.ChatID == nil {
		return
	}
	chatID := strings.TrimSpace(*tsk.ChatID)
	if chatID == "" {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}

	switch kind {
	case "assistant.delta":
		text, _ := payload["text"].(string)
		if text == "" {
			return
		}
		s.mu.Lock()
		if s.turns == nil {
			s.turns = map[string]*turnAccum{}
		}
		a := s.turns[taskID]
		if a == nil {
			a = &turnAccum{chatID: chatID}
			s.turns[taskID] = a
		}
		if !a.done {
			a.buf.WriteString(text)
		}
		s.mu.Unlock()

	case "done", "error":
		s.mu.Lock()
		text := ""
		if a := s.turns[taskID]; a != nil {
			text = a.buf.String()
			a.done = true
			delete(s.turns, taskID)
		}
		s.mu.Unlock()

		if text == "" {
			if kind == "error" {
				if msg, _ := payload["message"].(string); strings.TrimSpace(msg) != "" {
					text = strings.TrimSpace(msg)
				}
			}
		}
		if text == "" {
			s.MarkIdle(chatID)
			return
		}
		_, _ = s.RecordAssistant(chatID, taskID, text)
	}
}

// MarkIdle clears running status when a turn ends without assistant text.
func (s *Store) MarkIdle(chatID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.chats[chatID]
	if !ok {
		return
	}
	if sess.Status == StatusIdle {
		return
	}
	sess.Status = StatusIdle
	sess.UpdatedAt = s.now().Format(time.RFC3339Nano)
	s.persistLocked(sess)
}
