package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
)

func TestPairAndProtectedRoute(t *testing.T) {
	store := auth.NewStore("TEST-CODE")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/pair", store.HandlePair)
	mux.Handle("GET /v1/auth/me", store.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})))

	t.Run("bad pair code", func(t *testing.T) {
		body := []byte(`{"pairCode":"WRONG"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/pair", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", rec.Code)
		}
	})

	var token string
	t.Run("good pair code", func(t *testing.T) {
		body := []byte(`{"pairCode":"TEST-CODE"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/pair", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expiresAt"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Token == "" || resp.ExpiresAt == "" {
			t.Fatalf("empty token response: %+v", resp)
		}
		token = resp.Token
	})

	t.Run("protected without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", rec.Code)
		}
	})

	t.Run("protected with token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
