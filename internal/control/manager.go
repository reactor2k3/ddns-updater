// Package control manages durable DDNS settings and serializes changes with updates.
package control

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reactor2k3/ddns-updater/internal/data"
	"github.com/reactor2k3/ddns-updater/internal/models"
	"github.com/reactor2k3/ddns-updater/internal/params"
	"github.com/reactor2k3/ddns-updater/internal/records"
	"github.com/reactor2k3/ddns-updater/internal/update"
	"github.com/reactor2k3/ddns-updater/pkg/publicip/ipversion"
)

type Runner interface {
	Configure(context.Context, func() error) error
	ForceUpdate(context.Context) []error
	RefreshPublicIPs(context.Context) []error
	Status() update.RuntimeStatus
}
type History interface {
	GetEvents(string, string, ipversion.IPVersion) ([]models.HistoryEvent, error)
}
type Entry struct {
	ID          string                     `json:"id"`
	Revision    uint64                     `json:"revision"`
	Provider    string                     `json:"providerId"`
	Domain      string                     `json:"domain"`
	Description string                     `json:"description"`
	IP          string                     `json:"ip"`
	Enabled     bool                       `json:"enabled"`
	Config      map[string]json.RawMessage `json:"config"`
}
type Event struct {
	Version string    `json:"version,omitempty"`
	Domain  string    `json:"domain"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}
type diskConfig struct {
	Settings []map[string]json.RawMessage `json:"settings"`
	Entries  []Entry                      `json:"ui_entries,omitempty"`
	Events   []Event                      `json:"ui_events,omitempty"`
}
type Manager struct {
	mu            sync.Mutex
	path          string
	db            *data.Database
	runner        Runner
	history       History
	entries       []Entry
	bindings      []string
	events        []Event
	readOnly      bool
	queue         atomic.Bool
	ipQueue       atomic.Bool
	lastIPRefresh time.Time
	ctx           context.Context
}

var domainPattern = regexp.MustCompile(`^(?:\*\.)?[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z]{2,63}$`)

// New imports ordinary config.json files and uses the same file for all later edits.
// Native settings remain available for other ddns-updater tools; UI metadata also
// preserves paused entries and stable IDs. Call before starting the update service.
func New(ctx context.Context, path string, db *data.Database, runner Runner, history History) (*Manager, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return newFromRaw(ctx, path, db, runner, history, raw)
}

func newFromRaw(ctx context.Context, path string, db *data.Database, runner Runner, history History, raw []byte) (*Manager, error) {
	m := &Manager{path: path, db: db, runner: runner, history: history, ctx: ctx, readOnly: os.Getenv("CONFIG") != ""}
	var stored diskConfig
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}
	m.entries = stored.Entries
	m.events = stored.Events
	if m.entries == nil {
		for _, setting := range stored.Settings {
			b, _ := json.Marshal(map[string]any{"settings": []any{setting}})
			ps, _, err := params.Parse(b)
			if err != nil {
				return nil, err
			}
			for _, p := range ps {
				e := Entry{ID: newID(), Revision: 1, Domain: p.BuildDomainName(), Enabled: true, Config: map[string]json.RawMessage{}, IP: ipName(p.IPVersion())}
				_ = json.Unmarshal(setting["provider"], &e.Provider)
				for k, v := range setting {
					if k != "provider" && k != "domain" && k != "host" && k != "owner" && k != "ip_version" {
						e.Config[k] = v
					}
				}
				m.entries = append(m.entries, e)
			}
		}
	}
	seen := map[string]bool{}
	for i := range m.entries {
		e := &m.entries[i]
		if e.ID == "" || seen[e.ID] {
			return nil, errors.New("invalid or duplicate record ID")
		}
		seen[e.ID] = true
		if e.Revision == 0 {
			e.Revision = 1
		}
	}
	next, bindings, _, err := m.compile(m.entries)
	if err != nil {
		return nil, err
	}
	m.bindings = bindings
	m.db.Replace(next)
	return m, nil
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func ipName(ip ipversion.IPVersion) string {
	switch ip {
	case ipversion.IP4:
		return "IPv4"
	case ipversion.IP6:
		return "IPv6"
	default:
		return "Automatisch"
	}
}
func native(e Entry, version string) map[string]json.RawMessage {
	result := map[string]json.RawMessage{}
	for k, v := range e.Config {
		result[k] = v
	}
	for k, v := range map[string]string{"provider": e.Provider, "domain": e.Domain, "ip_version": version} {
		result[k], _ = json.Marshal(v)
	}
	return result
}
func versions(ip string) []string {
	switch ip {
	case "IPv4":
		return []string{"ipv4"}
	case "IPv6":
		return []string{"ipv6"}
	case "Dual Stack":
		return []string{"ipv4", "ipv6"}
	case "Automatisch":
		return []string{"ipv4 or ipv6"}
	default:
		return nil
	}
}
func (m *Manager) compile(entries []Entry) (next []records.Record, bindings []string, settings []map[string]json.RawMessage, err error) {
	previous := map[string]records.Record{}
	for _, r := range m.db.SelectAll() {
		previous[r.Provider.BuildDomainName()+"/"+r.Provider.IPVersion().String()] = r
	}
	for _, e := range entries {
		if versions(e.IP) == nil || !domainPattern.MatchString(e.Domain) || strings.Contains(e.Domain, "..") || len(e.Domain) > 253 || len(e.Description) > 200 {
			return nil, nil, nil, errors.New("invalid record")
		}
		for _, version := range versions(e.IP) {
			setting := native(e, version)
			raw, _ := json.Marshal(map[string]any{"settings": []any{setting}})
			ps, _, parseErr := params.Parse(raw)
			if parseErr != nil {
				return nil, nil, nil, errors.New("invalid provider settings")
			}
			if len(ps) != 1 || ps[0].IPVersion().String() != version {
				return nil, nil, nil, errors.New("IP version not supported by provider")
			}
			if !e.Enabled {
				continue
			}
			p := ps[0]
			r := records.New(p, nil)
			if old, ok := previous[p.BuildDomainName()+"/"+version]; ok {
				r.History = old.History
				for _, existing := range m.entries {
					if existing.ID != e.ID {
						continue
					}
					a, _ := json.Marshal(native(existing, version))
					b, _ := json.Marshal(native(e, version))
					if bytes.Equal(a, b) && existing.Enabled == e.Enabled {
						r = old
						r.Provider = p
					}
				}
			}
			if len(r.History) == 0 && m.history != nil {
				r.History, err = m.history.GetEvents(p.Domain(), p.Owner(), p.IPVersion())
				if err != nil {
					return nil, nil, nil, err
				}
			}
			next = append(next, r)
			bindings = append(bindings, e.ID)
			settings = append(settings, setting)
		}
	}
	return next, bindings, settings, nil
}
func writeAtomic(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
func (m *Manager) commit(ctx context.Context, entries []Entry, event Event) error {
	next, bindings, settings, err := m.compile(entries)
	if err != nil {
		return err
	}
	events := append([]Event{event}, m.events...)
	if len(events) > 200 {
		events = events[:200]
	}
	if settings == nil {
		settings = []map[string]json.RawMessage{}
	}
	// Keep a non-nil empty UI list, otherwise deleting the final entry would
	// accidentally trigger legacy import after restart.
	if entries == nil {
		entries = []Entry{}
	}
	return m.runner.Configure(ctx, func() error {
		value := diskConfig{Settings: settings, Entries: entries, Events: events}
		// ui_entries must survive even when empty (see the custom encoder below).
		raw := map[string]any{"settings": value.Settings, "ui_entries": value.Entries, "ui_events": value.Events}
		if err := writeAtomic(m.path, raw); err != nil {
			return err
		}
		m.db.Replace(next)
		m.entries = entries
		m.bindings = bindings
		m.events = events
		return nil
	})
}
