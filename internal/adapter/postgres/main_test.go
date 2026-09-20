package postgres

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTestDSN = "postgres://deuce:deuce@localhost:5432/deuce?sslmode=disable"

var (
	testPool    *pgxpool.Pool
	testQueries *Queries
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DB_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("parse db config: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("connect to db: %v", err)
	}

	testPool = pool
	testQueries = New(pool)

	code := m.Run()

	// os.Exit does not run deferred calls, so close explicitly.
	pool.Close()
	os.Exit(code)
}
