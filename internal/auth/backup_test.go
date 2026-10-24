package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reactor2k3/ddns-updater/internal/backup"
)

func TestBackupRequiresFreshAuthenticatedSameOriginSession(t *testing.T) {
	s := testService(t)
	c := client(s)
	dir := s.config.DataDir
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"settings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	queued := false
	s.config.Backup = &backup.Web{DataDir: dir, ConfigPath: path, ValidateConfig: func(b []byte) ([]byte, error) { return b, nil }, ValidateHistory: func([]byte) error { return nil }, RequestRestore: func(map[string][]byte) error { queued = true; return nil }}
	requireStatus(t, c.request(t, "POST", "/api/auth/backup/download", map[string]any{}), 401)
	setupClient(t, c, true)
	c.origin = "https://evil.example"
	requireStatus(t, c.request(t, "POST", "/api/auth/backup/download", map[string]any{}), 403)
	c.origin = s.config.Origin
	download := c.request(t, "POST", "/api/auth/backup/download", map[string]any{})
	requireStatus(t, download, 200)
	if download.Header().Get("Content-Type") != "application/zip" || download.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe download headers")
	}
	files, err := backup.ReadUpload(download.Body.Bytes())
	if err != nil || !validBackupAccount(files["auth.json"]) {
		t.Fatal("account missing from download", err)
	}
	now := s.now()
	s.now = func() time.Time { return now.Add(6 * time.Minute) }
	requireStatus(t, c.request(t, "POST", "/api/auth/backup/download", map[string]any{}), 403)
	s.now = func() time.Time { return now }
	request := func(raw []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", s.config.Origin+"/api/auth/backup/restore", bytes.NewReader(raw))
		r.Header.Set("Origin", s.config.Origin)
		r.RemoteAddr = c.ip
		for _, cookie := range c.cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.Wrap(http.NotFoundHandler()).ServeHTTP(w, r)
		return w
	}
	requireStatus(t, request([]byte("not a ZIP")), 400)
	if queued {
		t.Fatal("invalid restore queued")
	}
	requireStatus(t, request(download.Body.Bytes()), 202)
	if !queued {
		t.Fatal("valid restore not queued")
	}
	requireStatus(t, c.request(t, "GET", "/api/auth/state", nil), 503)
	if _, err = s.config.Backup.Download(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestBackupRejectsEmptyTokenGrants(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, true)
	raw, err := os.ReadFile(s.accountPath())
	if err != nil || !validBackupAccount(raw) {
		t.Fatal("normal account rejected", err)
	}
	s.account.Devices[0].Hash = tokenHash("")
	raw, _ = json.Marshal(s.account)
	if validBackupAccount(raw) {
		t.Fatal("empty-token device grant accepted from backup")
	}
	requireStatus(t, client(s).request(t, "GET", "/api/entries", nil), 401)
}
