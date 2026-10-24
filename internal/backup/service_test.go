package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBackupIncludesAccountAndCustomConfiguration(t *testing.T) {
	dataDir, outputDir := t.TempDir(), filepath.Join(t.TempDir(), "backups")
	configPath := filepath.Join(t.TempDir(), "custom-config.json")
	if err := os.WriteFile(configPath, []byte(`{"settings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"updates.json": "{}", "auth.json": `{"password":{"hash":"synthetic"},"credentials":[{"id":"synthetic-passkey"}],"devices":[{"label":"Test laptop"}]}`} {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := createBackup(outputDir, dataDir, configPath); err != nil {
		t.Fatal(err)
	}
	archives, err := filepath.Glob(filepath.Join(outputDir, "*.zip"))
	if err != nil || len(archives) != 1 {
		t.Fatal("backup archive not created")
	}
	info, err := os.Stat(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("backup readable by other users")
	}
	archive, err := zip.OpenReader(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := map[string]string{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = string(content)
		if file.Mode().Perm() != 0600 {
			t.Fatal("unsafe restored file permissions")
		}
	}
	if len(files) != 3 || files["custom-config.json"] != `{"settings":[]}` || files["auth.json"] == "" || files["updates.json"] != "{}" {
		t.Fatal("incomplete backup")
	}
}

func TestBackupBeforeAccountSetupAndOnFailure(t *testing.T) {
	dataDir, outputDir := t.TempDir(), t.TempDir()
	configPath := filepath.Join(dataDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := createBackup(outputDir, dataDir, configPath); err != nil {
		t.Fatal("optional authentication/history files prevented backup", err)
	}
	failedDir := t.TempDir()
	if err := createBackup(failedDir, dataDir, filepath.Join(dataDir, "missing.json")); err == nil {
		t.Fatal("missing config accepted")
	}
	leftovers, err := os.ReadDir(failedDir)
	if err != nil || len(leftovers) != 0 {
		t.Fatal("partial backup left behind")
	}
}

func TestBackupRejectsPartialJSONWithoutPublishingArchive(t *testing.T) {
	dataDir, outputDir := t.TempDir(), t.TempDir()
	configPath := filepath.Join(dataDir, "config.json")
	for path, content := range map[string]string{configPath: `{}`, filepath.Join(dataDir, "updates.json"): `{"records":`} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := createBackup(outputDir, dataDir, configPath); err == nil {
		t.Fatal("incomplete JSON was archived")
	}
	files, err := os.ReadDir(outputDir)
	if err != nil || len(files) != 0 {
		t.Fatal("invalid archive or temporary file left behind")
	}
}
