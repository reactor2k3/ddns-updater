package control

import (
	"context"
	"encoding/json"

	"github.com/reactor2k3/ddns-updater/internal/data"
)

func (m *Manager) BackupSnapshot(ctx context.Context, create func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runner.Configure(ctx, create)
}

// ValidateBackup imports into an isolated database and pauses restored records.
func ValidateBackup(raw []byte) ([]byte, error) {
	m, err := newFromRaw(context.Background(), "", data.NewDatabase(nil, nil), nil, nil, raw)
	if err != nil {
		return nil, err
	}
	for i := range m.entries {
		m.entries[i].Enabled = false
	}
	if m.entries == nil {
		m.entries = []Entry{}
	}
	return json.Marshal(diskConfig{Settings: []map[string]json.RawMessage{}, Entries: m.entries, Events: m.events})
}
