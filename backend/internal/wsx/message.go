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
	"context"
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

// TypedHandler is a Handler that receives a pre-decoded payload. The
// framework unmarshals the inbound payload into T before calling fn.
type TypedHandler[T any] func(ctx context.Context, c *Conn, req T) error

// HandleTyped registers a typed handler for the given message type.
// The inbound payload is automatically decoded and validated; on failure a
// BadRequest error is returned to the client. Paired with HandleSessionTyped
// in the session package for handlers that also need a locked session.
func HandleTyped[T any](h *Hub, typ string, fn TypedHandler[T]) {
	h.Handle(typ, func(ctx context.Context, c *Conn, in Inbound) error {
		var req T
		if err := in.DecodePayload(&req); err != nil {
			return err
		}
		return fn(ctx, c, req)
	})
}
