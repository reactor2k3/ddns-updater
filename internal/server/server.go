package server

import (
	"fmt"
	"github.com/reactor2k3/ddns-updater/internal/auth"
	"github.com/reactor2k3/ddns-updater/internal/backup"
	"github.com/reactor2k3/ddns-updater/internal/control"
	"os"
	"strconv"

	"github.com/qdm12/goservices/httpserver"
)

func New(address, rootURL string, logger Logger, manager *control.Manager,
	webBackup *backup.Web,
) (server *httpserver.Server, err error) {
	publicOrigin := os.Getenv("AUTH_ORIGIN")
	if publicOrigin == "" {
		publicOrigin = "http://localhost:8000"
	}
	dataDir := os.Getenv("DATADIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	allowHTTP := true
	if value := os.Getenv("AUTH_ALLOW_INSECURE_HTTP"); value != "" {
		allowHTTP, err = strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid AUTH_ALLOW_INSECURE_HTTP: %w", err)
		}
	}
	access, setupKey, err := auth.New(auth.Config{Origin: publicOrigin, Root: rootURL, DataDir: dataDir, SetupKey: os.Getenv("AUTH_SETUP_KEY"), Backup: webBackup, AllowInsecureHTTP: allowHTTP})
	if err != nil {
		return nil, err
	}
	if setupKey != "" {
		logger.Warn("Initial setup key: " + setupKey)
	}
	if allowHTTP && len(publicOrigin) >= 7 && publicOrigin[:7] == "http://" {
		logger.Warn("HTTP mode: credentials and sessions are unencrypted. Passkeys require HTTPS with a trusted certificate (localhost excepted).")
	}
	return httpserver.New(httpserver.Settings{
		Handler: access.Wrap(AppHandler(rootURL, manager)),
		Address: &address,
		Logger:  logger,
	})
}
