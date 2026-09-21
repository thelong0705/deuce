package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thelong0705/deuce/internal/adapter/crypto"
	"github.com/thelong0705/deuce/internal/adapter/httpapi"
	"github.com/thelong0705/deuce/internal/adapter/postgres"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dsn  = env("DB_URL", "postgres://deuce:deuce@localhost:5432/deuce?sslmode=disable")
		addr = env("HTTP_ADDR", ":8080")
	)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("parse db config: %w", err)
	}
	defer pool.Close()

	// pgxpool.New only parses the DSN, so fail fast here rather than on the
	// first request.
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}

	// The only place that knows about every layer: adapters are constructed
	// here and handed to the use case as the ports it declared.
	var (
		users  = postgres.NewUserRepository(postgres.New(pool))
		hasher = crypto.NewBcryptHasher()
		userUC = usecase.NewUser(users, hasher)
		api    = httpapi.NewServer(userUC)
	)

	srv := &http.Server{
		Addr:    addr,
		Handler: api.Handler(),

		// net/http has no defaults, so without these a slow client can hold a
		// connection open indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownErr := make(chan error, 1)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop

		slog.Info("shutdown signal received, draining connections")

		// Stop accepting new connections and give in-flight requests a bounded
		// window to finish.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		shutdownErr <- srv.Shutdown(ctx)
	}()

	slog.Info("listening", "addr", addr)

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	if err := <-shutdownErr; err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	slog.Info("stopped cleanly")
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
