package backup

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func zipFiles(outputFilepath string, inputFilepaths ...string) error {
	f, err := os.CreateTemp(filepath.Dir(outputFilepath), ".ddns-backup-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	w := zip.NewWriter(f)
	defer w.Close()
	seen := make(map[string]bool)
	for _, inputPath := range inputFilepaths {
		name := filepath.Base(inputPath)
		if seen[name] {
			return fmt.Errorf("duplicate backup filename: %s", name)
		}
		seen[name] = true
		err = addFile(w, inputPath)
		if err != nil {
			return err
		}
	}
	if err = w.Close(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), outputFilepath)
}

func addFile(w *zip.Writer, inputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if filepath.Ext(f.Name()) == ".json" && !json.Valid(content) {
		return fmt.Errorf("invalid JSON in backup input %s", info.Name())
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	// Using FileInfoHeader() above only uses the basename of the file. If we want
	// to preserve the folder structure we can overwrite this with the full path.
	// header.Name = filepath
	header.Method = zip.Deflate
	header.SetMode(0600)
	ioWriter, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = ioWriter.Write(content)
	return err
}
