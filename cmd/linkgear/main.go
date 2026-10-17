package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sunshow/linkgear/internal/api"
	"github.com/sunshow/linkgear/internal/config"
	"github.com/sunshow/linkgear/internal/db"
	"github.com/sunshow/linkgear/internal/service"
	"github.com/sunshow/linkgear/internal/worker"
)

func main() {
	cfg := config.Load()

	gdb, driver, err := db.Open(cfg.DSN)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		log.Fatalf("sql db: %v", err)
	}
	// Keep GORM pool open first so in-memory sqlite shared-cache survives migrate.Close.
	if err := db.MigrateDSN(cfg.DSN); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("linkgear db ready (%s)", driver)

	svc := service.New(gdb, cfg.PublicURL)
	apiSrv := &api.Server{Svc: svc}
	ui, err := api.NewUI(svc, cfg.PublicURL)
	if err != nil {
		log.Fatalf("ui: %v", err)
	}

	mux := http.NewServeMux()
	api.Mount(mux, apiSrv, ui)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outbox := worker.NewOutbox(gdb, cfg.DeliveryWorkers, cfg.DeliveryPoll)
	go outbox.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("linkgear listening on %s (public %s)", cfg.HTTPAddr, cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("shutting down…")
	cancel()
	shutdownCtx, c2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer c2()
	_ = srv.Shutdown(shutdownCtx)
	_ = sqlDB.Close()
}
