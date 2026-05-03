// Package api provides HTTP and WebSocket clients for talking to the
// mli game server.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient wraps an http.Client with the mli server base URL.
type HTTPClient struct {
	BaseURL string
	Token   string
	client  *http.Client
}

// NewHTTPClient creates a client. baseURL should not have a trailing slash.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// WithToken returns a copy of the client with the given auth token set.
func (c *HTTPClient) WithToken(token string) *HTTPClient {
	cp := *c
	cp.Token = token
	return &cp
}

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, string(bodyData))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return nil
}

// --- Auth ---

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginResponse struct {
	User    User      `json:"user"`
	Session Session   `json:"session"`
	Expires time.Time `json:"expires_at"`
}

type MeResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *HTTPClient) Register(ctx context.Context, username, password string) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/auth/register", LoginRequest{Username: username, Password: password}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/auth/login", LoginRequest{Username: username, Password: password}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) Me(ctx context.Context) (*MeResponse, error) {
	var out MeResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/auth/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Game Config ---

type GameConfig struct {
	Actions       json.RawMessage `json:"actions"`
	IDRegistry    json.RawMessage `json:"id_registry"`
	Attributes    json.RawMessage `json:"attributes"`
	AttrRegistry  json.RawMessage `json:"attr_registry"`
	LevelCurveCSV string          `json:"level_curve_csv"`
}

func (c *HTTPClient) FetchGameConfig(ctx context.Context) (*GameConfig, error) {
	var out GameConfig
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/game/config", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
