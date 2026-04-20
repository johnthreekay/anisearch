package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/johnthreekay/anisearch/internal/auth"
	"github.com/johnthreekay/anisearch/internal/config"
	"github.com/johnthreekay/anisearch/internal/nyaa"
	"github.com/johnthreekay/anisearch/internal/qbit"
	"github.com/johnthreekay/anisearch/internal/sonarr"
)

type Server struct {
	cfg      *config.Config
	qbit     *qbit.Client
	sonarr   *sonarr.Client
	sessions *auth.Manager
	mux      *http.ServeMux
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		cfg:      cfg,
		qbit:     qbit.NewClient(cfg.QBitURL, cfg.QBitUser, cfg.QBitPass, cfg.QBitCategory),
		sonarr:   sonarr.NewClient(cfg.SonarrURL, cfg.SonarrAPIKey),
		sessions: auth.NewManager(),
		mux:      http.NewServeMux(),
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Auth endpoints (public)
	s.mux.HandleFunc("/login", s.handleLoginPage)
	s.mux.HandleFunc("/setup", s.handleSetupPage)
	s.mux.HandleFunc("/api/login", s.handleLogin)
	s.mux.HandleFunc("/api/setup", s.handleSetup)
	s.mux.HandleFunc("/api/logout", s.handleLogout)
	s.mux.HandleFunc("/api/auth/check", s.handleAuthCheck)

	// Protected
	s.mux.HandleFunc("/", s.requireAuth(s.handleIndex))
	s.mux.HandleFunc("/api/search", s.requireAuth(s.handleSearch))
	s.mux.HandleFunc("/api/search/page", s.requireAuth(s.handleSearchPage))
	s.mux.HandleFunc("/api/grab", s.requireAuth(s.handleGrab))
	s.mux.HandleFunc("/api/rescan", s.requireAuth(s.handleRescan))
	s.mux.HandleFunc("/api/status", s.requireAuth(s.handleStatus))
	s.mux.HandleFunc("/api/torrents", s.requireAuth(s.handleTorrents))
	s.mux.HandleFunc("/api/series", s.requireAuth(s.handleSeries))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Session cookie
		token := auth.GetSessionToken(r)
		if s.sessions.ValidateSession(token) {
			next(w, r)
			return
		}

		// API key
		apiKey := r.Header.Get("X-Api-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("apikey")
		}
		if apiKey != "" && apiKey == s.cfg.APIKey {
			next(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusUnauthorized, APIResponse{Success: false, Error: "authentication required"})
			return
		}

		if s.cfg.NeedsSetup() {
			http.Redirect(w, r, "/setup", http.StatusFound)
		} else {
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	}
}

// ── Auth handlers ─────────────────────────────────

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if s.sessions.ValidateSession(token) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if s.cfg.NeedsSetup() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	http.ServeFile(w, r, "web/templates/login.html")
}

func (s *Server) handleSetupPage(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.NeedsSetup() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	http.ServeFile(w, r, "web/templates/setup.html")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{Success: false, Error: "POST required"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request"})
		return
	}

	if req.Username != s.cfg.Username || !auth.CheckPassword(s.cfg.PasswordHash, req.Password) {
		log.Printf("Failed login attempt for user: %s from %s", req.Username, r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, APIResponse{Success: false, Error: "invalid username or password"})
		return
	}

	token, err := s.sessions.CreateSession()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: "session creation failed"})
		return
	}

	auth.SetSessionCookie(w, token)
	log.Printf("User %s logged in from %s", req.Username, r.RemoteAddr)
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"redirect": "/"}})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{Success: false, Error: "POST required"})
		return
	}
	if !s.cfg.NeedsSetup() {
		writeJSON(w, http.StatusForbidden, APIResponse{Success: false, Error: "setup already completed"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request"})
		return
	}

	if req.Username == "" || len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "username required, password must be at least 8 characters"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: "failed to hash password"})
		return
	}

	s.cfg.Username = req.Username
	s.cfg.PasswordHash = hash
	if err := s.cfg.Save(""); err != nil {
		log.Printf("Failed to save config after setup: %v", err)
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: "failed to save config"})
		return
	}

	token, err := s.sessions.CreateSession()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: "session creation failed"})
		return
	}

	auth.SetSessionCookie(w, token)
	log.Printf("Initial setup completed. User: %s", req.Username)
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"redirect": "/", "apiKey": s.cfg.APIKey}})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if token != "" {
		s.sessions.DestroySession(token)
	}
	auth.ClearSessionCookie(w)

	if strings.HasPrefix(r.Header.Get("Accept"), "application/json") || strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "logged out"})
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if s.sessions.ValidateSession(token) {
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"user": s.cfg.Username}})
		return
	}
	apiKey := r.Header.Get("X-Api-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("apikey")
	}
	if apiKey != "" && apiKey == s.cfg.APIKey {
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"user": "api"}})
		return
	}
	writeJSON(w, http.StatusUnauthorized, APIResponse{Success: false, Error: "not authenticated"})
}

// ── Page handlers ─────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "web/templates/index.html")
}

// ── API handlers ──────────────────────────────────

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "missing query parameter 'q'"})
		return
	}

	category := r.URL.Query().Get("c")
	if category == "" {
		category = nyaa.CategoryAnimeEnglish
	}

	opts := nyaa.SearchOptions{
		Query:           query,
		Category:        category,
		Filter:          r.URL.Query().Get("f"),
		User:            r.URL.Query().Get("user"),
		PreferredGroups: s.cfg.PreferredGroups,
		PreferredRes:    s.cfg.PreferredRes,
	}

	results, hasNext, err := nyaa.SearchHTMLPage(opts, 1)
	if err != nil {
		log.Printf("Search error: %v", err)
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"results": results,
			"hasNext": hasNext,
			"page":    1,
		},
	})
}

func (s *Server) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "missing query parameter 'q'"})
		return
	}

	pageStr := r.URL.Query().Get("p")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 2 {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "invalid page number (must be >= 2)"})
		return
	}

	category := r.URL.Query().Get("c")
	if category == "" {
		category = nyaa.CategoryAnimeEnglish
	}

	opts := nyaa.SearchOptions{
		Query:           query,
		Category:        category,
		Filter:          r.URL.Query().Get("f"),
		User:            r.URL.Query().Get("user"),
		PreferredGroups: s.cfg.PreferredGroups,
		PreferredRes:    s.cfg.PreferredRes,
	}

	results, hasNext, err := nyaa.SearchHTMLPage(opts, page)
	if err != nil {
		log.Printf("Search page %d error: %v", page, err)
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"results": results,
			"hasNext": hasNext,
			"page":    page,
		},
	})
}

func (s *Server) handleGrab(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{Success: false, Error: "POST required"})
		return
	}

	var req struct {
		Magnet  string `json:"magnet"`
		Torrent string `json:"torrent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request body"})
		return
	}

	var err error
	if req.Magnet != "" {
		err = s.qbit.AddMagnet(req.Magnet)
	} else if req.Torrent != "" {
		err = s.qbit.AddTorrentURL(req.Torrent)
	} else {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "provide magnet or torrent URL"})
		return
	}

	if err != nil {
		log.Printf("Grab error: %v", err)
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "torrent added to qBittorrent"})
}

func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{Success: false, Error: "POST required"})
		return
	}

	seriesIDStr := r.URL.Query().Get("seriesId")
	if seriesIDStr != "" {
		seriesID, err := strconv.Atoi(seriesIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "invalid seriesId"})
			return
		}
		if err := s.sonarr.RescanSeries(seriesID); err != nil {
			writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: fmt.Sprintf("rescan triggered for series %d", seriesID)})
	} else {
		if err := s.sonarr.RescanAll(); err != nil {
			writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "full library rescan triggered"})
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"qbit":   "unknown",
		"sonarr": "unknown",
	}

	if err := s.qbit.TestConnection(); err != nil {
		status["qbit"] = fmt.Sprintf("error: %v", err)
	} else {
		status["qbit"] = "connected"
	}

	if err := s.sonarr.TestConnection(); err != nil {
		status["sonarr"] = fmt.Sprintf("error: %v", err)
	} else {
		status["sonarr"] = "connected"
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: status})
}

func (s *Server) handleTorrents(w http.ResponseWriter, r *http.Request) {
	torrents, err := s.qbit.GetTorrents()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: torrents})
}

func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	series, err := s.sonarr.GetSeries()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: series})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
