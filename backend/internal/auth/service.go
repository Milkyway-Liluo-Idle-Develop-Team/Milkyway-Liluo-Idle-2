// Package auth implements the user system: registration, login, session
// lifecycle, and request authentication. Other modules depend on this for
// the current user; this package depends on db, config, apperror, httpx,
// and wsx, but on no business module.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/edrowsluo/new-mli/backend/internal/apperror"
	"github.com/edrowsluo/new-mli/backend/internal/config"
	"github.com/edrowsluo/new-mli/backend/internal/db"
	"github.com/edrowsluo/new-mli/backend/internal/db/gen"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	minUsernameLen = 3
	maxUsernameLen = 32
	minPasswordLen = 8
	maxPasswordLen = 128

	tokenBytes = 32 // 256-bit random
)

// User is the public user record. Never include PasswordHash here.
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// Session is the public session record. Token is only returned at creation.
type Session struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Service is the user/auth business layer. Construct one per process.
type Service struct {
	db  *db.DB
	cfg config.Auth
}

// NewService creates a Service.
func NewService(database *db.DB, cfg config.Auth) *Service {
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = bcrypt.DefaultCost
	}
	return &Service{db: database, cfg: cfg}
}

// SessionTTL exposes the configured session lifetime so transports can set
// matching cookie ages.
func (s *Service) SessionTTL() time.Duration { return s.cfg.SessionTTL }

// Register creates a new user. Returns Conflict if the username is taken.
func (s *Service) Register(ctx context.Context, username, password string) (User, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(password); err != nil {
		return User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.BcryptCost)
	if err != nil {
		return User{}, apperror.Internal("hash password").WithCause(err)
	}

	u, err := s.db.Queries.CreateUser(ctx, dbgen.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, apperror.Conflict("username already taken")
		}
		return User{}, apperror.Internal("create user").WithCause(err)
	}
	return toUser(u), nil
}

// Login validates credentials and creates a new session. Returns the
// Session including its raw token (only available at creation).
func (s *Service) Login(ctx context.Context, username, password, userAgent, ip string) (Session, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return Session{}, apperror.Unauthorized("invalid credentials")
	}

	u, err := s.db.Queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Compare against a constant to keep timing closer to the success path.
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return Session{}, apperror.Unauthorized("invalid credentials")
		}
		return Session{}, apperror.Internal("lookup user").WithCause(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return Session{}, apperror.Unauthorized("invalid credentials")
	}

	return s.createSession(ctx, u.ID, userAgent, ip)
}

// Authenticate looks up a session by its raw token, refreshing last_used_at
// on success. Returns Unauthorized for missing/expired/revoked tokens.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (User, Session, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return User{}, Session{}, apperror.Unauthorized("missing session token")
	}
	hash := hashToken(rawToken)

	sess, err := s.db.Queries.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, Session{}, apperror.Unauthorized("invalid or expired session")
		}
		return User{}, Session{}, apperror.Internal("lookup session").WithCause(err)
	}

	u, err := s.db.Queries.GetUserByID(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, Session{}, apperror.Unauthorized("session owner missing")
		}
		return User{}, Session{}, apperror.Internal("lookup user").WithCause(err)
	}

	// Best-effort touch; failures here shouldn't break the request.
	_ = s.db.Queries.TouchSession(ctx, sess.ID)

	return toUser(u), toSession(sess, ""), nil
}

// Logout revokes a single session by its raw token. Idempotent.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}
	hash := hashToken(rawToken)
	sess, err := s.db.Queries.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // already gone
		}
		return apperror.Internal("lookup session").WithCause(err)
	}
	if err := s.db.Queries.RevokeSession(ctx, sess.ID); err != nil {
		return apperror.Internal("revoke session").WithCause(err)
	}
	return nil
}

// LogoutAll revokes every active session for the user.
func (s *Service) LogoutAll(ctx context.Context, userID int64) error {
	if err := s.db.Queries.RevokeAllSessionsForUser(ctx, userID); err != nil {
		return apperror.Internal("revoke all sessions").WithCause(err)
	}
	return nil
}

// CleanupExpired removes expired/old-revoked rows. Call periodically (e.g.
// once an hour) from a background goroutine.
func (s *Service) CleanupExpired(ctx context.Context) error {
	if err := s.db.Queries.DeleteExpiredSessions(ctx); err != nil {
		return apperror.Internal("delete expired sessions").WithCause(err)
	}
	return nil
}

func (s *Service) createSession(ctx context.Context, userID int64, userAgent, ip string) (Session, error) {
	rawToken, err := generateToken()
	if err != nil {
		return Session{}, apperror.Internal("generate token").WithCause(err)
	}
	id := uuid.NewString()
	expires := time.Now().Add(s.cfg.SessionTTL).UTC()

	sess, err := s.db.Queries.CreateSession(ctx, dbgen.CreateSessionParams{
		ID:        id,
		UserID:    userID,
		TokenHash: hashToken(rawToken),
		UserAgent: userAgent,
		Ip:        ip,
		ExpiresAt: expires,
	})
	if err != nil {
		return Session{}, apperror.Internal("create session").WithCause(err)
	}
	return toSession(sess, rawToken), nil
}

// --- helpers ---

// dummyHash is a precomputed bcrypt hash so failed username lookups still
// run a bcrypt compare (constant-time-ish defense vs. user-enumeration).
var dummyHash = []byte("$2a$12$abcdefghijklmnopqrstuuLZdQ6mfYxK6.lnQq.Eo3IYcFJ9b3K3W")

func generateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the lowercase hex SHA-256 of the raw token. SHA-256 is
// adequate here because the token is high-entropy (256-bit random); we use
// a hash only so a DB leak doesn't expose live tokens.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeUsername(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) < minUsernameLen || utf8.RuneCountInString(s) > maxUsernameLen {
		return "", apperror.Validation("username length must be 3-32 characters")
	}
	for _, r := range s {
		if !(r == '_' || r == '-' || r == '.' ||
			(r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z')) {
			return "", apperror.Validation("username may only contain letters, digits, '_', '-', '.'")
		}
	}
	return s, nil
}

func validatePassword(s string) error {
	n := len(s)
	if n < minPasswordLen || n > maxPasswordLen {
		return apperror.Validation("password length must be 8-128 characters")
	}
	return nil
}

func toUser(u dbgen.User) User {
	return User{
		ID:        u.ID,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}
}

func toSession(s dbgen.Session, rawToken string) Session {
	return Session{
		ID:        s.ID,
		UserID:    s.UserID,
		Token:     rawToken,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}

// isUniqueViolation reports whether err corresponds to a SQLite UNIQUE
// constraint violation. SQLite drivers don't share a single error type,
// so we string-match — every driver shapes this message the same way.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}
