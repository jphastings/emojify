// server/server_test.go
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSuggestEmojiRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/xrpc/me.byjp.emojify.suggestEmoji", "application/json", strings.NewReader(`{"text":"such a beautiful sunny afternoon","limit":3}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
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
			t.Errorf("score %d out of per-mille range [0,1000]", s.Score)
		}
	}
}

func TestSuggestEmojiMissingText(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/xrpc/me.byjp.emojify.suggestEmoji", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
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

// erroringEmbedder simulates an internal/infra failure (e.g. a model error
// or resource exhaustion) that is unrelated to input length, so tests can
// verify handleSuggest doesn't mischaracterize it as TextTooLong.
type erroringEmbedder struct{ dims int }

func (e erroringEmbedder) Dims() int { return e.dims }
func (e erroringEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, errors.New("embedder: simulated failure")
}

func TestSuggestEmojiInternalError(t *testing.T) {
	idx, err := emojify.ReadIndex(bytes.NewReader(emojify.DefaultIndexBytes()))
	if err != nil {
		t.Fatalf("ReadIndex: %v", err)
	}
	// WithIndex pins the index explicitly: erroringEmbedder lives outside
	// package emojify, so it can never implement the unexported indexProvider
	// interface New() uses to pick a matching default index, and would
	// otherwise always be paired with the static one regardless of idx.Dims.
	m, err := emojify.New(
		emojify.WithEmbedder(erroringEmbedder{dims: idx.Dims}),
		emojify.WithIndex(bytes.NewReader(emojify.DefaultIndexBytes())),
	)
	if err != nil {
		t.Fatalf("emojify.New: %v", err)
	}
	t.Cleanup(func() { m.Close() })

	handler := New(m)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/xrpc/me.byjp.emojify.suggestEmoji", "application/json", strings.NewReader(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}

	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.Error != "InternalError" {
		t.Errorf("error = %q, want %q (must not be mismapped to TextTooLong)", body.Error, "InternalError")
	}
	if body.Message == "embedder: simulated failure" || body.Message == "emojify: embedding input: embedder: simulated failure" {
		t.Errorf("internal error message leaked embedder detail: %q", body.Message)
	}
}

func TestLexiconRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/lexicons/me.byjp.emojify.suggestEmoji.json")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestIndexRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("content-type = %q, want text/html; charset=utf-8", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("Emojify")) {
		t.Error("index page body does not mention Emojify")
	}
	// The page must not *load* anything from a third party: it ships inside a
	// self-hostable binary (see server/static.go), so a webfont or CDN script
	// would phone out from someone's private box just to render a page.
	// Ordinary hyperlinks (the GitHub footer link) and the SVG XML namespace
	// in the favicon are not fetches, so they are deliberately not matched.
	for _, fetch := range []string{
		"fonts.googleapis.com",
		"fonts.gstatic.com",
		"<script src=",
		"@import",
		`rel="stylesheet"`,
	} {
		if bytes.Contains(body, []byte(fetch)) {
			t.Errorf("index page loads external resource (%q); it must stay self-contained", fetch)
		}
	}
}

// TestIndexRouteDoesNotSwallowUnknownPaths guards the "/{$}" pattern: a bare
// "GET /" would make the landing page a catch-all, masking 404s for typo'd
// API paths.
func TestIndexRouteDoesNotSwallowUnknownPaths(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/not-a-real-path")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d for unknown path, want 404", resp.StatusCode)
	}
}
