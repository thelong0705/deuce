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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	rediscache "github.com/thelong0705/deuce/internal/adapter/cache/redis"
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

const (
	sessionTTL = 7 * 24 * time.Hour
	// sessionCacheTTL bounds how long a deactivated account keeps working:
	// nothing evicts on deactivation, so the entry has to lapse on its own.
	sessionCacheTTL = 10 * time.Minute
)

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}

	var (
		dsn       = env("DB_URL", "postgres://deuce:deuce@localhost:5432/deuce?sslmode=disable")
		addr      = listenAddr()
		redisAddr = env("REDIS_ADDR", "localhost:6379")
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

	rdb := redis.NewClient(rediscache.Options(redisAddr))
	defer func() { _ = rdb.Close() }()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}

	var (
		queries     = postgres.New(pool)
		userRepo    = postgres.NewUserRepository(queries)
		venueRepo   = postgres.NewVenueRepository(queries)
		courtRepo   = postgres.NewCourtRepository(queries)
		bookingRepo = postgres.NewBookingRepository(queries)
		sessionRepo = postgres.NewSessionRepository(queries)
		cityRepo    = postgres.NewCityRepository(queries)
		hasher      = crypto.NewBcryptHasher()
		cache       = rediscache.NewSessionCache(rdb, sessionCacheTTL)
		userUC      = usecase.NewUser(userRepo, hasher, userRepo, sessionRepo, cache, sessionTTL)
		venueUC     = usecase.NewVenue(venueRepo, userRepo)
		courtUC     = usecase.NewCourt(courtRepo, venueRepo, bookingRepo)
		bookingUC   = usecase.NewBooking(bookingRepo, bookingRepo, userRepo)
		cityUC      = usecase.NewCity(cityRepo)
		api         = httpapi.NewServer(userUC, venueUC, courtUC, bookingUC, cityUC)
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

// listenAddr prefers PORT, which Cloud Run and similar platforms set to tell
// the container where to listen. Kubernetes injects PORT too when a Service is
// named "port", and its value is a URL rather than a number, so anything that
// is not a plain port number is ignored.
func listenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		if n, err := strconv.Atoi(port); err == nil && n > 0 && n < 65536 {
			return ":" + port
		}
		slog.Warn("ignoring unusable PORT", "port", port)
	}

	return env("HTTP_ADDR", ":8080")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
