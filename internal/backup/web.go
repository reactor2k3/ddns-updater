package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxUploadSize = 8 << 20
const maxExpandedSize = 16 << 20

// Web backs the authenticated interface. Apply runs after services have stopped.
type Web struct {
	DataDir, ConfigPath, BackupDir string
	Snapshot                       func(context.Context, func() error) error
	ValidateConfig                 func([]byte) ([]byte, error)
	ValidateHistory                func([]byte) error
	RequestRestore                 func(map[string][]byte) error
}

func (b *Web) SaveResult(success bool) error {
	raw, err := json.Marshal(map[string]any{"success": success, "time": time.Now().UTC()})
	if err != nil {
		return err
	}
	return atomicBytes(filepath.Join(b.DataDir, ".restore-result.json"), raw)
}

func (b *Web) Download(ctx context.Context) (result []byte, err error) {
	create := func() error {
		dir, err := os.MkdirTemp(b.DataDir, ".backup-download-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		if err = createBackup(dir, b.DataDir, b.ConfigPath); err != nil {
			return err
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.zip"))
		if err != nil || len(files) != 1 {
			return errors.New("backup archive missing")
		}
		result, err = os.ReadFile(files[0])
		return err
	}
	if b.Snapshot != nil {
		err = b.Snapshot(ctx, create)
	} else {
		err = create()
	}
	return result, err
}

// ReadUpload accepts flat, bounded archives. Uploaded names never become paths.
func ReadUpload(raw []byte) (map[string][]byte, error) {
	if len(raw) == 0 || len(raw) > MaxUploadSize {
		return nil, errors.New("invalid archive size")
	}
	r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(r.File) < 2 || len(r.File) > 4 {
		return nil, errors.New("invalid archive")
	}
	files := make(map[string][]byte)
	total := 0
	for _, file := range r.File {
		name := file.Name
		if name == "" || strings.ContainsAny(name, "/\\:") || name == "." || name == ".." || !file.Mode().IsRegular() {
			return nil, errors.New("invalid archive entry")
		}
		if file.UncompressedSize64 > maxExpandedSize {
			return nil, errors.New("archive too large")
		}
		if name != "auth.json" && name != "updates.json" && name != ".setup-key" {
			if !strings.HasSuffix(name, ".json") {
				return nil, errors.New("unexpected archive entry")
			}
			name = "config.json"
		}
		if _, exists := files[name]; exists {
			return nil, errors.New("duplicate archive entry")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(reader, int64(maxExpandedSize-total+1)))
		_ = reader.Close()
		if err != nil {
			return nil, err
		}
		total += len(content)
		if total > maxExpandedSize {
			return nil, errors.New("archive too large")
		}
		if name != ".setup-key" && !json.Valid(content) {
			return nil, errors.New("invalid JSON")
		}
		files[name] = content
	}
	if len(files["config.json"]) == 0 || len(files["auth.json"]) == 0 {
		return nil, errors.New("configuration and account required")
	}
	delete(files, ".setup-key")
	if files["updates.json"] == nil {
		files["updates.json"] = []byte(`{"records":[]}`)
	}
	return files, nil
}

func (b *Web) Prepare(files map[string][]byte) error {
	if os.Getenv("CONFIG") != "" {
		return errors.New("environment configuration is read-only")
	}
	if b.RequestRestore == nil || b.ValidateConfig == nil || b.ValidateHistory == nil {
		return errors.New("restore unavailable")
	}
	var err error
	files["config.json"], err = b.ValidateConfig(files["config.json"])
	if err != nil {
		return err
	}
	return b.ValidateHistory(files["updates.json"])
}

func atomicBytes(path string, content []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".restore-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(content)
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
	return os.Rename(f.Name(), path)
}

// Apply saves a safety ZIP first and rolls back replaced files on write failure.
func (b *Web) Apply(files map[string][]byte) error {
	dir := b.BackupDir
	if dir == "" {
		dir = filepath.Join(b.DataDir, "backups")
	}
	if err := createBackup(dir, b.DataDir, b.ConfigPath); err != nil {
		return fmt.Errorf("safety backup: %w", err)
	}
	names := []string{"config.json", "updates.json", "auth.json"}
	paths, previous := map[string]string{}, map[string][]byte{}
	for _, name := range names {
		paths[name] = filepath.Join(b.DataDir, name)
		if name == "config.json" {
			paths[name] = b.ConfigPath
		}
		content, err := os.ReadFile(paths[name])
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		previous[name] = content
	}
	for i, name := range names {
		if err := atomicBytes(paths[name], files[name]); err != nil {
			var rollbackErr error
			for _, old := range names[:i] {
				if previous[old] == nil {
					rollbackErr = errors.Join(rollbackErr, os.Remove(paths[old]))
				} else {
					rollbackErr = errors.Join(rollbackErr, atomicBytes(paths[old], previous[old]))
				}
			}
			return errors.Join(err, rollbackErr)
		}
	}
	_ = os.Remove(filepath.Join(b.DataDir, ".setup-key"))
	return nil
}
