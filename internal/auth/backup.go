package auth

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/reactor2k3/ddns-updater/internal/backup"
)

func validBackupAccount(raw []byte) bool {
	var a account
	if json.Unmarshal(raw, &a) != nil {
		return false
	}
	salt, e1 := base64.RawURLEncoding.DecodeString(a.Password.Salt)
	hash, e2 := base64.RawURLEncoding.DecodeString(a.Password.Hash)
	id, e3 := base64.RawURLEncoding.DecodeString(a.ID)
	if e1 != nil || e2 != nil || e3 != nil || len(salt) != 16 || len(hash) != 32 || len(id) != 32 || !usernamePattern.MatchString(a.Username) || len(a.Devices) > 10 || len(a.Credentials) > 10 {
		return false
	}
	seen := map[string]bool{}
	for _, d := range a.Devices {
		id, idErr := base64.RawURLEncoding.DecodeString(d.ID)
		hash, hashErr := base64.RawURLEncoding.DecodeString(d.Hash)
		if idErr != nil || hashErr != nil || len(id) != 32 || len(hash) != 32 || d.Hash == tokenHash("") || seen[d.ID] || seen[d.Hash] || utf8.RuneCountInString(d.Label) > 64 || len(d.Name) > 256 || d.CreatedAt.IsZero() || !d.ExpiresAt.After(d.CreatedAt) {
			return false
		}
		for _, r := range d.Label {
			if unicode.IsControl(r) {
				return false
			}
		}
		seen[d.ID], seen[d.Hash] = true, true
	}
	seen = map[string]bool{}
	for _, c := range a.Credentials {
		id := base64.RawURLEncoding.EncodeToString(c.ID)
		if len(c.ID) == 0 || len(c.ID) > 1024 || len(c.PublicKey) == 0 || len(c.PublicKey) > 8192 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (s *Service) backupRoute(w http.ResponseWriter, r *http.Request, route string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if !s.rateAllowed(r) {
		problem(w, 429, "RATE_LIMITED")
		return
	}
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
	b := s.config.Backup
	if b == nil {
		problem(w, 503, "BACKUP_UNAVAILABLE")
		return
	}
	switch route {
	case "backup/download":
		raw, err := b.Download(r.Context())
		if err != nil {
			problem(w, 500, "BACKUP_FAILED")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="ddns-updater-backup-`+s.now().UTC().Format("20060102-150405")+`.zip"`)
		w.WriteHeader(200)
		_, _ = w.Write(raw)
	case "backup/restore":
		r.Body = http.MaxBytesReader(w, r.Body, backup.MaxUploadSize)
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			problem(w, 413, "BACKUP_TOO_LARGE")
			return
		}
		files, err := backup.ReadUpload(raw)
		if err != nil || !validBackupAccount(files["auth.json"]) {
			problem(w, 400, "BACKUP_INVALID")
			return
		}
		if err = b.Prepare(files); err != nil {
			problem(w, 400, "BACKUP_INVALID")
			return
		}
		if err = b.RequestRestore(files); err != nil {
			problem(w, 409, "BACKUP_BUSY")
			return
		}
		s.storageFailed = true
		s.cookie(w, "session", "", time.Unix(0, 0))
		s.cookie(w, "device", "", time.Unix(0, 0))
		reply(w, 202, map[string]bool{"ok": true})
	default:
		http.NotFound(w, r)
	}
}
