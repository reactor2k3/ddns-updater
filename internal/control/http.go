package control

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/reactor2k3/ddns-updater/internal/constants"
	"github.com/reactor2k3/ddns-updater/internal/params"
	"github.com/reactor2k3/ddns-updater/internal/records"
)

//go:embed catalog.json
var catalogFS embed.FS

type field struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}
type providerSchema struct {
	ID     string  `json:"id"`
	Fields []field `json:"fields"`
}

func schema(id string) (providerSchema, bool) {
	b, _ := catalogFS.ReadFile("catalog.json")
	var providers []providerSchema
	_ = json.Unmarshal(b, &providers)
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return providerSchema{}, false
}
func secret(f field) bool { return f.Type == "password" || f.Type == "textarea" || f.Key == "url" }
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, code string) {
	respond(w, status, map[string]string{"error": code})
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		fail(w, 400, "INVALID_INPUT")
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail(w, 400, "INVALID_INPUT")
		return false
	}
	return true
}
func redacted(e Entry) map[string]any {
	visible := map[string]json.RawMessage{}
	configured := []string{}
	p, _ := schema(e.Provider)
	fields := map[string]field{}
	for _, f := range p.Fields {
		fields[f.Key] = f
	}
	for k, v := range e.Config {
		f, known := fields[k]
		if known && !secret(f) {
			visible[k] = v
		} else if string(v) != "\"\"" && string(v) != "null" {
			configured = append(configured, k)
		}
	}
	return map[string]any{"id": e.ID, "revision": e.Revision, "providerId": e.Provider, "domain": e.Domain, "description": e.Description, "ip": e.IP, "enabled": e.Enabled, "config": visible, "configuredSecrets": configured}
}
func validateFields(e Entry) bool {
	p, ok := schema(e.Provider)
	if !ok {
		return false
	}
	allowed := map[string]field{}
	for _, f := range p.Fields {
		allowed[f.Key] = f
	}
	for k, v := range e.Config {
		f, ok := allowed[k]
		if !ok {
			return false
		}
		var parsed any
		if json.Unmarshal(v, &parsed) != nil {
			return false
		}
		switch f.Type {
		case "checkbox":
			if _, ok := parsed.(bool); !ok {
				return false
			}
		case "number":
			if n, ok := parsed.(float64); !ok || n < 0 || n != float64(int64(n)) {
				return false
			}
		case "textarea":
			if k == "credentials" {
				if _, ok := parsed.(map[string]any); !ok {
					return false
				}
			} else if _, ok := parsed.(string); !ok {
				return false
			}
		default:
			if s, ok := parsed.(string); !ok || len(s) > 32768 {
				return false
			}
		}
	}
	return true
}
func (m *Manager) Handler(root string) http.Handler {
	mux := http.NewServeMux()
	root = strings.TrimSuffix(root, "/")
	mux.HandleFunc("GET "+root+"/api/entries", m.snapshot)
	mux.HandleFunc("POST "+root+"/api/entries", m.mutate)
	mux.HandleFunc("PUT "+root+"/api/entries/{id}", m.mutate)
	mux.HandleFunc("DELETE "+root+"/api/entries/{id}", m.remove)
	mux.HandleFunc("GET "+root+"/api/status", m.snapshot)
	mux.HandleFunc("POST "+root+"/api/update", m.force)
	mux.HandleFunc("POST "+root+"/api/public-ips/refresh", m.refreshIPs)
	return mux
}
func (m *Manager) snapshot(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []map[string]any{}
	events := append([]Event{}, m.events...)
	rs := m.db.SelectAll()
	for _, e := range m.entries {
		item := redacted(e)
		status := "waiting"
		ip4, ip6 := "", ""
		checked := time.Time{}
		ready, total := 0, 0
		selected := []records.Record{}
		for i, record := range rs {
			if i < len(m.bindings) && m.bindings[i] == e.ID {
				selected = append(selected, record)
			}
		}
		if !e.Enabled && m.history != nil {
			for _, version := range versions(e.IP) {
				raw, _ := json.Marshal(map[string]any{"settings": []any{native(e, version)}})
				ps, _, err := params.Parse(raw)
				if err != nil {
					continue
				}
				for _, p := range ps {
					history, err := m.history.GetEvents(p.Domain(), p.Owner(), p.IPVersion())
					if err == nil {
						record := records.New(p, history)
						record.Time = record.History.GetSuccessTime()
						selected = append(selected, record)
					}
				}
			}
		}
		issues := []map[string]string{}
		for _, record := range selected {
			total++
			switch record.Status {
			case constants.FAIL:
				status = "failure"
				code := "PROVIDER_FAILED"
				if record.Message == "public IP address not found" {
					code = "PUBLIC_IP_UNAVAILABLE"
				}
				issues = append(issues, map[string]string{"version": ipName(record.Provider.IPVersion()), "code": code})
			case constants.UPDATING:
				if status != "failure" {
					status = "updating"
				}
			case constants.UPTODATE, constants.SUCCESS:
				ready++
			}
			if record.Time.After(checked) {
				checked = record.Time
			}
			ip := record.History.GetCurrentIP()
			if ip.Is4() {
				ip4 = ip.String()
			} else if ip.Is6() {
				ip6 = ip.String()
			}
			history := record.History
			start := len(history) - 20
			if start < 0 {
				start = 0
			}
			for _, event := range history[start:] {
				events = append(events, Event{Domain: e.Domain, Message: "DNS_UPDATED", Time: event.Time, Version: ipName(record.Provider.IPVersion())})
			}
			if record.Status == constants.FAIL {
				events = append(events, Event{Domain: e.Domain, Message: "DNS_FAILED", Time: record.Time, Version: ipName(record.Provider.IPVersion())})
			}
		}
		if !e.Enabled {
			status = "paused"
		} else if total > 0 && ready == total {
			status = "ready"
		}
		item["status"] = status
		item["issues"] = issues
		item["ipv4"] = ip4
		item["ipv6"] = ip6
		item["checkedAt"] = checked
		result = append(result, item)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time.After(events[j].Time) })
	if len(events) > 200 {
		events = events[:200]
	}
	runtime := m.runner.Status()
	runtime.IPDiscoveryRunning = runtime.IPDiscoveryRunning || m.ipQueue.Load()
	respond(w, 200, map[string]any{"entries": result, "events": events, "status": runtime, "readOnly": m.readOnly})
}
func (m *Manager) mutate(w http.ResponseWriter, r *http.Request) {
	var input Entry
	if !decode(w, r, &input) {
		return
	}
	input.Domain = strings.ToLower(strings.TrimSpace(input.Domain))
	input.Description = strings.TrimSpace(input.Description)
	if input.Config == nil {
		input.Config = map[string]json.RawMessage{}
	}
	if !validateFields(input) {
		fail(w, 422, "INVALID_PROVIDER_CONFIG")
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readOnly {
		fail(w, 409, "CONFIG_READ_ONLY")
		return
	}
	id := r.PathValue("id")
	creating := r.Method == http.MethodPost
	index := -1
	for i, e := range m.entries {
		if e.ID == id {
			index = i
		}
		if e.Domain == input.Domain && e.ID != id {
			fail(w, 409, "DUPLICATE_DOMAIN")
			return
		}
	}
	if creating {
		if len(m.entries) >= 200 {
			fail(w, 409, "ENTRY_LIMIT")
			return
		}
		input.ID = newID()
		input.Revision = 1
	} else {
		if index < 0 {
			fail(w, 404, "ENTRY_NOT_FOUND")
			return
		}
		old := m.entries[index]
		if input.Revision != old.Revision {
			fail(w, 409, "REVISION_CONFLICT")
			return
		}
		input.ID = id
		input.Revision = old.Revision + 1
		if input.Provider == old.Provider {
			p, _ := schema(old.Provider)
			known := map[string]bool{}
			for _, f := range p.Fields {
				known[f.Key] = true
			}
			for _, f := range p.Fields {
				if secret(f) {
					value, present := input.Config[f.Key]
					if !present || string(value) == "\"\"" {
						if v, ok := old.Config[f.Key]; ok {
							input.Config[f.Key] = v
						}
					}
				}
			}
			// Preserve native optional fields from legacy configurations.
			for k, v := range old.Config {
				if _, present := input.Config[k]; !present && !known[k] {
					input.Config[k] = v
				}
			}
		}
	}
	entries := append([]Entry{}, m.entries...)
	if creating {
		entries = append(entries, input)
	} else {
		entries[index] = input
	}
	code := "ENTRY_CHANGED"
	if creating {
		code = "ENTRY_CREATED"
	}
	if !input.Enabled {
		code = "ENTRY_PAUSED"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := m.commit(ctx, entries, Event{Domain: input.Domain, Message: code, Time: time.Now().UTC()}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			fail(w, 503, "UPDATE_BUSY")
		} else {
			fail(w, 422, "SAVE_FAILED")
		}
		return
	}
	status := 200
	if creating {
		status = 201
	}
	respond(w, status, redacted(input))
}
func (m *Manager) remove(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision uint64 `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readOnly {
		fail(w, 409, "CONFIG_READ_ONLY")
		return
	}
	id := r.PathValue("id")
	entries := []Entry{}
	var old *Entry
	for _, e := range m.entries {
		if e.ID == id {
			copy := e
			old = &copy
		} else {
			entries = append(entries, e)
		}
	}
	if old == nil {
		fail(w, 404, "ENTRY_NOT_FOUND")
		return
	}
	if input.Revision != old.Revision {
		fail(w, 409, "REVISION_CONFLICT")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := m.commit(ctx, entries, Event{Domain: old.Domain, Message: "ENTRY_DELETED", Time: time.Now().UTC()}); err != nil {
		fail(w, 503, "SAVE_FAILED")
		return
	}
	respond(w, 200, map[string]bool{"deleted": true})
}
func (m *Manager) force(w http.ResponseWriter, _ *http.Request) {
	if !m.queue.CompareAndSwap(false, true) {
		fail(w, 409, "UPDATE_BUSY")
		return
	}
	if m.runner.Status().Running {
		m.queue.Store(false)
		fail(w, 409, "UPDATE_BUSY")
		return
	}
	go func() { defer m.queue.Store(false); _ = m.runner.ForceUpdate(m.ctx) }()
	respond(w, 202, map[string]bool{"accepted": true})
}

func (m *Manager) refreshIPs(w http.ResponseWriter, r *http.Request) {
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if !m.ipQueue.CompareAndSwap(false, true) {
		fail(w, 409, "IP_REFRESH_BUSY")
		return
	}
	m.mu.Lock()
	if time.Since(m.lastIPRefresh) < 30*time.Second || m.runner.Status().IPDiscoveryRunning {
		m.mu.Unlock()
		m.ipQueue.Store(false)
		w.Header().Set("Retry-After", "30")
		fail(w, 429, "IP_REFRESH_LIMITED")
		return
	}
	m.lastIPRefresh = time.Now()
	m.mu.Unlock()
	go func() { defer m.ipQueue.Store(false); _ = m.runner.RefreshPublicIPs(m.ctx) }()
	respond(w, 202, map[string]bool{"accepted": true})
}
