package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"google.golang.org/protobuf/proto"
)

// WSClient manages a single WebSocket connection to the game server.
// It speaks binary protobuf envelopes (the server default codec).
type WSClient struct {
	conn   *websocket.Conn
	done   chan struct{} // closed by caller to signal shutdown
	recvCh chan *pb.Envelope
}

// Connect opens a WS connection to the server with the given auth token.
func (c *WSClient) Connect(ctx context.Context, serverURL, token string) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("parse server url: %w", err)
	}
	// Replace http/https with ws/wss.
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = "/ws"
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	ws, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {serverURL}},
	})
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}

	c.conn = ws
	c.done = make(chan struct{})
	c.recvCh = make(chan *pb.Envelope, 64)
	go c.readLoop()
	return nil
}

// Send transmits a typed protobuf envelope.
func (c *WSClient) Send(id, typ string, payload proto.Message) error {
	env := &pb.Envelope{
		Id:   id,
		Type: typ,
	}
	if payload != nil {
		data, err := proto.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		env.Payload = data
	}
	data, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.conn.Write(ctx, websocket.MessageBinary, data)
}

// Recv returns a channel that yields every inbound server envelope.
func (c *WSClient) Recv() <-chan *pb.Envelope { return c.recvCh }

// Close cleanly shuts down the connection.
func (c *WSClient) Close() error {
	close(c.done)
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "client closing")
	}
	return nil
}

func (c *WSClient) readLoop() {
	defer close(c.recvCh)
	for {
		select {
		case <-c.done:
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		typ, data, err := c.conn.Read(ctx)
		cancel()
		if err != nil {
			select {
			case <-c.done:
				return
			default:
				c.recvCh <- &pb.Envelope{Type: "__error", Error: &pb.Error{Code: "ws_error", Message: err.Error()}}
				return
			}
		}

		// Ignore text frames (shouldn't happen with default proto codec).
		if typ != websocket.MessageBinary {
			continue
		}

		var env pb.Envelope
		if err := proto.Unmarshal(data, &env); err != nil {
			continue // malformed — drop silently
		}
		select {
		case c.recvCh <- &env:
		case <-c.done:
			return
		}
	}
}
