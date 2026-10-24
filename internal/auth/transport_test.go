package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransportPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, origin             string
		optIn, allowed, passkeys bool
	}{
		{"default LAN rejected", "http://192.168.1.100:8095", false, false, false},
		{"explicit LAN", "http://192.168.1.100:8095", true, true, false},
		{"private IPv6", "http://[fd00::23]:8095", true, true, false},
		{"public IPv4 rejected", "http://8.8.8.8", true, false, false},
		{"public IPv6 rejected", "http://[2606:4700:4700::1111]", true, false, false},
		{"hostname rejected", "http://ddns.example.com", true, false, false},
		{"loopback unchanged", "http://localhost", false, true, true},
		{"HTTPS unchanged", "https://ddns.example.com", false, true, true},
		{"HTTPS with opt-in", "https://ddns.example.com", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, err := New(Config{Origin: tc.origin, DataDir: t.TempDir(), AllowInsecureHTTP: tc.optIn})
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v err=%v", tc.allowed, err)
			}
			if err == nil && (s.webauthn != nil) != tc.passkeys {
				t.Fatal("unexpected passkey availability")
			}
		})
	}
}

func TestLANHTTPAuthenticationAndBoundaries(t *testing.T) {
	s, key, err := New(Config{Origin: "http://192.168.1.100:8095", DataDir: t.TempDir(), AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	if key == "" {
		t.Fatal("LAN setup must require a generated key")
	}
	c := client(s)
	c.ip = "192.168.1.20:54321"
	w := c.request(t, "GET", "/api/auth/state", nil)
	requireStatus(t, w, 200)
	var state map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state["insecureHTTP"] != true || state["passkeysAvailable"] != false {
		t.Fatal("incorrect HTTP state")
	}
	requireStatus(t, c.request(t, "GET", "/api/private", nil), 401)
	setup := map[string]any{"username": "admin", "password": testPassword}
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", setup), 403)
	setup["setupKey"] = key
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", setup), 201)
	cookie := c.cookies["ddns_session"]
	if cookie == nil || cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("incorrect HTTP cookie attributes")
	}
	requireStatus(t, c.request(t, "GET", "/api/private", nil), 200)
	for _, route := range []string{"register/begin", "register/finish", "login/begin", "login/finish"} {
		w := c.request(t, "POST", "/api/auth/passkey/"+route, map[string]any{})
		requireStatus(t, w, 403)
		if !strings.Contains(w.Body.String(), "PASSKEY_REQUIRES_HTTPS") {
			t.Fatal("passkey endpoint not explicitly disabled")
		}
	}
	c.origin = "http://attacker.example"
	requireStatus(t, c.request(t, "POST", "/api/auth/logout", map[string]any{}), 403)
	c.origin = s.config.Origin
	requireStatus(t, c.request(t, "POST", "/api/auth/logout", map[string]any{}), 200)
	login := map[string]any{"username": "admin", "password": testPassword}
	requireStatus(t, c.request(t, "POST", "/api/auth/login", login), 200)
	for _, peer := range []string{"8.8.8.8:1234", "[2606:4700:4700::1111]:1234", "invalid"} {
		c.ip = peer
		requireStatus(t, c.request(t, "GET", "/api/auth/state", nil), 403)
	}
	// Neither forwarded peer addresses nor a spoofed Host may bypass the boundary.
	r := httptest.NewRequest("GET", s.config.Origin+"/api/auth/state", nil)
	r.RemoteAddr = "8.8.8.8:1234"
	r.Header.Set("X-Forwarded-For", "192.168.1.20")
	w = httptest.NewRecorder()
	s.Wrap(http.NotFoundHandler()).ServeHTTP(w, r)
	requireStatus(t, w, 403)
	c.ip = "192.168.1.20:54321"
	requireStatus(t, c.request(t, "POST", "/api/auth/logout", map[string]any{}), 200)
	for i := 0; i < maxFailures-1; i++ {
		requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": "incorrect-password"}), 401)
	}
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": "incorrect-password"}), 429)
	requireStatus(t, c.request(t, "POST", "/api/auth/login", login), 429)
}
