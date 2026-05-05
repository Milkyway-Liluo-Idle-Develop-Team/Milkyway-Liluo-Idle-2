// Package testapi exposes test-only HTTP endpoints for E2E fixture management.
// These routes are registered only in non-production environments.
package testapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/edrowsluo/new-mli/backend/internal/db"
	dbgen "github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/edrowsluo/new-mli/backend/internal/httpx"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/go-chi/chi/v5"
)

// Handler exposes test-only routes.
type Handler struct {
	db      *sql.DB
	sessMgr *session.Manager
}

// NewHandler builds a Handler.
func NewHandler(db *sql.DB, sessMgr *session.Manager) *Handler {
	return &Handler{db: db, sessMgr: sessMgr}
}

// Mount registers routes on r under /test.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/test", func(r chi.Router) {
		r.Post("/delete-user", h.deleteUser)
		r.Post("/reset-user", h.resetUser)
		r.Post("/evict-session", h.evictSession)
		r.Post("/cleanup-expired", h.cleanupExpired)
	})
}

type deleteUserReq struct {
	Username string `json:"username"`
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	var req deleteUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Username == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "username required"})
		return
	}

	ctx := r.Context()

	// 1. Resolve user ID
	var userID int64
	if err := h.db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", req.Username).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			httpx.NoContent(w)
			return
		}
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 2. Evict in-memory session so the DB delete won't race with a running game loop
	if h.sessMgr != nil {
		h.sessMgr.Evict(userID)
	}

	// 3. Delete user (all child tables have ON DELETE CASCADE)
	if _, err := h.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httpx.NoContent(w)
}

type evictSessionReq struct {
	Username string `json:"username"`
}

type resetUserReq struct {
	Username string `json:"username"`
}

func (h *Handler) evictSession(w http.ResponseWriter, r *http.Request) {
	var req evictSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Username == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "username required"})
		return
	}

	ctx := r.Context()
	var userID int64
	if err := h.db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", req.Username).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			httpx.NoContent(w)
			return
		}
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if h.sessMgr != nil {
		if s, ok := h.sessMgr.Get(userID); ok {
			_ = s.GraceExpireNow(ctx, &db.DB{Conn: h.db, Queries: dbgen.New(h.db)})
		}
		h.sessMgr.Remove(userID)
	}
	httpx.NoContent(w)
}

func (h *Handler) resetUser(w http.ResponseWriter, r *http.Request) {
	var req resetUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Username == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "username required"})
		return
	}

	ctx := r.Context()

	var userID int64
	if err := h.db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", req.Username).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Evict running session so state is rebuilt from scratch on next connect
	if h.sessMgr != nil {
		h.sessMgr.Evict(userID)
	}

	// Delete all game-state rows but keep the user account and sessions
	tables := []string{
		"player_skills",
		"player_inventory",
		"player_unlocked_events",
		"player_active_events",
		"player_equipment",
		"player_init",
	}
	for _, tbl := range tables {
		if _, err := h.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", tbl), userID); err != nil {
			httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	httpx.NoContent(w)
}

type cleanupExpiredReq struct {
	Prefix    string `json:"prefix"`     // e.g. "test_"
	OlderThan string `json:"older_than"` // e.g. "1h", "30m"
}

func (h *Handler) cleanupExpired(w http.ResponseWriter, r *http.Request) {
	var req cleanupExpiredReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Prefix == "" {
		req.Prefix = "test_"
	}
	if req.OlderThan == "" {
		req.OlderThan = "1h"
	}

	duration, err := time.ParseDuration(req.OlderThan)
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid older_than duration"})
		return
	}
	cutoff := time.Now().UTC().Add(-duration)

	ctx := r.Context()

	// Evict any matching in-memory sessions first
	rows, err := h.db.QueryContext(ctx,
		"SELECT id FROM users WHERE username LIKE ? AND created_at < ?",
		req.Prefix+"%", cutoff)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	for _, id := range ids {
		if h.sessMgr != nil {
			h.sessMgr.Evict(id)
		}
	}

	res, err := h.db.ExecContext(ctx,
		"DELETE FROM users WHERE username LIKE ? AND created_at < ?",
		req.Prefix+"%", cutoff)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	affected, _ := res.RowsAffected()
	httpx.JSON(w, http.StatusOK, map[string]any{"deleted": affected})
}
