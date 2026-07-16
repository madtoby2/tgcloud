package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/madtoby2/tgcloud/internal/manager"
	"github.com/madtoby2/tgcloud/internal/server"
	"github.com/madtoby2/tgcloud/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:embed web
var webFS embed.FS

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "tgcloud.db", "sqlite database path")
	apiID := flag.Int("api-id", 0, "Telegram API ID")
	apiHash := flag.String("api-hash", "", "Telegram API hash")
	flag.Parse()

	if *apiID == 0 || *apiHash == "" {
		fmt.Println("ERROR: --api-id and --api-hash are required")
		fmt.Println("Get them from https://my.telegram.org/apps")
		os.Exit(1)
	}

	logger, _ := zap.NewDevelopment(zap.IncreaseLevel(zapcore.InfoLevel))
	defer logger.Sync()

	st, err := store.New(*dbPath)
	if err != nil {
		logger.Fatal("failed to open database", zap.Error(err))
	}
	defer st.Close()

	mgr := manager.New(st, *apiID, *apiHash, logger)

	// Strip the "web" prefix from embed paths so /index.html works
	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		logger.Fatal("failed to sub web fs", zap.Error(err))
	}

	srv := server.New(*addr, mgr, webSub, logger)

	logger.Info("tgcloud starting", zap.String("addr", *addr))

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down...")
		srv.Shutdown(context.Background())
	}()

	if err := srv.Start(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
