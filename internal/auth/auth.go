// Package auth protects the single-administrator web interface.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/reactor2k3/ddns-updater/internal/backup"
	"golang.org/x/crypto/argon2"
)

const (
	sessionLifetime  = 8 * time.Hour
	deviceLifetime   = 30 * 24 * time.Hour
	ceremonyLifetime = 5 * time.Minute
	lockDuration     = 15 * time.Minute
	maxFailures      = 5
)

type Config struct {
	Origin  string
	Root    string
	DataDir string
	Backup  *backup.Web
	// SetupKey is optional for loopback origins, and required for other origins.
	// An empty key on a non-loopback origin is securely generated and returned by New.
	SetupKey string
	// Only the loopback-bound development preview may omit a setup key.
	AllowLocalSetup bool
	// AllowInsecureHTTP permits unencrypted password login on private LAN IPs.
	AllowInsecureHTTP bool
}
type passwordRecord struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}
type account struct {
	ID            string                `json:"id"`
	Username      string                `json:"username"`
	Password      passwordRecord        `json:"password"`
	Credentials   []webauthn.Credential `json:"credentials"`
	Failures      int                   `json:"failures"`
	FailureWindow time.Time             `json:"failureWindow"`
	LockedUntil   time.Time             `json:"lockedUntil"`
	Devices       []device              `json:"devices"`
}

func (a account) WebAuthnID() []byte                         { id, _ := base64.RawURLEncoding.DecodeString(a.ID); return id }
func (a account) WebAuthnName() string                       { return a.Username }
func (a account) WebAuthnDisplayName() string                { return a.Username }
func (a account) WebAuthnCredentials() []webauthn.Credential { return a.Credentials }

type device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Label     string    `json:"label,omitempty"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"createdAt"`
	LastSeen  time.Time `json:"lastSeen"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type session struct {
	ExpiresAt       time.Time
	AuthenticatedAt time.Time
	DeviceID        string
}
type ceremony struct {
	Data        webauthn.SessionData
	ExpiresAt   time.Time
	Kind        string
	Remember    bool
	SessionHash string
}
type rateWindow struct {
	Start time.Time
	Count int
}
type Service struct {
	mu            sync.Mutex
	account       *account
	config        Config
	origin        *url.URL
	secure        bool
	insecureHTTP  bool
	setupKey      string
	webauthn      *webauthn.WebAuthn
	sessions      map[string]session
	ceremonies    map[string]ceremony
	rates         map[string]rateWindow
	kdf           chan struct{}
	now           func() time.Time
	storageFailed bool
}

func New(config Config) (*Service, string, error) {
	origin, err := url.Parse(config.Origin)
	if err != nil || origin.Host == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, "", errors.New("AUTH_ORIGIN must be an absolute origin without path or credentials")
	}
	local := origin.Hostname() == "localhost" || (net.ParseIP(origin.Hostname()) != nil && net.ParseIP(origin.Hostname()).IsLoopback())
	lanHTTP := origin.Scheme == "http" && !local && config.AllowInsecureHTTP && privateAddress(origin.Hostname())
	if origin.Scheme != "https" && !(local && origin.Scheme == "http") && !lanHTTP {
		return nil, "", errors.New("AUTH_ORIGIN requires HTTPS; private LAN IPs may explicitly enable AUTH_ALLOW_INSECURE_HTTP")
	}
	config.Origin = strings.TrimSuffix(config.Origin, "/")
	config.Root = strings.TrimSuffix(config.Root, "/")
	if config.Root != "" && (!strings.HasPrefix(config.Root, "/") || strings.Contains(config.Root, "..") || strings.ContainsAny(config.Root, "?#\\")) {
		return nil, "", errors.New("invalid authentication root path")
	}
	if config.DataDir == "" {
		config.DataDir = "./data"
	}
	if err = os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, "", fmt.Errorf("auth data directory: %w", err)
	}
	var wa *webauthn.WebAuthn
	if !lanHTTP {
		wa, err = webauthn.New(&webauthn.Config{RPDisplayName: "DDNS Updater", RPID: origin.Hostname(), RPOrigins: []string{config.Origin}})
		if err != nil {
			return nil, "", fmt.Errorf("passkey configuration: %w", err)
		}
	}
	s := &Service{config: config, origin: origin, secure: origin.Scheme == "https", insecureHTTP: lanHTTP, webauthn: wa, sessions: make(map[string]session), ceremonies: make(map[string]ceremony), rates: make(map[string]rateWindow), kdf: make(chan struct{}, 2), now: time.Now}
	data, err := os.ReadFile(s.accountPath())
	if err == nil {
		var a account
		if err = json.Unmarshal(data, &a); err != nil {
			return nil, "", fmt.Errorf("invalid auth.json: %w", err)
		}
		salt, e1 := base64.RawURLEncoding.DecodeString(a.Password.Salt)
		hash, e2 := base64.RawURLEncoding.DecodeString(a.Password.Hash)
		id, e3 := base64.RawURLEncoding.DecodeString(a.ID)
		if e1 != nil || e2 != nil || e3 != nil || len(salt) != 16 || len(hash) != 32 || len(id) != 32 || a.Username == "" {
			return nil, "", errors.New("invalid administrator credentials in auth.json")
		}
		s.account = &a
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("read auth.json: %w", err)
	}
	key := ""
	if s.account == nil && (!local || !config.AllowLocalSetup || config.SetupKey != "") {
		key = config.SetupKey
		if key == "" {
			keyBytes, readErr := os.ReadFile(filepath.Join(config.DataDir, ".setup-key"))
			if readErr == nil {
				key = strings.TrimSpace(string(keyBytes))
			} else if errors.Is(readErr, os.ErrNotExist) {
				key = randomToken()
				if err = os.WriteFile(filepath.Join(config.DataDir, ".setup-key"), []byte(key), 0600); err != nil {
					return nil, "", err
				}
			} else {
				return nil, "", readErr
			}
		}
		if len(key) < 24 {
			return nil, "", errors.New("setup key must contain at least 24 characters")
		}
		s.setupKey = tokenHash(key)
	}
	return s, key, nil
}
func (s *Service) accountPath() string { return filepath.Join(s.config.DataDir, "auth.json") }
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func passwordHash(password string) passwordRecord {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return passwordRecord{Salt: base64.RawURLEncoding.EncodeToString(salt), Hash: base64.RawURLEncoding.EncodeToString(hash)}
}
func passwordMatches(record passwordRecord, password string) bool {
	salt, e1 := base64.RawURLEncoding.DecodeString(record.Salt)
	expected, e2 := base64.RawURLEncoding.DecodeString(record.Hash)
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(hash, expected) == 1
}
func (s *Service) saveLocked() (err error) {
	defer func() {
		if err != nil {
			s.storageFailed = true
		}
	}()
	data, err := json.MarshalIndent(s.account, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(s.config.DataDir, ".auth-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, s.accountPath())
}
func (s *Service) failLocked() error {
	if s.account == nil {
		return nil
	}
	now := s.now()
	if now.Sub(s.account.FailureWindow) > lockDuration {
		s.account.Failures = 0
		s.account.FailureWindow = now
	}
	s.account.Failures++
	if s.account.Failures >= maxFailures {
		s.account.LockedUntil = now.Add(lockDuration)
		s.account.Failures = 0
		s.account.FailureWindow = now
	}
	return s.saveLocked()
}
func (s *Service) resetFailuresLocked() {
	s.account.Failures = 0
	s.account.FailureWindow = time.Time{}
	s.account.LockedUntil = time.Time{}
}
func (s *Service) cookie(w http.ResponseWriter, name, value string, until time.Time) {
	maxAge := int(time.Until(until).Seconds())
	if value == "" {
		maxAge = -1
	}
	http.SetCookie(w, &http.Cookie{Name: "ddns_" + name, Value: value, Path: s.config.Root + "/", Secure: s.secure, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge, Expires: until})
}
func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie("ddns_" + name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func (s *Service) pruneLocked() {
	now := s.now()
	for key, value := range s.sessions {
		if !now.Before(value.ExpiresAt) {
			delete(s.sessions, key)
		}
	}
	for key, value := range s.ceremonies {
		if !now.Before(value.ExpiresAt) {
			delete(s.ceremonies, key)
		}
	}
	for key, value := range s.rates {
		if now.Sub(value.Start) > time.Minute {
			delete(s.rates, key)
		}
	}
}
func deviceName(r *http.Request) string {
	agent := r.UserAgent()
	browser := "Browser"
	system := ""
	switch {
	case strings.Contains(agent, "Edg/"):
		browser = "Edge"
	case strings.Contains(agent, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(agent, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(agent, "Safari/"):
		browser = "Safari"
	}
	switch {
	case strings.Contains(agent, "Android"):
		system = "Android"
	case strings.Contains(agent, "iPhone") || strings.Contains(agent, "iPad"):
		system = "iOS"
	case strings.Contains(agent, "Windows"):
		system = "Windows"
	case strings.Contains(agent, "Macintosh"):
		system = "macOS"
	case strings.Contains(agent, "Linux"):
		system = "Linux"
	}
	if system != "" {
		return browser + " · " + system
	}
	return browser
}
func (s *Service) startSessionLocked(w http.ResponseWriter, r *http.Request, remember bool) error {
	s.pruneLocked()
	if len(s.sessions) >= 256 {
		return errors.New("too many active sessions")
	}
	deviceID := ""
	delete(s.sessions, tokenHash(cookieValue(r, "session")))
	if !remember {
		oldHash := tokenHash(cookieValue(r, "device"))
		devices := s.account.Devices[:0]
		for _, d := range s.account.Devices {
			if d.Hash != oldHash && s.now().Before(d.ExpiresAt) {
				devices = append(devices, d)
			}
		}
		if len(devices) != len(s.account.Devices) {
			s.account.Devices = devices
			if err := s.saveLocked(); err != nil {
				return err
			}
		}
		s.cookie(w, "device", "", time.Unix(0, 0))
	}
	if remember {
		// Replace this browser's previous remembered grant rather than accumulating duplicates.
		oldHash := tokenHash(cookieValue(r, "device"))
		label := ""
		devices := s.account.Devices[:0]
		for _, d := range s.account.Devices {
			if d.Hash == oldHash {
				label = d.Label
			}
			if d.Hash != oldHash && s.now().Before(d.ExpiresAt) {
				devices = append(devices, d)
			}
		}
		s.account.Devices = devices
		if len(devices) >= 10 {
			return errors.New("maximum trusted devices reached")
		}
		token := randomToken()
		deviceID = randomToken()
		now := s.now()
		s.account.Devices = append(s.account.Devices, device{ID: deviceID, Hash: tokenHash(token), Name: deviceName(r), Label: label, CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(deviceLifetime)})
		if err := s.saveLocked(); err != nil {
			return err
		}
		s.cookie(w, "device", token, now.Add(deviceLifetime))
	}
	token := randomToken()
	s.sessions[tokenHash(token)] = session{ExpiresAt: s.now().Add(sessionLifetime), AuthenticatedAt: s.now(), DeviceID: deviceID}
	s.cookie(w, "session", token, s.now().Add(sessionLifetime))
	return nil
}
func (s *Service) authorisedLocked(w http.ResponseWriter, r *http.Request) (session, bool) {
	if s.account == nil || s.storageFailed {
		return session{}, false
	}
	s.pruneLocked()
	hash := tokenHash(cookieValue(r, "session"))
	if entry, ok := s.sessions[hash]; ok {
		if entry.DeviceID != "" {
			found := false
			for _, d := range s.account.Devices {
				if d.ID == entry.DeviceID && s.now().Before(d.ExpiresAt) {
					found = true
					break
				}
			}
			if !found {
				delete(s.sessions, hash)
				return session{}, false
			}
		}
		return entry, true
	}
	// A trusted-device token creates an ordinary, revocable session. It cannot
	// authorise credential changes, because it has no fresh authentication time.
	if cookieValue(r, "device") == "" {
		return session{}, false
	}
	deviceHash := tokenHash(cookieValue(r, "device"))
	for i := range s.account.Devices {
		d := &s.account.Devices[i]
		if subtle.ConstantTimeCompare([]byte(deviceHash), []byte(d.Hash)) != 1 || !s.now().Before(d.ExpiresAt) {
			continue
		}
		if len(s.sessions) >= 256 {
			return session{}, false
		}
		d.LastSeen = s.now()
		if err := s.saveLocked(); err != nil {
			return session{}, false
		}
		token := randomToken()
		entry := session{ExpiresAt: s.now().Add(sessionLifetime), DeviceID: d.ID}
		s.sessions[tokenHash(token)] = entry
		s.cookie(w, "session", token, entry.ExpiresAt)
		return entry, true
	}
	return session{}, false
}

func validPassword(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= 12 && utf8.RuneCountInString(value) <= 128 && len(value) <= 512
}
