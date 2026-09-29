// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package auth

import (
	"log"
	"time"
)

// OnChange is invoked after tokens are issued or pruned.
type OnChange func()

func (s *Store) SetOnChange(fn OnChange) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

func (s *Store) notifyChange() {
	s.mu.RLock()
	fn := s.onChange
	s.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// ExportTokens returns token -> expiresAt RFC3339 for persistence.
func (s *Store) ExportTokens() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.tokens))
	now := time.Now().UTC()
	for tok, exp := range s.tokens {
		if now.After(exp) {
			continue
		}
		out[tok] = exp.Format(time.RFC3339Nano)
	}
	return out
}

// ImportTokens restores unexpired tokens from disk.
func (s *Store) ImportTokens(tokens map[string]string) {
	if len(tokens) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	n := 0
	for tok, expStr := range tokens {
		exp, err := time.Parse(time.RFC3339Nano, expStr)
		if err != nil {
			exp, err = time.Parse(time.RFC3339, expStr)
		}
		if err != nil || now.After(exp) {
			continue
		}
		s.tokens[tok] = exp.UTC()
		n++
	}
	log.Printf("auth tokens restored: %d", n)
}
