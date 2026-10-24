package control

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

	"github.com/reactor2k3/ddns-updater/internal/data"
	"github.com/reactor2k3/ddns-updater/internal/update"
)

type testPersistence struct{}

func (testPersistence) Close() error                                           { return nil }
func (testPersistence) StoreNewIP(string, string, netip.Addr, time.Time) error { return nil }

type testRunner struct{}

type discoveryRunner struct {
	testRunner
	called chan struct{}
}

func (r *discoveryRunner) RefreshPublicIPs(context.Context) []error { close(r.called); return nil }
func TestPublicIPRefreshEndpointAndRateLimit(t *testing.T) {
	m := testManager(t)
	runner := &discoveryRunner{called: make(chan struct{})}
	m.runner = runner
	response := request(t, m, "POST", "/api/public-ips/refresh", `{}`)
	if response.Code != 202 {
		t.Fatalf("refresh not accepted: %d", response.Code)
	}
	select {
	case <-runner.called:
	case <-time.After(time.Second):
		t.Fatal("IP refresh was not dispatched")
	}
	response = request(t, m, "POST", "/api/public-ips/refresh", `{}`)
	if response.Code != 429 && response.Code != 409 {
		t.Fatalf("repeat refresh not limited: %d", response.Code)
	}
}

func (testRunner) Configure(_ context.Context, f func() error) error { return f() }
func (testRunner) ForceUpdate(context.Context) []error               { return nil }
func (testRunner) RefreshPublicIPs(context.Context) []error          { return nil }
func (testRunner) Status() update.RuntimeStatus                      { return update.RuntimeStatus{PeriodSeconds: 300} }
func testManager(t *testing.T) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"settings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	db := data.NewDatabase(nil, testPersistence{})
	m, err := New(context.Background(), path, db, testRunner{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func request(t *testing.T, m *Manager, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	m.Handler("").ServeHTTP(w, r)
	return w
}
func TestDualStackCRUDSecretsAndRestart(t *testing.T) {
	m := testManager(t)
	w := request(t, m, "POST", "/api/entries", `{"providerId":"ipv64","domain":"test.example.com","ip":"Dual Stack","enabled":true,"config":{"key":"disposable-secret"}}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "disposable-secret") {
		t.Fatal("secret in mutation response")
	}
	if len(m.db.SelectAll()) != 2 {
		t.Fatal("dual stack must compile to two provider records")
	}
	e := m.entries[0]
	id := e.ID
	snapshot := request(t, m, "GET", "/api/entries", "")
	if snapshot.Code != 200 || strings.Contains(snapshot.Body.String(), "disposable-secret") {
		t.Fatal("secret in list")
	}
	body := `{"providerId":"ipv64","domain":"test.example.com","ip":"Dual Stack","enabled":false,"revision":1,"config":{"key":""}}`
	w = request(t, m, "PUT", "/api/entries/"+id, body)
	if w.Code != 200 {
		t.Fatalf("pause: %d %s", w.Code, w.Body)
	}
	if string(m.entries[0].Config["key"]) != `"disposable-secret"` {
		t.Fatal("blank edit lost stored credential")
	}
	if len(m.db.SelectAll()) != 0 {
		t.Fatal("paused entry remained in updater database")
	}
	if request(t, m, "PUT", "/api/entries/"+id, body).Code != 409 {
		t.Fatal("stale revision accepted")
	}
	persisted, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	var config diskConfig
	if json.Unmarshal(persisted, &config) != nil || len(config.Settings) != 0 || len(config.Entries) != 1 {
		t.Fatal("wrong durable paused config")
	}
	restarted, err := New(context.Background(), m.path, data.NewDatabase(nil, testPersistence{}), testRunner{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.entries[0].ID != id || restarted.entries[0].Enabled {
		t.Fatal("paused record not preserved after restart")
	}
	w = request(t, restarted, "DELETE", "/api/entries/"+id, `{"revision":2}`)
	if w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
	restarted, err = New(context.Background(), m.path, data.NewDatabase(nil, testPersistence{}), testRunner{}, nil)
	if err != nil || len(restarted.entries) != 0 {
		t.Fatal("deleted final record returned on restart")
	}
}
func TestInvalidAndDuplicateSettingsDoNotMutate(t *testing.T) {
	m := testManager(t)
	for _, body := range []string{
		`{"providerId":"ipv64","domain":"test.example.com","ip":"IPv4","enabled":true,"config":{}}`,
		`{"providerId":"ipv64","domain":"https://bad.example.com","ip":"IPv4","enabled":true,"config":{"key":"secret"}}`,
		`{"providerId":"ipv64","domain":"test.example.com","ip":"IPv4","enabled":true,"config":{"provider":"custom","key":"secret"}}`,
		`{"providerId":"ipv64","domain":"test.example.com","ip":"bogus","enabled":true,"config":{"key":"secret"}}`,
		`{"providerId":"unknown","domain":"test.example.com","ip":"IPv4","enabled":true,"config":{}}`,
	} {
		if w := request(t, m, "POST", "/api/entries", body); w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("accepted invalid settings: %d", w.Code)
		}
	}
	if len(m.entries) != 0 || len(m.db.SelectAll()) != 0 {
		t.Fatal("invalid input mutated runtime")
	}
	valid := `{"providerId":"ipv64","domain":"test.example.com","ip":"IPv4","enabled":true,"config":{"key":"secret"}}`
	if request(t, m, "POST", "/api/entries", valid).Code != 201 {
		t.Fatal("valid settings rejected")
	}
	if request(t, m, "POST", "/api/entries", valid).Code != 409 {
		t.Fatal("duplicate domain accepted")
	}
	m.readOnly = true
	if request(t, m, "POST", "/api/entries", valid).Code != 409 {
		t.Fatal("CONFIG read-only not respected")
	}
}
func TestStorageFailureLeavesRuntimeUnchanged(t *testing.T) {
	m := testManager(t)
	m.path = filepath.Join(t.TempDir(), "nonexistent", "config.json")
	w := request(t, m, "POST", "/api/entries", `{"providerId":"ipv64","domain":"test.example.com","ip":"IPv4","enabled":true,"config":{"key":"secret"}}`)
	if w.Code < 400 || len(m.entries) != 0 || len(m.db.SelectAll()) != 0 {
		t.Fatal("failed storage modified runtime")
	}
}
