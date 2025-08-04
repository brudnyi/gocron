package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"gitlab.uis.dev/service/gocron/internal/api"
	"gitlab.uis.dev/service/gocron/internal/config"
	"gitlab.uis.dev/service/gocron/internal/scheduler"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Error("cannot load config", "error", err)
		os.Exit(1)
	}
	log.Info("configuration loaded successfully")

	conn, err := pgx.Connect(context.Background(), cfg.Postgres.URL)
	if err != nil {
		log.Error("cannot connect to postgres", "error", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())
	if err := conn.Ping(context.Background()); err != nil {
		log.Error("cannot ping postgres", "error", err)
		os.Exit(1)
	}
	log.Info("database connection established")

	store := postgres.NewStore(conn)
	sched, err := scheduler.New(log, cfg, store)
	if err != nil {
		log.Error("failed to create scheduler", "error", err)
		os.Exit(1)
	}

	httpServer := api.NewServer(log, sched)
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.Port),
		Handler: httpServer,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		sched.Start(ctx)
	}()

	go func() {
		log.Info("starting server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen and serve returned err", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown failed", "error", err)
	}

	sched.Stop()

	log.Info("shutdown complete")
}
