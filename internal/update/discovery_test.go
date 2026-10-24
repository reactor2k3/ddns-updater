package update

import (
	"context"
	"errors"
	"github.com/reactor2k3/ddns-updater/internal/data"
	"net/netip"
	"testing"
	"time"
)

func TestStartupAndManualIPDiscoveryWithoutDNSUpdates(t *testing.T) {
	db := data.NewDatabase(nil, fixturePersistence{})
	release := make(chan struct{})
	close(release)
	updater := &blockingUpdater{entered: make(chan struct{}), release: release}
	service := NewService(db, updater, fixtureIPs{}, 25*time.Millisecond, 5*time.Minute, fixtureLogger{}, fixtureLookup{}, time.Now, fixtureHeartbeat{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	if errs := service.RefreshPublicIPs(ctx); len(errs) != 0 {
		t.Fatal(errs)
	}
	state := service.Status()
	if state.IPv4 != "203.0.113.42" || state.IPv6 != "2001:db8::42" || state.IPCheckedAt.IsZero() || state.IPDiscoveryRunning {
		t.Fatalf("addresses not discovered without active DNS records: %+v", state)
	}
	if !state.LastCheck.IsZero() {
		t.Fatal("IP-only discovery must not claim a DNS check")
	}
	initialCheck := state.IPCheckedAt
	for !service.Status().IPCheckedAt.After(initialCheck) {
		select {
		case <-ctx.Done():
			t.Fatal("periodic IP discovery did not run")
		case <-time.After(time.Millisecond):
		}
	}
	updater.mu.Lock()
	defer updater.mu.Unlock()
	if len(updater.ips) != 0 {
		t.Fatal("IP discovery sent a provider update")
	}
}

type noIPv6 struct{ fixtureIPs }

type selectedAutomaticIP struct {
	fixtureIPs
	calls int
	fail  bool
}

func (f *selectedAutomaticIP) IP(context.Context) (netip.Addr, error) {
	f.calls++
	if f.fail {
		return netip.Addr{}, errors.New("selected generic provider unavailable")
	}
	return netip.MustParseAddr("2001:db8::99"), nil
}
func TestAutomaticIPUsesSelectedGenericProviderAndCaches(t *testing.T) {
	getter := &selectedAutomaticIP{}
	now := time.Now()
	service := NewService(nil, nil, getter, time.Hour, 0, fixtureLogger{}, nil, func() time.Time { return now }, nil)
	for range 2 {
		ip, ip4, ip6, errs := service.currentIPs(context.Background(), true, false, false)
		if len(errs) != 0 || ip.String() != "2001:db8::99" || !ip4.Is4() || !ip6.Is6() {
			t.Fatalf("generic selection overridden: %s %v", ip, errs)
		}
	}
	if getter.calls != 1 {
		t.Fatal("generic IP was not cached")
	}
	now = now.Add(31 * time.Second)
	getter.fail = true
	ip, _, _, errs := service.currentIPs(context.Background(), true, false, false)
	if ip.IsValid() || len(errs) == 0 {
		t.Fatal("failed selected provider silently fell back to another provider")
	}
}

func (noIPv6) IP6(context.Context) (netip.Addr, error) {
	return netip.Addr{}, errors.New("IPv6 unavailable")
}
func TestUnusedIPv6FailureDoesNotFailIPv4AndClearsStaleAddress(t *testing.T) {
	service := NewService(nil, nil, noIPv6{}, time.Hour, 0, fixtureLogger{}, nil, time.Now, nil)
	service.stats.IPv6 = "2001:db8::99"
	_, ip4, ip6, errs := service.currentIPs(context.Background(), false, true, false)
	if len(errs) != 0 || !ip4.Is4() || ip6.IsValid() {
		t.Fatal("unavailable unused IPv6 failed an IPv4-only record")
	}
	state := service.Status()
	if state.IPv6 != "" || state.IPv6Status != "unavailable" || state.IPv4Status != "available" {
		t.Fatalf("incorrect address availability: %+v", state)
	}
	_, _, _, errs = service.currentIPs(context.Background(), false, true, true)
	if len(errs) == 0 {
		t.Fatal("a dual-stack record must report its missing IPv6 address")
	}
}
