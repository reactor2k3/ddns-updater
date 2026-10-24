package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Service struct {
	// Injected fields
	backupPeriod   time.Duration
	dataDir        string
	configFilePath string
	outputDir      string
	logger         Logger

	// Internal fields
	stopCh chan<- struct{}
	done   <-chan struct{}
}

func New(backupPeriod time.Duration,
	dataDir, outputDir, configFilePath string, logger Logger,
) *Service {
	return &Service{
		logger:         logger,
		backupPeriod:   backupPeriod,
		dataDir:        dataDir,
		configFilePath: configFilePath,
		outputDir:      outputDir,
	}
}

func (s *Service) String() string {
	return "backup"
}

func makeZipFileName() string {
	return "ddns-updater-backup-" + strconv.Itoa(int(time.Now().UnixNano())) + ".zip"
}

func (s *Service) Start(ctx context.Context) (runError <-chan error, startErr error) {
	ready := make(chan struct{})
	runErrorCh := make(chan error, 1)
	stopCh := make(chan struct{})
	s.stopCh = stopCh
	done := make(chan struct{})
	s.done = done
	go run(ready, runErrorCh, stopCh, done,
		s.outputDir, s.dataDir, s.configFilePath, s.backupPeriod, s.logger)
	select {
	case <-ready:
	case <-ctx.Done():
		return nil, s.Stop()
	}
	return runErrorCh, nil
}

func run(ready chan<- struct{}, runError chan<- error, stopCh <-chan struct{},
	done chan<- struct{}, outputDir, dataDir, configFilePath string, backupPeriod time.Duration,
	logger Logger,
) {
	defer close(done)

	if backupPeriod == 0 {
		close(ready)
		logger.Info("disabled")
		return
	}

	logger.Info("each " + backupPeriod.String() +
		"; writing zip files to directory " + outputDir)
	timer := time.NewTimer(backupPeriod)
	close(ready)

	for {
		select {
		case <-timer.C:
		case <-stopCh:
			_ = timer.Stop()
			return
		}
		err := createBackup(outputDir, dataDir, configFilePath)
		if err != nil {
			runError <- err
			return
		}
		timer.Reset(backupPeriod)
	}
}

func createBackup(outputDir, dataDir, configFilePath string) error {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return err
	}
	if configFilePath == "" {
		configFilePath = filepath.Join(dataDir, "config.json")
	}
	inputs := []string{configFilePath}
	for _, name := range []string{"updates.json", "auth.json", ".setup-key"} {
		path := filepath.Join(dataDir, name)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		inputs = append(inputs, path)
	}
	return zipFiles(filepath.Join(outputDir, makeZipFileName()), inputs...)
}

func (s *Service) Stop() (err error) {
	close(s.stopCh)
	<-s.done
	return nil
}
