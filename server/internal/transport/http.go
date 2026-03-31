package transport

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/relayhub/relayhub/server/internal/admin"
	"github.com/relayhub/relayhub/server/internal/gateway"
	runtimeapp "github.com/relayhub/relayhub/server/internal/runtime"
)

//go:embed static/*
var embeddedStatic embed.FS

func NewRouter(app *runtimeapp.App) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	gateway.Mount(r, app)
	admin.Mount(r, app)

	staticDir := os.Getenv("RELAYHUB_CONSOLE_DIR")
	if staticDir == "" {
		staticDir = "./apps/console/out"
	}

	if stat, err := os.Stat(staticDir); err == nil && stat.IsDir() {
		fileServer(r, "/", http.Dir(staticDir))
		return r
	}

	subFS, err := fs.Sub(embeddedStatic, "static")
	if err == nil {
		fileServer(r, "/", http.FS(subFS))
	}
	return r
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("path must not contain any URL parameters")
	}
	fs := http.FileServer(root)
	r.Get(path+"*", func(w http.ResponseWriter, req *http.Request) {
		requestPath := req.URL.Path
		cleanPath := strings.TrimPrefix(requestPath, path)
		if cleanPath == "" || cleanPath == "/" {
			req.URL.Path = "/index.html"
		} else {
			fullPath := filepath.Join(".", cleanPath)
			if _, err := root.Open(fullPath); err != nil {
				req.URL.Path = "/index.html"
			}
		}
		fs.ServeHTTP(w, req)
	})
}
