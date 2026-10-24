package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
)

const testPassword = "only-a-disposable-test-password-72"

type testClient struct {
	s       *Service
	cookies map[string]*http.Cookie
	origin  string
	ip      string
}

func testService(t *testing.T) *Service {
	t.Helper()
	s, _, err := New(Config{Origin: "http://localhost", DataDir: t.TempDir(), AllowLocalSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func client(s *Service) *testClient {
	return &testClient{s: s, cookies: map[string]*http.Cookie{}, origin: s.config.Origin, ip: "127.0.0.1:12345"}
}
func (c *testClient) request(t *testing.T, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if value != nil {
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, c.s.config.Origin+c.s.config.Root+path, bytes.NewReader(body))
	r.RemoteAddr = c.ip
	r.Header.Set("Origin", c.origin)
	r.Header.Set("Content-Type", "application/json")
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.s.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("private workspace")) })).ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
		} else {
			c.cookies[cookie.Name] = cookie
		}
	}
	return w
}
func requireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d want=%d body=%s", w.Code, want, w.Body.String())
	}
}
func setupClient(t *testing.T, c *testClient, remember bool) {
	t.Helper()
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "admin", "password": testPassword, "remember": remember}), 201)
}
func TestSingleAccountAndSession(t *testing.T) {
	s := testService(t)
	c := client(s)
	w := c.request(t, "GET", "/", nil)
	requireStatus(t, w, 303)
	if w.Header().Get("Location") != "/setup" {
		t.Fatal("first request must redirect to setup")
	}
	setupClient(t, c, false)
	requireStatus(t, c.request(t, "GET", "/", nil), 200)
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "other", "password": testPassword}), 409)
	raw, err := os.ReadFile(s.accountPath())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(testPassword)) {
		t.Fatal("password stored in plaintext")
	}
	cookie := c.cookies["ddns_session"]
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge > int(sessionLifetime.Seconds()) {
		t.Fatal("session cookie attributes are unsafe")
	}
	requireStatus(t, c.request(t, "POST", "/api/auth/logout", map[string]any{}), 200)
	requireStatus(t, c.request(t, "GET", "/", nil), 303)
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword}), 200)
	old := c.cookies["ddns_session"].Value
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword}), 200)
	if c.cookies["ddns_session"].Value == old {
		t.Fatal("session not rotated")
	}
	if _, exists := s.sessions[tokenHash(old)]; exists {
		t.Fatal("old session still authorised after reauthentication")
	}
	s.now = func() time.Time { return time.Now().Add(sessionLifetime + time.Minute) }
	requireStatus(t, c.request(t, "GET", "/", nil), 303)
}
func TestPersistentAccountLockAndExpiry(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, false)
	for attempt := 1; attempt <= 5; attempt++ {
		want := 401
		if attempt == 5 {
			want = 429
		}
		requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": "wrong"}), want)
	}
	s2, _, err := New(s.config)
	if err != nil {
		t.Fatal(err)
	}
	other := client(s2)
	requireStatus(t, other.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword}), 429)
	s2.now = func() time.Time { return time.Now().Add(16 * time.Minute) }
	requireStatus(t, other.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword}), 200)
	if !s2.account.LockedUntil.IsZero() || s2.account.Failures != 0 {
		t.Fatal("successful login did not reset failures")
	}
}

func TestDeviceNamesPersistAndRequireAuthorisation(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, true)
	id := s.account.Devices[0].ID
	body := map[string]any{"id": id, "name": "  Büro-Laptop 🖥️  "}
	requireStatus(t, client(s).request(t, "POST", "/api/auth/devices/rename", body), 401)
	c.origin = "https://evil.example"
	requireStatus(t, c.request(t, "POST", "/api/auth/devices/rename", body), 403)
	c.origin = s.config.Origin
	for _, invalid := range []string{strings.Repeat("x", 65), "bad\nname", "bad\x00name"} {
		requireStatus(t, c.request(t, "POST", "/api/auth/devices/rename", map[string]any{"id": id, "name": invalid}), 400)
	}
	requireStatus(t, c.request(t, "POST", "/api/auth/devices/rename", map[string]any{"id": "unknown", "name": "Laptop"}), 404)
	requireStatus(t, c.request(t, "POST", "/api/auth/devices/rename", body), 200)
	if s.account.Devices[0].Label != "Büro-Laptop 🖥️" || s.account.Devices[0].Name == "" {
		t.Fatal("name or browser identity lost")
	}
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword, "remember": true}), 200)
	if s.account.Devices[0].Label != "Büro-Laptop 🖥️" {
		t.Fatal("reauthentication lost device name")
	}
	restarted, _, err := New(s.config)
	if err != nil {
		t.Fatal(err)
	}
	c.s = restarted
	response := c.request(t, "GET", "/api/auth/devices", nil)
	requireStatus(t, response, 200)
	if !strings.Contains(response.Body.String(), "Büro-Laptop") {
		t.Fatal("name not persisted")
	}
	id = restarted.account.Devices[0].ID
	requireStatus(t, c.request(t, "POST", "/api/auth/devices/rename", map[string]any{"id": id, "name": ""}), 200)
	if restarted.account.Devices[0].Label != "" {
		t.Fatal("empty name did not restore default")
	}
}
func TestOriginRateLimitAndLocalSetup(t *testing.T) {
	s := testService(t)
	c := client(s)
	c.origin = "https://attacker.example"
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "admin", "password": testPassword}), 403)
	c.origin = s.config.Origin
	c.ip = "198.51.100.9:1234"
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "admin", "password": testPassword}), 403)
	c.ip = "127.0.0.1:1234"
	for i := 0; i < 20; i++ {
		requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{}), 409)
	}
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{}), 429)
	if _, _, err := New(Config{Origin: "http://ddns.example", DataDir: t.TempDir()}); err == nil {
		t.Fatal("insecure external origin accepted")
	}
}
func TestSetupKeySecureCookiesAndRoot(t *testing.T) {
	s, key, err := New(Config{Origin: "https://ddns.example", DataDir: t.TempDir(), Root: "/ddns"})
	if err != nil {
		t.Fatal(err)
	}
	if len(key) < 24 {
		t.Fatal("setup key missing")
	}
	c := client(s)
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "admin", "password": testPassword}), 403)
	requireStatus(t, c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "admin", "password": testPassword, "setupKey": key, "remember": true}), 201)
	for _, name := range []string{"ddns_session", "ddns_device"} {
		cookie := c.cookies[name]
		if cookie == nil || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/ddns/" || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatal("unsafe cookie", name)
		}
	}
	requireStatus(t, c.request(t, "GET", "/", nil), 200)
	if _, err = os.Stat(filepath.Join(s.config.DataDir, ".setup-key")); !os.IsNotExist(err) {
		t.Fatal("used setup key was not removed")
	}
}
func TestTrustedDevicePersistsAndCanBeRevoked(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, true)
	deviceToken := c.cookies["ddns_device"].Value
	raw, _ := os.ReadFile(s.accountPath())
	if bytes.Contains(raw, []byte(deviceToken)) {
		t.Fatal("trusted device token stored in plaintext")
	}
	s2, _, err := New(s.config)
	if err != nil {
		t.Fatal(err)
	}
	c.s = s2
	delete(c.cookies, "ddns_session")
	requireStatus(t, c.request(t, "GET", "/", nil), 200)
	requireStatus(t, c.request(t, "POST", "/api/auth/passkey/register/begin", map[string]any{}), 403)
	w := c.request(t, "GET", "/api/auth/devices", nil)
	requireStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "hash") || strings.Contains(w.Body.String(), deviceToken) {
		t.Fatal("device secret exposed")
	}
	manager := client(s2)
	requireStatus(t, manager.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword}), 200)
	requireStatus(t, manager.request(t, "POST", "/api/auth/devices/revoke", map[string]any{"id": s2.account.Devices[0].ID}), 200)
	requireStatus(t, c.request(t, "GET", "/", nil), 303)
	setupDevice := client(s2)
	requireStatus(t, setupDevice.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword, "remember": true}), 200)
	requireStatus(t, manager.request(t, "POST", "/api/auth/devices/revoke", map[string]any{"all": true}), 200)
	requireStatus(t, manager.request(t, "GET", "/", nil), 303)
	requireStatus(t, setupDevice.request(t, "GET", "/", nil), 303)
}
func TestMalformedPersistentAccountFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{"username":"admin"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := New(Config{Origin: "http://localhost", DataDir: dir, AllowLocalSetup: true}); err == nil {
		t.Fatal("malformed administrator state enabled a new setup")
	}
}

func TestTrustedDeviceExpiresAfterThirtyDays(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, true)
	s.now = func() time.Time { return time.Now().Add(31 * 24 * time.Hour) }
	requireStatus(t, c.request(t, "GET", "/", nil), 303)
}

func TestStorageFailureFailsClosed(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, false)
	if err := os.Remove(s.accountPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.accountPath(), 0700); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": "wrong"}), 500)
	requireStatus(t, c.request(t, "POST", "/api/auth/login", map[string]any{"username": "admin", "password": testPassword}), 503)
	requireStatus(t, c.request(t, "GET", "/", nil), 303)
}

func TestOnlyOneConcurrentSetupSucceeds(t *testing.T) {
	s := testService(t)
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			c := client(s)
			results <- c.request(t, "POST", "/api/auth/setup", map[string]any{"username": "admin", "password": testPassword}).Code
		}()
	}
	a, b := <-results, <-results
	if !((a == 201 && b == 409) || (a == 409 && b == 201)) {
		t.Fatalf("concurrent setup statuses: %d %d", a, b)
	}
}
func b64(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }

type softKey struct {
	key  *ecdsa.PrivateKey
	id   []byte
	user string
}

func registerSoftKey(t *testing.T, c *testClient) *softKey {
	t.Helper()
	w := c.request(t, "POST", "/api/auth/passkey/register/begin", map[string]any{})
	requireStatus(t, w, 200)
	var options struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			User      struct {
				ID string `json:"id"`
			} `json:"user"`
			AuthenticatorSelection struct {
				UserVerification string `json:"userVerification"`
			} `json:"authenticatorSelection"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if options.PublicKey.AuthenticatorSelection.UserVerification != "required" {
		t.Fatal("user verification not required")
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	_, _ = rand.Read(id)
	cose, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: private.X.FillBytes(make([]byte, 32)), -3: private.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte(c.s.origin.Hostname()))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, 0x45, 0, 0, 0, 0)
	authData = append(authData, make([]byte, 16)...)
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(id)))
	authData = append(authData, id...)
	authData = append(authData, cose...)
	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"type": "webauthn.create", "challenge": options.PublicKey.Challenge, "origin": c.s.config.Origin, "crossOrigin": false})
	credential := map[string]any{"id": b64(id), "rawId": b64(id), "type": "public-key", "response": map[string]any{"clientDataJSON": b64(data), "attestationObject": b64(attestation), "transports": []string{"internal"}}, "clientExtensionResults": map[string]any{}}
	requireStatus(t, c.request(t, "POST", "/api/auth/passkey/register/finish", credential), 201)
	return &softKey{key: private, id: id, user: options.PublicKey.User.ID}
}
func assertion(t *testing.T, c *testClient, key *softKey, origin string, flags byte, counter uint32) map[string]any {
	t.Helper()
	w := c.request(t, "POST", "/api/auth/passkey/login/begin", map[string]any{"remember": false})
	requireStatus(t, w, 200)
	var options struct {
		PublicKey struct {
			Challenge        string `json:"challenge"`
			UserVerification string `json:"userVerification"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if options.PublicKey.UserVerification != "required" {
		t.Fatal("user verification not required")
	}
	rpHash := sha256.Sum256([]byte(c.s.origin.Hostname()))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, flags)
	authData = binary.BigEndian.AppendUint32(authData, counter)
	data, _ := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": options.PublicKey.Challenge, "origin": origin, "crossOrigin": false})
	hash := sha256.Sum256(data)
	signed := append(append([]byte{}, authData...), hash[:]...)
	digest := sha256.Sum256(signed)
	signature, err := ecdsa.SignASN1(rand.Reader, key.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"id": b64(key.id), "rawId": b64(key.id), "type": "public-key", "response": map[string]any{"clientDataJSON": b64(data), "authenticatorData": b64(authData), "signature": b64(signature), "userHandle": key.user}, "clientExtensionResults": map[string]any{}}
}
func TestPasskeyVerificationAndReplayProtection(t *testing.T) {
	s := testService(t)
	c := client(s)
	setupClient(t, c, false)
	key := registerSoftKey(t, c)
	requireStatus(t, c.request(t, "POST", "/api/auth/logout", map[string]any{}), 200)
	wrongOrigin := assertion(t, c, key, "https://attacker.example", 0x05, 1)
	requireStatus(t, c.request(t, "POST", "/api/auth/passkey/login/finish", wrongOrigin), 401)
	noUV := assertion(t, c, key, s.config.Origin, 0x01, 1)
	requireStatus(t, c.request(t, "POST", "/api/auth/passkey/login/finish", noUV), 401)
	good := assertion(t, c, key, s.config.Origin, 0x05, 1)
	requireStatus(t, c.request(t, "POST", "/api/auth/passkey/login/finish", good), 200)
	requireStatus(t, c.request(t, "GET", "/", nil), 200)
	requireStatus(t, c.request(t, "POST", "/api/auth/passkey/login/finish", good), 400)
	if s.account.Credentials[0].Authenticator.SignCount != 1 {
		t.Fatal("signature counter not persisted")
	}
	s2, _, err := New(s.config)
	if err != nil {
		t.Fatal(err)
	}
	c2 := client(s2)
	good2 := assertion(t, c2, key, s2.config.Origin, 0x05, 2)
	requireStatus(t, c2.request(t, "POST", "/api/auth/passkey/login/finish", good2), 200)
	if s2.account.Credentials[0].Authenticator.SignCount != 2 {
		t.Fatal("persistent passkey did not work after restart")
	}
	cloned := assertion(t, c2, key, s2.config.Origin, 0x05, 2)
	requireStatus(t, c2.request(t, "POST", "/api/auth/passkey/login/finish", cloned), 401)
	tampered := assertion(t, c2, key, s2.config.Origin, 0x05, 3)
	response := tampered["response"].(map[string]any)
	response["signature"] = b64([]byte("not a valid signature"))
	requireStatus(t, c2.request(t, "POST", "/api/auth/passkey/login/finish", tampered), 401)
	expired := assertion(t, c2, key, s2.config.Origin, 0x05, 3)
	s2.now = func() time.Time { return time.Now().Add(6 * time.Minute) }
	requireStatus(t, c2.request(t, "POST", "/api/auth/passkey/login/finish", expired), 400)
}
