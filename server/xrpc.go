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
	Score int    `json:"score"` // basis points, 0-1000
}

type suggestEmojisResponse struct {
	Suggestions []suggestion `json:"suggestions"`
}

func toBasisPoints(score float32) int {
	bp := int(score*1000 + 0.5)
	if bp < 0 {
		bp = 0
	}
	if bp > 1000 {
		bp = 1000
	}
	return bp
}
