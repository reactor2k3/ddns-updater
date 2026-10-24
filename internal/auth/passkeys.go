package auth

import (
	"bytes"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (s *Service) beginCeremonyLocked(w http.ResponseWriter, r *http.Request, data *webauthn.SessionData, kind string, remember bool) bool {
	s.pruneLocked()
	if len(s.ceremonies) >= 256 {
		problem(w, 429, "RATE_LIMITED")
		return false
	}
	token := randomToken()
	data.Expires = s.now().Add(ceremonyLifetime)
	s.ceremonies[tokenHash(token)] = ceremony{Data: *data, ExpiresAt: data.Expires, Kind: kind, Remember: remember, SessionHash: tokenHash(cookieValue(r, "session"))}
	s.cookie(w, "challenge", token, data.Expires)
	return true
}
func (s *Service) consumeCeremonyLocked(w http.ResponseWriter, r *http.Request, kind string) (ceremony, bool) {
	hash := tokenHash(cookieValue(r, "challenge"))
	value, ok := s.ceremonies[hash]
	delete(s.ceremonies, hash)
	s.cookie(w, "challenge", "", s.now())
	if !ok || value.Kind != kind || !s.now().Before(value.ExpiresAt) {
		problem(w, 400, "CHALLENGE_EXPIRED")
		return ceremony{}, false
	}
	if kind == "registration" && value.SessionHash != tokenHash(cookieValue(r, "session")) {
		problem(w, 403, "CHALLENGE_INVALID")
		return ceremony{}, false
	}
	return value, true
}
func (s *Service) registerBegin(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.authorisedLocked(w, r)
	if !ok {
		problem(w, 401, "AUTH_REQUIRED")
		return
	}
	if entry.AuthenticatedAt.IsZero() || s.now().Sub(entry.AuthenticatedAt) >= ceremonyLifetime {
		problem(w, 403, "REAUTH_REQUIRED")
		return
	}
	if len(s.account.Credentials) >= 10 {
		problem(w, 409, "PASSKEY_LIMIT")
		return
	}
	options, data, err := s.webauthn.BeginRegistration(*s.account, webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, UserVerification: protocol.VerificationRequired}))
	if err != nil {
		problem(w, 500, "PASSKEY_UNAVAILABLE")
		return
	}
	if !s.beginCeremonyLocked(w, r, data, "registration", false) {
		return
	}
	reply(w, 200, options)
}
func (s *Service) registerFinish(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.authorisedLocked(w, r)
	if !ok {
		problem(w, 401, "AUTH_REQUIRED")
		return
	}
	if entry.AuthenticatedAt.IsZero() || s.now().Sub(entry.AuthenticatedAt) >= ceremonyLifetime {
		problem(w, 403, "REAUTH_REQUIRED")
		return
	}
	challenge, ok := s.consumeCeremonyLocked(w, r, "registration")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	credential, err := s.webauthn.FinishRegistration(*s.account, challenge.Data, r)
	if err != nil {
		problem(w, 400, "PASSKEY_INVALID")
		return
	}
	for _, existing := range s.account.Credentials {
		if bytes.Equal(existing.ID, credential.ID) {
			problem(w, 409, "PASSKEY_EXISTS")
			return
		}
	}
	s.account.Credentials = append(s.account.Credentials, *credential)
	if err = s.saveLocked(); err != nil {
		problem(w, 500, "STORAGE_ERROR")
		return
	}
	reply(w, 201, map[string]bool{"ok": true})
}
func (s *Service) loginBegin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Remember bool `json:"remember"`
	}
	if !decode(w, r, &input) {
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
	if len(s.account.Credentials) == 0 {
		problem(w, 409, "NO_PASSKEY")
		return
	}
	options, data, err := s.webauthn.BeginLogin(*s.account, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		problem(w, 500, "PASSKEY_UNAVAILABLE")
		return
	}
	if !s.beginCeremonyLocked(w, r, data, "login", input.Remember) {
		return
	}
	reply(w, 200, options)
}
func (s *Service) loginFinish(w http.ResponseWriter, r *http.Request) {
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
	challenge, ok := s.consumeCeremonyLocked(w, r, "login")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	credential, err := s.webauthn.FinishLogin(*s.account, challenge.Data, r)
	if err != nil || credential.Authenticator.CloneWarning {
		if saveErr := s.failLocked(); saveErr != nil {
			problem(w, 500, "STORAGE_ERROR")
			return
		}
		problem(w, 401, "LOGIN_FAILED")
		return
	}
	found := false
	for i := range s.account.Credentials {
		if bytes.Equal(s.account.Credentials[i].ID, credential.ID) {
			s.account.Credentials[i] = *credential
			found = true
			break
		}
	}
	if !found {
		problem(w, 401, "LOGIN_FAILED")
		return
	}
	s.resetFailuresLocked()
	if err = s.saveLocked(); err != nil {
		problem(w, 500, "STORAGE_ERROR")
		return
	}
	if err = s.startSessionLocked(w, r, challenge.Remember); err != nil {
		problem(w, 500, "DEVICE_LIMIT_OR_STORAGE")
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
