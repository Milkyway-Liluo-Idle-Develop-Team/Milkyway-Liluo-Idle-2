// Package wsx is the WebSocket framework: connection management, message
// routing, and lifecycle. It is transport-only — it knows nothing about
// users, auth, or game state. Modules register handlers by message type,
// and the Hub dispatches inbound frames to them.
//
// Wire format (JSON, single envelope for both directions):
//
//	{ "id": "...", "type": "module.action", "payload": {...} }
//
// Server replies that target a specific request reuse the same id. Pushes
// from server (events) leave id empty.
package wsx

import (
	"encoding/json"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
)

// Inbound is a message received from a client.
type Inbound struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Outbound is a message sent to a client. Either Payload or Error is set,
// not both. Replies to a specific Inbound copy its ID; broadcasts/pushes
// leave ID empty.
type Outbound struct {
	ID      string             `json:"id,omitempty"`
	Type    string             `json:"type"`
	Payload any                `json:"payload,omitempty"`
	Error   *apperror.AppError `json:"error,omitempty"`
}

// DecodePayload unmarshals an Inbound's payload into dst. Returns a
// BadRequest apperror on failure so handlers can return it directly.
func (in Inbound) DecodePayload(dst any) error {
	if len(in.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(in.Payload, dst); err != nil {
		return apperror.BadRequest("invalid message payload").WithCause(err)
	}
	return nil
}
