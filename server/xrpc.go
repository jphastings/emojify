// server/xrpc.go
package server

import (
	"encoding/json"
	"net/http"
)

type xrpcError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeXRPCError writes the XRPC error envelope (not Go's default error
// shape) so no handler hand-rolls this format.
func writeXRPCError(w http.ResponseWriter, status int, name, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(xrpcError{Error: name, Message: message})
}

type suggestion struct {
	Emoji string `json:"emoji"`
	Name  string `json:"name"`
	Score int    `json:"score"` // per-mille (parts per thousand), 0-1000
}

type suggestEmojisResponse struct {
	Suggestions []suggestion `json:"suggestions"`
}

// toPerMille converts a 0..1 score to an integer 0-1000 scale (parts per
// thousand). Despite the name similarity, this is NOT basis points — true
// basis points are parts per ten-thousand (0-10000). This wire format was
// already 0-1000 throughout the codebase before this was named accurately,
// and changing the range now would be a breaking API change, so the range
// stays 0-1000; only the name/docs are corrected.
func toPerMille(score float32) int {
	pm := int(score*1000 + 0.5)
	if pm < 0 {
		pm = 0
	}
	if pm > 1000 {
		pm = 1000
	}
	return pm
}
