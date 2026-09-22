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
	"github.com/redis/go-redis/v9"

	"github.com/thelong0705/deuce/internal/adapter/crypto"
	"github.com/thelong0705/deuce/internal/adapter/httpapi"
	"github.com/thelong0705/deuce/internal/adapter/postgres"
	"github.com/thelong0705/deuce/internal/adapter/rediscache"
	"github.com/thelong0705/deuce/internal/domain/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

const (
	sessionTTL = 7 * 24 * time.Hour
	// sessionCacheTTL bounds how long a deactivated account can keep using a
	// session it had already made requests with: nothing evicts on
	// deactivation, so the entry has to lapse on its own.
	sessionCacheTTL = 30 * time.Second
)

func run() error {
	var (
		dsn       = env("DB_URL", "postgres://deuce:deuce@localhost:5432/deuce?sslmode=disable")
		addr      = env("HTTP_ADDR", ":8080")
		redisAddr = os.Getenv("REDIS_ADDR")
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

	var (
		queries     = postgres.New(pool)
		userRepo    = postgres.NewUserRepository(queries)
		venueRepo   = postgres.NewVenueRepository(queries)
		courtRepo   = postgres.NewCourtRepository(queries)
		bookingRepo = postgres.NewBookingRepository(queries)
		sessionRepo = postgres.NewSessionRepository(queries)
		hasher      = crypto.NewBcryptHasher()
	)

	// The cache is a decorator on the same port, so leaving REDIS_ADDR unset
	// takes it out of the picture entirely and nothing else changes.
	sessions := sessionStore(ctx, sessionRepo, redisAddr)

	var (
		userUC    = usecase.NewUser(userRepo, hasher, userRepo, sessions, sessionTTL)
		venueUC   = usecase.NewVenue(venueRepo, userRepo)
		courtUC   = usecase.NewCourt(courtRepo, venueRepo)
		bookingUC = usecase.NewBooking(bookingRepo, bookingRepo, userRepo)
		api       = httpapi.NewServer(userUC, venueUC, courtUC, bookingUC)
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

// sessionStore puts Redis in front of the given store when REDIS_ADDR is set.
// An unreachable Redis is logged and skipped rather than fatal: the cache is an
// optimisation, and refusing to start without it would make the server less
// available than it was before the cache existed.
func sessionStore(ctx context.Context, inner usecase.SessionStore, redisAddr string) usecase.SessionStore {
	if redisAddr == "" {
		slog.Info("session cache disabled", "reason", "REDIS_ADDR not set")
		return inner
	}

	client := redis.NewClient(rediscache.Options(redisAddr))
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Error("session cache disabled", "addr", redisAddr, "error", err)
		_ = client.Close()
		return inner
	}

	slog.Info("session cache enabled", "addr", redisAddr, "ttl", sessionCacheTTL)

	return rediscache.NewSessionStore(inner, client, sessionCacheTTL)
}
