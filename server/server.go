// server/server.go
package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jphastings/emojify"
)

const (
	defaultLimit = 3
	maxLimit     = 5
)

type handler struct {
	matcher *emojify.Matcher
}

// New builds the emojify HTTP handler: the XRPC query, health checks, and
// the lexicon document, wrapped in CORS/rate-limiting/max-bytes/bounded-
// concurrency middleware.
func New(matcher *emojify.Matcher) http.Handler {
	return newWithLimits(matcher, defaultLimitConfig)
}

func newWithLimits(matcher *emojify.Matcher, cfg limitConfig) http.Handler {
	h := &handler{matcher: matcher}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /xrpc/me.byjp.emojify.suggestEmojis", h.handleSuggest)
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /xrpc/_health", h.handleXRPCHealth)
	mux.HandleFunc("GET /lexicons/me.byjp.emojify.suggestEmojis.json", h.handleLexicon)

	var wrapped http.Handler = mux
	wrapped = withBoundedConcurrency(wrapped)
	wrapped = withMaxBytes(wrapped)
	wrapped = withRateLimit(cfg, wrapped)
	wrapped = withCORS(wrapped)
	return wrapped
}

func (h *handler) handleSuggest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	text := r.URL.Query().Get("text")
	if text == "" {
		writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", "text parameter is required")
		return
	}

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxLimit {
			writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", "limit must be an integer between 1 and 5")
			return
		}
		limit = n
	}

	suggestions, err := h.matcher.Suggest(r.Context(), text, limit)
	if err != nil {
		if errors.Is(err, emojify.ErrTextTooLong) {
			writeXRPCError(w, http.StatusBadRequest, "TextTooLong", err.Error())
		} else {
			writeXRPCError(w, http.StatusInternalServerError, "InternalError", "an internal error occurred")
		}
		return
	}
	if len(suggestions) == 0 {
		writeXRPCError(w, http.StatusNotFound, "NoMatch", "no suggestion cleared the minimum score")
		return
	}

	out := make([]suggestion, len(suggestions))
	for i, s := range suggestions {
		out[i] = suggestion{Emoji: s.Emoji, Name: s.Name, Score: toBasisPoints(s.Score)}
	}

	emojis := make([]string, len(out))
	for i, s := range out {
		emojis[i] = s.Emoji
	}
	log.Printf("suggestEmojis: %v (%s)", emojis, time.Since(start))

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, suggestEmojisResponse{Suggestions: out})
}

func (h *handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]any{"status": "ok"})
}

func (h *handler) handleXRPCHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"version": "0.1.0"})
}

func (h *handler) handleLexicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(emojify.LexiconJSON)
}

func writeJSON(w http.ResponseWriter, v any) {
	json.NewEncoder(w).Encode(v)
}
