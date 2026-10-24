package update

import (
	"context"
	"encoding/json"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/reactor2k3/ddns-updater/internal/data"
	"github.com/reactor2k3/ddns-updater/internal/healthchecksio"
	"github.com/reactor2k3/ddns-updater/internal/params"
	"github.com/reactor2k3/ddns-updater/internal/records"
)

type fixturePersistence struct{}

func (fixturePersistence) Close() error                                           { return nil }
func (fixturePersistence) StoreNewIP(string, string, netip.Addr, time.Time) error { return nil }

type fixtureLogger struct{}

func (fixtureLogger) Info(string)  {}
func (fixtureLogger) Warn(string)  {}
func (fixtureLogger) Error(string) {}
func (fixtureLogger) Debug(string) {}

type fixtureHeartbeat struct{}

func (fixtureHeartbeat) Ping(context.Context, healthchecksio.State) error { return nil }

type fixtureIPs struct{}

func (fixtureIPs) IP(context.Context) (netip.Addr, error) {
	return netip.MustParseAddr("203.0.113.42"), nil
}
func (fixtureIPs) IP4(context.Context) (netip.Addr, error) {
	return netip.MustParseAddr("203.0.113.42"), nil
}
func (fixtureIPs) IP6(context.Context) (netip.Addr, error) {
	return netip.MustParseAddr("2001:db8::42"), nil
}

type fixtureLookup struct{}

func (fixtureLookup) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return nil, nil
}

type matchingLookup struct{}

func (matchingLookup) LookupNetIP(_ context.Context, network, _ string) ([]netip.Addr, error) {
	if network == "ip4" {
		return []netip.Addr{netip.MustParseAddr("203.0.113.42")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("2001:db8::42")}, nil
}

func TestUnchangedDualStackDoesNotCallProvider(t *testing.T) {
	var initial []records.Record
	for _, version := range []string{"ipv4", "ipv6"} {
		b, _ := json.Marshal(map[string]any{"settings": []any{map[string]string{"provider": "ipv64", "domain": "test.example.com", "ip_version": version, "key": "disposable"}}})
		ps, _, err := params.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		initial = append(initial, records.New(ps[0], nil))
	}
	db := data.NewDatabase(initial, fixturePersistence{})
	release := make(chan struct{})
	close(release)
	u := &blockingUpdater{entered: make(chan struct{}), release: release}
	s := NewService(db, u, fixtureIPs{}, time.Hour, 5*time.Minute, fixtureLogger{}, matchingLookup{}, time.Now, fixtureHeartbeat{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	for range 3 {
		if errs := s.ForceUpdate(ctx); len(errs) != 0 {
			t.Fatal(errs)
		}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.ips) != 0 {
		t.Fatal("provider was called although both DNS addresses were already correct")
	}
}

type blockingUpdater struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	ips     []netip.Addr
}

func (u *blockingUpdater) Update(ctx context.Context, _ uint, ip netip.Addr) error {
	u.once.Do(func() { close(u.entered) })
	select {
	case <-u.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	u.mu.Lock()
	u.ips = append(u.ips, ip)
	u.mu.Unlock()
	return nil
}
func TestDualStackUpdatesAndConfigurationAreSerialized(t *testing.T) {
	var initial []records.Record
	for _, version := range []string{"ipv4", "ipv6"} {
		b, _ := json.Marshal(map[string]any{"settings": []any{map[string]string{"provider": "ipv64", "domain": "test.example.com", "ip_version": version, "key": "disposable"}}})
		ps, _, err := params.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		initial = append(initial, records.New(ps[0], nil))
	}
	db := data.NewDatabase(initial, fixturePersistence{})
	u := &blockingUpdater{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewService(db, u, fixtureIPs{}, time.Hour, 0, fixtureLogger{}, fixtureLookup{}, time.Now, fixtureHeartbeat{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	updated := make(chan []error, 1)
	go func() { updated <- s.ForceUpdate(ctx) }()
	select {
	case <-u.entered:
	case <-ctx.Done():
		t.Fatal("update did not start")
	}
	applied := make(chan struct{})
	configured := make(chan error, 1)
	go func() { configured <- s.Configure(ctx, func() error { db.Replace(nil); close(applied); return nil }) }()
	select {
	case <-applied:
		t.Fatal("configuration replaced records during an update")
	case <-time.After(25 * time.Millisecond):
	}
	close(u.release)
	if errs := <-updated; len(errs) != 0 {
		t.Fatal(errs)
	}
	if err := <-configured; err != nil {
		t.Fatal(err)
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.ips) != 2 || u.ips[0].Is4() == u.ips[1].Is4() {
		t.Fatal("both IP families must be updated exactly once")
	}
	if len(db.SelectAll()) != 0 {
		t.Fatal("configuration was not applied")
	}
	state := s.Status()
	if state.Running || state.IPv4 != "203.0.113.42" || state.IPv6 != "2001:db8::42" {
		t.Fatalf("incorrect runtime status: %+v", state)
	}
}
func TestCanceledForceUpdateDoesNotBlockBeforeStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewService(nil, nil, nil, time.Hour, 0, nil, nil, time.Now, nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if len(s.ForceUpdate(ctx)) == 0 {
			t.Error("cancellation not returned")
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ForceUpdate ignored canceled context")
	}
}
