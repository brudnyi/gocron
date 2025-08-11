package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.uis.dev/service/gocron/internal/api"
	"gitlab.uis.dev/service/gocron/internal/config"
	"gitlab.uis.dev/service/gocron/internal/scheduler"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("application finished with an error", "error", err)
		os.Exit(1)
	}
	log.Info("application shutdown complete")
}

func run(log *slog.Logger) error {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}
	log.Info("configuration loaded successfully")

	pool, err := pgxpool.New(context.Background(), cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("cannot connect to postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("cannot ping postgres: %w", err)
	}
	log.Info("database connection established")

	store := postgres.NewStore(pool)
	sched, err := scheduler.New(log, cfg, store)
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
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
			// In this goroutine, we can't return an error, so we trigger a shutdown.
			stop()
		}
	}()

	<-ctx.Done()

	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown failed", "error", err)
	}

	sched.Stop()

	return nil
}
