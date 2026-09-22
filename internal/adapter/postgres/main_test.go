package postgres

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultTestDSN points at a database of the tests' own. They write freely and
// never clean up, so they must not run against the one the server uses.
// Override with TEST_DB_URL.
const defaultTestDSN = "postgres://deuce:deuce@localhost:5432/deuce_test?sslmode=disable"

var (
	testPool    *pgxpool.Pool
	testQueries *Queries
	testStore   *Store
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
	testStore = NewStore(pool)

	code := m.Run()

	// os.Exit does not run deferred calls, so close explicitly.
	pool.Close()
	os.Exit(code)
}
