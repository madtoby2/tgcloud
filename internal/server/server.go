package server

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/madtoby2/tgcloud/internal/handler"
	"github.com/madtoby2/tgcloud/internal/manager"
	"go.uber.org/zap"
)

type Server struct {
	http *http.Server
	mgr  *manager.Manager
}

func New(addr string, mgr *manager.Manager, webFS fs.FS, logger *zap.Logger) *Server {
	wsh := handler.NewWSHub()
	h := handler.New(mgr, wsh)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}))

	// API routes
	r.Mount("/", h.Routes())

	// Static web frontend (serve from / — index.html, style.css, app.js)
	fileServer := http.FileServer(http.FS(webFS))
	r.Get("/", fileServer.ServeHTTP)
	r.Get("/style.css", fileServer.ServeHTTP)
	r.Get("/app.js", fileServer.ServeHTTP)
	r.NotFound(fileServer.ServeHTTP)

	return &Server{
		http: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		mgr: mgr,
	}
}

func (s *Server) Start() error { return s.http.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error {
	s.mgr.Close()
	return s.http.Shutdown(ctx)
}
