package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "anisearch_session"
	SessionMaxAge     = 30 * 24 * time.Hour // 30 days
	BcryptCost        = 12
)

type Session struct {
	Token     string
	CreatedAt time.Time
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session // token -> session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// HashPassword creates a bcrypt hash of the plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateSession generates a new session token and stores it.
func (m *Manager) CreateSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	token := hex.EncodeToString(b)

	m.mu.Lock()
	m.sessions[token] = &Session{
		Token:     token,
		CreatedAt: time.Now(),
	}
	m.mu.Unlock()

	return token, nil
}

// ValidateSession checks if a session token is valid and not expired.
func (m *Manager) ValidateSession(token string) bool {
	if token == "" {
		return false
	}

	m.mu.RLock()
	session, exists := m.sessions[token]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Since(session.CreatedAt) > SessionMaxAge {
		m.DestroySession(token)
		return false
	}

	return true
}

// DestroySession removes a session.
func (m *Manager) DestroySession(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

// SetSessionCookie writes the session cookie to the response.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // set to true if behind HTTPS (Cloudflare handles this)
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// GetSessionToken extracts the session token from the request cookie.
func GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// CleanupExpired removes expired sessions. Call periodically.
func (m *Manager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for token, session := range m.sessions {
		if time.Since(session.CreatedAt) > SessionMaxAge {
			delete(m.sessions, token)
		}
	}
}
