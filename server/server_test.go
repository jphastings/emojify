// server/server_test.go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jphastings/emojify"
)

func newTestMatcher(t *testing.T) *emojify.Matcher {
	t.Helper()
	m, err := emojify.New()
	if err != nil {
		t.Fatalf("emojify.New: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestSuggestEmojisRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/xrpc/me.byjp.emojify.suggestEmojis?text=such+a+beautiful+sunny+afternoon&limit=3")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Suggestions []struct {
			Emoji string `json:"emoji"`
			Name  string `json:"name"`
			Score int    `json:"score"`
		} `json:"suggestions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body.Suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}
	for _, s := range body.Suggestions {
		if s.Score < 0 || s.Score > 1000 {
			t.Errorf("score %d out of basis-point range [0,1000]", s.Score)
		}
	}
}

func TestSuggestEmojisMissingText(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/xrpc/me.byjp.emojify.suggestEmojis")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Fatalf("status = %d, want 4xx", resp.StatusCode)
	}
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Error == "" {
		t.Errorf("expected an XRPC error envelope, got: %+v", body)
	}
}

func TestHealthzRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestLexiconRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/lexicons/me.byjp.emojify.suggestEmojis.json")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}
