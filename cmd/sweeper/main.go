// Command sweeper puts back the slots nobody paid for.
//
// A hold occupies its slot until something cancels it, so without this an
// abandoned checkout blocks that hour for ever. It runs apart from the server
// because it is a different job with a different failure: the server being
// down should not stop slots being released, and this falling over should not
// stop anyone booking.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/thelong0705/deuce/internal/adapter/postgres"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("sweeper failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}

	var (
		dsn      = env("DB_URL", "postgres://deuce:deuce@localhost:5432/deuce?sslmode=disable")
		interval = envDuration("SWEEP_INTERVAL", time.Minute)
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("parse db config: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}

	sweeper := usecase.NewSweeper(postgres.NewBookingRepository(postgres.New(pool)))

	slog.Info("sweeping", "interval", interval)

	// Once on startup, so a restart does not wait out an interval with slots
	// already overdue.
	sweep(ctx, sweeper)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopped cleanly")
			return nil
		case <-ticker.C:
			sweep(ctx, sweeper)
		}
	}
}

// sweep logs what it did rather than returning it: one bad pass is not a
// reason to stop, since the next one will find the same holds.
func sweep(ctx context.Context, sweeper *usecase.Sweeper) {
	released, err := sweeper.ReleaseLapsedHolds(ctx, time.Now())
	if err != nil {
		if ctx.Err() != nil {
			return
		}

		slog.Error("release lapsed holds", "error", err)
		return
	}

	if released > 0 {
		slog.Info("released lapsed holds", "slots", released)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("ignoring unreadable duration", "key", key, "value", raw, "error", err)
		return fallback
	}

	return d
}
