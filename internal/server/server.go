package server

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/madtoby2/tgcloud/internal/handler"
)

func New(h *handler.Handler, webFS fs.FS) chi.Router {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(corsMiddleware)

	// API routes
	r.Mount("/", h.Routes())

	// Static file serving (SPA mode)
	if webFS != nil {
		fileServer := http.FileServer(http.FS(webFS))
		r.Get("/", fileServer.ServeHTTP)
		r.Get("/assets/*", fileServer.ServeHTTP)
		r.NotFound(fileServer.ServeHTTP)
	} else {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<!DOCTYPE html><html><head><title>tgcloud</title></head>
<body style="background:#0f172a;color:#e2e8f0;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center"><h1 style="font-size:3rem;margin:0">tgcloud</h1>
<p style="color:#64748b">Telegram cloud control panel</p>
<p style="color:#475569">Frontend not built — run <code>cd web && npm run build</code></p></div></body></html>`))
		})
	}

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
