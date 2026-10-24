package auth

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

//go:embed ui/*
var assets embed.FS
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)

func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, status int, code string) {
	reply(w, status, map[string]string{"error": code})
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		problem(w, 400, "INVALID_INPUT")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		problem(w, 400, "INVALID_INPUT")
		return false
	}
	return true
}
func (s *Service) rateAllowed(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if len(s.rates) >= 4096 {
		if _, exists := s.rates[ip]; !exists {
			return false
		}
	}
	entry := s.rates[ip]
	if s.now().Sub(entry.Start) >= time.Minute {
		entry = rateWindow{Start: s.now()}
	}
	entry.Count++
	s.rates[ip] = entry
	return entry.Count <= 20
}
func (s *Service) Wrap(next http.Handler) http.Handler {
	folder, _ := fs.Sub(assets, "ui")
	files := http.StripPrefix(s.config.Root+"/auth-assets/", http.FileServer(http.FS(folder)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; worker-src 'self'; frame-ancestors 'self'; object-src 'none'; base-uri 'self'; form-action 'self'")
		if s.secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if s.insecureHTTP && !privatePeer(r.RemoteAddr) {
			problem(w, http.StatusForbidden, "LAN_ACCESS_REQUIRED")
			return
		}
		if !strings.EqualFold(r.Host, s.origin.Host) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, s.config.Origin+r.URL.RequestURI(), http.StatusTemporaryRedirect)
			} else {
				problem(w, 403, "ORIGIN_REJECTED")
			}
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && r.Header.Get("Origin") != s.config.Origin {
			problem(w, 403, "ORIGIN_REJECTED")
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			problem(w, 403, "ORIGIN_REJECTED")
			return
		}
		relative := strings.TrimPrefix(r.URL.Path, s.config.Root)
		if strings.HasPrefix(relative, "/auth-assets/") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'self'; object-src 'none'; base-uri 'self'")
			files.ServeHTTP(w, r)
			return
		}
		if relative == "/login" || relative == "/setup" || relative == "/account" {
			if r.Method != "GET" {
				w.WriteHeader(405)
				return
			}
			s.mu.Lock()
			hasAccount := s.account != nil
			_, signedIn := s.authorisedLocked(w, r)
			s.mu.Unlock()
			if relative == "/account" && !signedIn {
				http.Redirect(w, r, s.config.Root+"/login", 303)
				return
			}
			if signedIn && relative != "/account" && !(relative == "/login" && r.URL.Query().Get("reauth") == "1") {
				http.Redirect(w, r, s.config.Root+"/", 303)
				return
			}
			if relative != "/account" && (relative == "/setup") == hasAccount {
				destination := "/setup"
				if hasAccount {
					destination = "/login"
				}
				http.Redirect(w, r, s.config.Root+destination, 303)
				return
			}
			page, _ := assets.ReadFile("ui/login.html")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'self'; object-src 'none'; base-uri 'self'; form-action 'self'")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(strings.ReplaceAll(string(page), "__ROOT__", s.config.Root)))
			return
		}
		if strings.HasPrefix(relative, "/api/auth/") {
			s.route(w, r, strings.TrimPrefix(relative, "/api/auth/"))
			return
		}
		s.mu.Lock()
		_, allowed := s.authorisedLocked(w, r)
		setup := s.account == nil
		s.mu.Unlock()
		if !allowed {
			if strings.HasPrefix(relative, "/api/") {
				problem(w, 401, "AUTH_REQUIRED")
				return
			}
			destination := "/login"
			if setup {
				destination = "/setup"
			}
			http.Redirect(w, r, s.config.Root+destination, 303)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Service) route(w http.ResponseWriter, r *http.Request, path string) {
	if strings.HasPrefix(path, "backup/") {
		s.backupRoute(w, r, path)
		return
	}
	s.mu.Lock()
	failed := s.storageFailed
	s.mu.Unlock()
	if failed {
		problem(w, 503, "STORAGE_ERROR")
		return
	}
	if path == "state" && r.Method == "GET" {
		s.mu.Lock()
		entry, signedIn := s.authorisedLocked(w, r)
		state := map[string]any{"setupRequired": s.account == nil, "setupKeyRequired": s.setupKey != "", "authenticated": signedIn, "passkeys": 0, "fresh": false, "passkeysAvailable": s.webauthn != nil, "insecureHTTP": s.insecureHTTP}
		if s.account != nil {
			state["passkeys"] = len(s.account.Credentials)
			state["backupAvailable"] = s.config.Backup != nil
		}
		if signedIn {
			state["username"] = s.account.Username
			if raw, err := os.ReadFile(filepath.Join(s.config.DataDir, ".restore-result.json")); err == nil {
				var result struct {
					Success bool      `json:"success"`
					Time    time.Time `json:"time"`
				}
				if json.Unmarshal(raw, &result) == nil && s.now().Sub(result.Time) < time.Hour {
					state["restoreResult"] = result
				}
			}
			state["fresh"] = !entry.AuthenticatedAt.IsZero() && s.now().Sub(entry.AuthenticatedAt) < ceremonyLifetime
		}
		s.mu.Unlock()
		reply(w, 200, state)
		return
	}
	if path == "devices" && r.Method == "GET" {
		s.devices(w, r)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !s.rateAllowed(r) {
		w.Header().Set("Retry-After", "60")
		problem(w, 429, "RATE_LIMITED")
		return
	}
	if strings.HasPrefix(path, "passkey/") && s.webauthn == nil {
		problem(w, http.StatusForbidden, "PASSKEY_REQUIRES_HTTPS")
		return
	}
	switch path {
	case "setup":
		s.setup(w, r)
	case "login":
		s.login(w, r)
	case "logout":
		s.logout(w, r)
	case "passkey/register/begin":
		s.registerBegin(w, r)
	case "passkey/register/finish":
		s.registerFinish(w, r)
	case "passkey/login/begin":
		s.loginBegin(w, r)
	case "passkey/login/finish":
		s.loginFinish(w, r)
	case "devices/revoke":
		s.revokeDevice(w, r)
	case "devices/rename":
		s.renameDevice(w, r)
	default:
		problem(w, 404, "NOT_FOUND")
	}
}
func (s *Service) setup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		SetupKey string `json:"setupKey"`
		Remember bool   `json:"remember"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !usernamePattern.MatchString(input.Username) || !validPassword(input.Password) {
		problem(w, 400, "CREDENTIAL_RULES")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.account != nil {
		problem(w, 409, "SETUP_COMPLETED")
		return
	}
	if s.setupKey != "" && tokenHash(input.SetupKey) != s.setupKey {
		problem(w, 403, "SETUP_KEY_INVALID")
		return
	}
	if s.setupKey == "" {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if parsed := net.ParseIP(ip); parsed == nil || !parsed.IsLoopback() {
			problem(w, 403, "ORIGIN_REJECTED")
			return
		}
	}
	// O_EXCL also prevents two concurrent processes from claiming first setup.
	a := &account{ID: randomToken(), Username: input.Username, Password: passwordHash(input.Password)}
	file, err := os.OpenFile(s.accountPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		problem(w, 409, "SETUP_COMPLETED")
		return
	}
	data, _ := json.Marshal(a)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		problem(w, 500, "STORAGE_ERROR")
		return
	}
	s.account = a
	s.setupKey = ""
	_ = os.Remove(filepath.Join(s.config.DataDir, ".setup-key"))
	if err = s.startSessionLocked(w, r, input.Remember); err != nil {
		problem(w, 500, "STORAGE_ERROR")
		return
	}
	reply(w, 201, map[string]bool{"ok": true})
}
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Username) > 128 || len(input.Password) > 512 {
		problem(w, 401, "LOGIN_FAILED")
		return
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		w.Header().Set("Retry-After", "2")
		problem(w, 429, "RATE_LIMITED")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.account == nil {
		problem(w, 409, "SETUP_REQUIRED")
		return
	}
	if s.now().Before(s.account.LockedUntil) {
		w.Header().Set("Retry-After", "900")
		problem(w, 429, "ACCOUNT_LOCKED")
		return
	}
	validPassword := passwordMatches(s.account.Password, input.Password)
	if input.Username != s.account.Username || !validPassword {
		if err := s.failLocked(); err != nil {
			problem(w, 500, "STORAGE_ERROR")
			return
		}
		if s.now().Before(s.account.LockedUntil) {
			w.Header().Set("Retry-After", "900")
			problem(w, 429, "ACCOUNT_LOCKED")
		} else {
			problem(w, 401, "LOGIN_FAILED")
		}
		return
	}
	s.resetFailuresLocked()
	if err := s.saveLocked(); err != nil {
		problem(w, 500, "STORAGE_ERROR")
		return
	}
	if err := s.startSessionLocked(w, r, input.Remember); err != nil {
		problem(w, 500, "DEVICE_LIMIT_OR_STORAGE")
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenHash(cookieValue(r, "session")))
	if s.account != nil {
		hash := tokenHash(cookieValue(r, "device"))
		kept := s.account.Devices[:0]
		for _, d := range s.account.Devices {
			if d.Hash != hash {
				kept = append(kept, d)
			}
		}
		s.account.Devices = kept
		if err := s.saveLocked(); err != nil {
			problem(w, 500, "STORAGE_ERROR")
			return
		}
	}
	s.cookie(w, "session", "", time.Unix(0, 0))
	s.cookie(w, "device", "", time.Unix(0, 0))
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Service) devices(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.authorisedLocked(w, r)
	if !ok {
		problem(w, 401, "AUTH_REQUIRED")
		return
	}
	list := make([]map[string]any, 0)
	for _, d := range s.account.Devices {
		if s.now().Before(d.ExpiresAt) {
			list = append(list, map[string]any{"id": d.ID, "name": d.Name, "label": d.Label, "createdAt": d.CreatedAt, "lastSeen": d.LastSeen, "expiresAt": d.ExpiresAt, "current": d.ID == entry.DeviceID})
		}
	}
	reply(w, 200, map[string]any{"devices": list})
}
func (s *Service) renameDevice(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ID == "" || !utf8.ValidString(input.Name) || utf8.RuneCountInString(input.Name) > 64 || strings.ContainsFunc(input.Name, unicode.IsControl) {
		problem(w, 400, "INVALID_INPUT")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.authorisedLocked(w, r); !ok {
		problem(w, 401, "AUTH_REQUIRED")
		return
	}
	for i := range s.account.Devices {
		d := &s.account.Devices[i]
		if d.ID != input.ID || !s.now().Before(d.ExpiresAt) {
			continue
		}
		d.Label = strings.TrimSpace(input.Name)
		if err := s.saveLocked(); err != nil {
			problem(w, 500, "STORAGE_ERROR")
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	problem(w, 404, "DEVICE_NOT_FOUND")
}
func (s *Service) revokeDevice(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID  string `json:"id"`
		All bool   `json:"all"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ID == "" && !input.All {
		problem(w, 400, "INVALID_INPUT")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.authorisedLocked(w, r)
	if !ok {
		problem(w, 401, "AUTH_REQUIRED")
		return
	}
	kept := s.account.Devices[:0]
	for _, d := range s.account.Devices {
		if !input.All && d.ID != input.ID {
			kept = append(kept, d)
		}
	}
	s.account.Devices = kept
	if err := s.saveLocked(); err != nil {
		problem(w, 500, "STORAGE_ERROR")
		return
	}
	for hash, entry := range s.sessions {
		if input.All || (entry.DeviceID != "" && entry.DeviceID == input.ID) {
			delete(s.sessions, hash)
		}
	}
	if input.All {
		s.cookie(w, "session", "", time.Unix(0, 0))
		s.cookie(w, "device", "", time.Unix(0, 0))
	}
	reply(w, 200, map[string]bool{"ok": true})
}
