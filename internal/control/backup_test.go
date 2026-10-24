package control

import (
	"encoding/json"
	"testing"
)

func TestBackupValidationPausesLegacyAndManagedRecords(t *testing.T) {
	t.Setenv("TMPDIR", "does-not-exist") // Scratch images need no temporary directory.
	legacy := []byte(`{"settings":[{"provider":"ipv64","domain":"restore.example.com","key":"synthetic-key","ip_version":"ipv4"}]}`)
	raw, err := ValidateBackup(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var config diskConfig
	if err = json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Entries) != 1 || config.Entries[0].Enabled || len(config.Settings) != 0 {
		t.Fatal("restored legacy record not paused")
	}
	config.Entries[0].Enabled = true
	raw, _ = json.Marshal(config)
	raw, err = ValidateBackup(raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &config)
	if config.Entries[0].Enabled {
		t.Fatal("restored managed record active")
	}
	if _, err = ValidateBackup([]byte(`{"settings":[{"provider":"invalid"}]}`)); err == nil {
		t.Fatal("invalid provider accepted")
	}
}
