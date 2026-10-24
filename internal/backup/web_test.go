package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testZIP(t *testing.T, names []string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(`{}`))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestUploadRejectsUnsafeArchives(t *testing.T) {
	for _, names := range [][]string{
		{"config.json"}, {"auth.json", "../config.json"}, {"auth.json", `x\config.json`},
		{"auth.json", "config.json", "custom.json"}, {"auth.json", "config.json", "auth.json"},
		{"auth.json", "config.json", "script.exe"}, {"auth.json", "config.json", "nested/"},
	} {
		if _, err := ReadUpload(testZIP(t, names)); err == nil {
			t.Fatalf("unsafe archive accepted: %q", names)
		}
	}
	if _, err := ReadUpload(bytes.Repeat([]byte{0}, MaxUploadSize+1)); err == nil {
		t.Fatal("oversize archive accepted")
	}
	files, err := ReadUpload(testZIP(t, []string{"auth.json", "custom-settings.json"}))
	if err != nil || files["config.json"] == nil || files["updates.json"] == nil {
		t.Fatal("custom filename or absent initial history rejected", err)
	}
	var bomb bytes.Buffer
	w := zip.NewWriter(&bomb)
	f, _ := w.Create("config.json")
	_, _ = f.Write(bytes.Repeat([]byte{' '}, maxExpandedSize+1))
	f, _ = w.Create("auth.json")
	_, _ = f.Write([]byte(`{}`))
	_ = w.Close()
	if _, err := ReadUpload(bomb.Bytes()); err == nil {
		t.Fatal("compressed oversized input accepted")
	}
}

func TestWebDownloadAndSafetyRestore(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "custom.json")
	for name, raw := range map[string]string{"custom.json": `{"settings":[]}`, "auth.json": `{"old":true}`, "updates.json": `{"records":[]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b := &Web{DataDir: dir, ConfigPath: config, BackupDir: filepath.Join(dir, "backups")}
	raw, err := b.Download(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	files, err := ReadUpload(raw)
	if err != nil {
		t.Fatal(err)
	}
	files["auth.json"] = []byte(`{"new":true}`)
	if err = b.Apply(files); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "auth.json"))
	if !bytes.Equal(content, files["auth.json"]) {
		t.Fatal("restore missing")
	}
	archives, _ := filepath.Glob(filepath.Join(b.BackupDir, "*.zip"))
	if len(archives) != 1 {
		t.Fatal("safety archive missing")
	}
	safety, _ := os.ReadFile(archives[0])
	previous, err := ReadUpload(safety)
	if err != nil || !bytes.Contains(previous["auth.json"], []byte(`"old"`)) {
		t.Fatal("previous account not recoverable")
	}
	for _, name := range []string{"auth.json", "updates.json", "custom.json"} {
		content, _ := os.ReadFile(filepath.Join(dir, name))
		if !json.Valid(content) {
			t.Fatal("invalid restored JSON")
		}
	}
}
