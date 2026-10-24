package update

import (
	"context"
	"errors"
	"github.com/reactor2k3/ddns-updater/internal/data"
	"github.com/reactor2k3/ddns-updater/internal/params"
	"github.com/reactor2k3/ddns-updater/internal/records"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("request failed with private-api-secret in request URL")
}

type fixtureNotifier struct{}

func (fixtureNotifier) Notify(string) {}
func TestProviderFailureDoesNotExposeCredentials(t *testing.T) {
	ps, _, err := params.Parse([]byte(`{"settings":[{"provider":"ipv64","domain":"test.example.com","ip_version":"ipv4","key":"private-api-secret"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	db := data.NewDatabase([]records.Record{records.New(ps[0], nil)}, fixturePersistence{})
	updater := NewUpdater(db, &http.Client{Transport: errorTransport{}}, fixtureNotifier{}, fixtureLogger{}, time.Now, true)
	err = updater.Update(context.Background(), 0, netip.MustParseAddr("203.0.113.42"))
	if err == nil {
		t.Fatal("provider failure not reported")
	}
	record, _ := db.Select(0)
	if strings.Contains(err.Error(), "private-api-secret") || strings.Contains(record.Message, "private-api-secret") {
		t.Fatal("provider credential leaked into error or record")
	}
}
