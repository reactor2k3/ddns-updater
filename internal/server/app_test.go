package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reactor2k3/ddns-updater/internal/auth"
	"github.com/reactor2k3/ddns-updater/internal/control"
	"github.com/reactor2k3/ddns-updater/internal/data"
	"github.com/reactor2k3/ddns-updater/internal/update"
)

type integrationRunner struct{}

func (integrationRunner) Configure(_ context.Context, apply func() error) error { return apply() }
func (integrationRunner) ForceUpdate(context.Context) []error                   { return nil }
func (integrationRunner) RefreshPublicIPs(context.Context) []error              { return nil }
func (integrationRunner) Status() update.RuntimeStatus {
	return update.RuntimeStatus{PeriodSeconds: 300}
}

type integrationPersistence struct{}

func (integrationPersistence) Close() error                                           { return nil }
func (integrationPersistence) StoreNewIP(string, string, netip.Addr, time.Time) error { return nil }

// Exercise the production composition: embedded assets and management endpoints
// must both remain behind authentication, including deployments below a path.
func TestAuthenticatedEmbeddedManagementAndRestart(t *testing.T) {
	for _, root := range []string{"", "/ddns"} {
		t.Run(root, func(t *testing.T) {
			t.Setenv("CONFIG", "")
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			if err := os.WriteFile(path, []byte(`{"settings":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			const origin = "https://ddns.example.com"
			newHandler := func() (http.Handler, string) {
				db := data.NewDatabase(nil, integrationPersistence{})
				manager, err := control.New(context.Background(), path, db, integrationRunner{}, nil)
				if err != nil {
					t.Fatal(err)
				}
				access, key, err := auth.New(auth.Config{Origin: origin, Root: root, DataDir: dir})
				if err != nil {
					t.Fatal(err)
				}
				return access.Wrap(AppHandler(root, manager)), key
			}
			handler, key := newHandler()
			request := func(method, route, body string, cookies []*http.Cookie, mutationOrigin string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, origin+root+route, strings.NewReader(body))
				if mutationOrigin != "" {
					r.Header.Set("Origin", mutationOrigin)
				}
				for _, cookie := range cookies {
					r.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			if request("GET", "/api/entries", "", nil, "").Code != 401 {
				t.Fatal("anonymous API access")
			}
			if w := request("GET", "/", "", nil, ""); w.Code != 303 || w.Header().Get("Location") != root+"/setup" {
				t.Fatal("missing initial setup redirect")
			}
			body, _ := json.Marshal(map[string]any{"username": "admin", "password": "test-only-password-123", "setupKey": key, "remember": true})
			if request("POST", "/api/auth/setup", string(body), nil, "https://evil.example").Code != 403 {
				t.Fatal("cross-origin setup accepted")
			}
			setup := request("POST", "/api/auth/setup", string(body), nil, origin)
			if setup.Code != 201 {
				t.Fatalf("setup: %d %s", setup.Code, setup.Body)
			}
			cookies := setup.Result().Cookies()
			for _, cookie := range cookies {
				if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != root+"/" {
					t.Fatal("unsafe cookie")
				}
			}
			if request("POST", "/api/auth/setup", string(body), cookies, origin).Code != 409 {
				t.Fatal("second account setup accepted")
			}
			page := request("GET", "/", "", cookies, "")
			if page.Code != 200 || !strings.Contains(page.Body.String(), "entry-dialog") || page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("embedded UI or security headers missing")
			}
			for _, route := range []string{"/config.json", "/auth.json", "/.setup-key", "/api/update"} {
				w := request("GET", route, "", cookies, "")
				if w.Code != 404 && w.Code != 405 {
					t.Fatalf("unexpected public route %s: %d", route, w.Code)
				}
			}
			entry := `{"providerId":"ipv64","domain":"test.example.com","ip":"Dual Stack","enabled":false,"config":{"key":"integration-only-secret"}}`
			if request("POST", "/api/entries", entry, cookies, "").Code != 403 {
				t.Fatal("mutation without origin accepted")
			}
			created := request("POST", "/api/entries", entry, cookies, origin)
			if created.Code != 201 || strings.Contains(created.Body.String(), "integration-only-secret") {
				t.Fatalf("create failed or secret exposed: %d", created.Code)
			}
			if request("POST", "/api/entries", entry, cookies, origin).Code != 409 {
				t.Fatal("duplicate accepted")
			}
			handler, _ = newHandler()
			list := request("GET", "/api/entries", "", cookies, "")
			if list.Code != 200 || !strings.Contains(list.Body.String(), `"status":"paused"`) || strings.Contains(list.Body.String(), "integration-only-secret") {
				t.Fatal("remembered session, paused entry or secret redaction lost on restart")
			}
			if request("GET", "/api/entries", "", nil, "").Code != 401 {
				t.Fatal("restart removed authentication")
			}
		})
	}
}
