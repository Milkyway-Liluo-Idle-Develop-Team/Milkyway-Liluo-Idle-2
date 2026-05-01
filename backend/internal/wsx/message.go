// Package wsx is the WebSocket framework: connection management, message
// routing, and lifecycle. It is transport-only — it knows nothing about
// users, auth, or game state. Modules register handlers by message type,
// and the Hub dispatches inbound frames to them.
//
// Wire format (protobuf binary, single envelope for both directions):
//
//	Envelope { id, type, payload bytes, error }
//
// Server replies that target a specific request reuse the same id. Pushes
// from server (events) leave id empty.
package wsx

import (
	"context"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Inbound is a message received from a client.
type Inbound struct {
	ID      string
	Type    string
	Payload []byte
}

// Outbound is a message sent to a client. Either Payload or Error is set,
// not both. Replies to a specific Inbound copy its ID; broadcasts/pushes
// leave ID empty.
type Outbound struct {
	ID      string
	Type    string
	Payload proto.Message
	Error   *apperror.AppError
}

// DecodePayload unmarshals an Inbound's payload into dst. Tries proto binary
// first, then protojson (dev mode) as fallback.
func (in Inbound) DecodePayload(dst proto.Message) error {
	if len(in.Payload) == 0 {
		return nil
	}
	if err := proto.Unmarshal(in.Payload, dst); err != nil {
		if err2 := protojson.Unmarshal(in.Payload, dst); err2 != nil {
			return apperror.BadRequest("invalid message payload").WithCause(err)
		}
	}
	return nil
}

// TypedHandler is a Handler that receives a pre-decoded payload. The
// framework unmarshals the inbound payload into T before calling fn.
type TypedHandler[T proto.Message] func(ctx context.Context, c *Conn, req T) error

// HandleTyped registers a typed handler for the given message type.
// The inbound payload is automatically decoded and validated; on failure a
// BadRequest error is returned to the client. Paired with HandleSessionTyped
// in the session package for handlers that also need a locked session.
func HandleTyped[T proto.Message](h *Hub, typ string, fn TypedHandler[T]) {
	h.Handle(typ, func(ctx context.Context, c *Conn, in Inbound) error {
		var req T
		if err := in.DecodePayload(req); err != nil {
			return err
		}
		return fn(ctx, c, req)
	})
}
