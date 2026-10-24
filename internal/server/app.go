package server

import (
	"embed"
	"github.com/reactor2k3/ddns-updater/internal/control"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var appAssets embed.FS

// AppHandler serves the modern UI and management API. Authentication wraps it.
func AppHandler(root string, manager *control.Manager) http.Handler {
	folder, _ := fs.Sub(appAssets, "web")
	root = strings.TrimSuffix(root, "/")
	api := manager.Handler(root)
	files := http.StripPrefix(root+"/", http.FileServerFS(folder))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, root+"/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == root && root != "" {
			http.Redirect(w, r, root+"/", 308)
			return
		}
		if r.URL.Path == root+"/sw.js" {
			w.Header().Set("Service-Worker-Allowed", root+"/")
		}
		relative := strings.TrimPrefix(r.URL.Path, root+"/")
		if relative != "" && !fs.ValidPath(relative) {
			http.NotFound(w, r)
			return
		}
		allowed := map[string]bool{"": true, "index.html": true, "styles.css": true, "theme-init.js": true, "i18n.js": true, "form.js": true, "providers.js": true, "live.js": true, "manifest.webmanifest": true, "sw.js": true}
		if !allowed[relative] && !strings.HasPrefix(relative, "icons/") {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
