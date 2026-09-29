// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultTokenTTL = 30 * 24 * time.Hour

// Store holds pair code and issued bearer tokens (in-memory + optional disk).
type Store struct {
	pairCode string
	ttl      time.Duration

	mu       sync.RWMutex
	tokens   map[string]time.Time // token -> expiresAt
	onChange OnChange
}

func NewStore(pairCode string) *Store {
	return &Store{
		pairCode: pairCode,
		ttl:      defaultTokenTTL,
		tokens:   make(map[string]time.Time),
	}
}

func (s *Store) PairCode() string { return s.pairCode }

type pairRequest struct {
	GatewayURL string `json:"gatewayUrl"`
	PairCode   string `json:"pairCode"`
}

type pairResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

func (s *Store) HandlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req pairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.PairCode == "" || req.PairCode != s.pairCode {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid pair code"})
		return
	}

	token, err := randomToken(32)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token issue failed"})
		return
	}
	exp := time.Now().UTC().Add(s.ttl)

	s.mu.Lock()
	s.tokens[token] = exp
	s.mu.Unlock()
	s.notifyChange()

	writeJSON(w, http.StatusOK, pairResponse{
		Token:     token,
		ExpiresAt: exp.Format(time.RFC3339Nano),
	})
}

// Middleware rejects requests without a valid Bearer token.
func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" || !s.ValidToken(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ValidToken reports whether token is issued and unexpired.
func (s *Store) ValidToken(token string) bool {
	return s.valid(token)
}

func (s *Store) valid(token string) bool {
	s.mu.RLock()
	exp, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().UTC().After(exp) {
		s.mu.Lock()
		delete(s.tokens, token)
		s.mu.Unlock()
		return false
	}
	return true
}

func bearerToken(h string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GeneratePairCode returns a human-typable XXXX-XXXX code.
func GeneratePairCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, 8)
	for i := range out {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out[:4]) + "-" + string(out[4:]), nil
}
