package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/madtoby2/tgcloud/internal/config"
	"github.com/madtoby2/tgcloud/internal/handler"
	"github.com/madtoby2/tgcloud/internal/manager"
	"github.com/madtoby2/tgcloud/internal/operator"
	"github.com/madtoby2/tgcloud/internal/server"
	"github.com/madtoby2/tgcloud/internal/store"
	"go.uber.org/zap"
)

//go:embed web
var webFS embed.FS

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	// Config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Data directory
	os.MkdirAll(cfg.DataDir, 0700)
	sessionDir := filepath.Join(cfg.DataDir, "sessions")
	os.MkdirAll(sessionDir, 0700)
	dbPath := filepath.Join(cfg.DataDir, "tgcloud.db")

	// Store
	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	logger.Info("tgcloud starting",
		zap.String("listen", cfg.ListenAddr),
		zap.String("data", cfg.DataDir),
	)

	// Manager (account pool + auth)
	mgr := manager.New(st, cfg.APIID, cfg.APIHash, logger)

	// Operator engine
	eng := operator.New(logger)

	// WebSocket hub
	wsh := handler.NewWSHub(logger)

	// Handler
	h := &handler.Handler{WS: wsh}
	h.WireServices(mgr, eng, st)

	// Static files
	var staticFS fs.FS
	webSub, err := fs.Sub(webFS, "web")
	if err == nil {
		staticFS = webSub
	} else {
		logger.Warn("no embedded web UI, using stub page", zap.Error(err))
	}

	// Server
	router := server.New(h, staticFS)
	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		logger.Info("shutting down...")
		srv.Close()
		mgr.Close()
		st.Close()
	}()

	fmt.Printf("\n  tgcloud v2.0.0\n")
	fmt.Printf("  http://localhost%s\n\n", cfg.ListenAddr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
