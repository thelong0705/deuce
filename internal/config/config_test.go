package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

// every key either command reads, cleared before each case so the developer's
// own environment cannot decide what a test sees.
var allKeys = []string{
	"DB_URL", "REDIS_ADDR", "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET",
	"CORS_ORIGINS", "HTTP_ADDR", "PORT", "SWEEP_INTERVAL",
}

// secrets is what a Server needs beyond the settings a case sets itself.
const secrets = "STRIPE_SECRET_KEY=sk_test\nSTRIPE_WEBHOOK_SECRET=whsec_test\n"

func TestLoadServer(t *testing.T) {
	tests := map[string]struct {
		file    string
		env     map[string]string
		want    Server
		wantErr string
	}{
		"the file supplies everything": {
			file: secrets + "DB_URL=postgres://local\nREDIS_ADDR=localhost:6379\n",
			want: Server{
				DBURL:               "postgres://local",
				RedisAddr:           "localhost:6379",
				StripeSecretKey:     "sk_test",
				StripeWebhookSecret: "whsec_test",
				HTTPAddr:            ":8080",
			},
		},
		// Which is what lets a deployment ignore the file it never ships with.
		"the environment wins over the file": {
			file: secrets + "DB_URL=postgres://local\nREDIS_ADDR=localhost:6379\n",
			env:  map[string]string{"DB_URL": "postgres://elsewhere"},
			want: Server{
				DBURL:               "postgres://elsewhere",
				RedisAddr:           "localhost:6379",
				StripeSecretKey:     "sk_test",
				StripeWebhookSecret: "whsec_test",
				HTTPAddr:            ":8080",
			},
		},
		// Comma-separated, and envconfig keeps whatever sits either side of a
		// comma, so a space would be part of the origin.
		"origins split on the comma": {
			file: secrets + "DB_URL=postgres://local\nREDIS_ADDR=localhost:6379\n" +
				"CORS_ORIGINS=http://localhost:5173,https://deuce.dev\n",
			want: Server{
				DBURL:               "postgres://local",
				RedisAddr:           "localhost:6379",
				StripeSecretKey:     "sk_test",
				StripeWebhookSecret: "whsec_test",
				CORSOrigins:         []string{"http://localhost:5173", "https://deuce.dev"},
				HTTPAddr:            ":8080",
			},
		},
		"a missing secret names itself": {
			file:    "DB_URL=postgres://local\nREDIS_ADDR=localhost:6379\n",
			wantErr: "required key STRIPE_SECRET_KEY missing value",
		},
		"no file at all": {
			wantErr: "required key DB_URL missing value",
		},
		"an unreadable file": {
			file:    "this is not an assignment\n",
			wantErr: "load .env",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inDir(t, tc.file, tc.env)

			got, err := LoadServer()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLoadSweeper(t *testing.T) {
	tests := map[string]struct {
		file    string
		want    Sweeper
		wantErr string
	}{
		"an interval of its own": {
			file: "DB_URL=postgres://local\nSWEEP_INTERVAL=5s\n",
			want: Sweeper{DBURL: "postgres://local", Interval: 5 * time.Second},
		},
		"falls back to a minute": {
			file: "DB_URL=postgres://local\n",
			want: Sweeper{DBURL: "postgres://local", Interval: time.Minute},
		},
		// Refusing to start beats sweeping every minute when the operator
		// asked for every ten seconds.
		"an unreadable interval": {
			file:    "DB_URL=postgres://local\nSWEEP_INTERVAL=10 seconds\n",
			wantErr: "SWEEP_INTERVAL",
		},
		"no database": {
			wantErr: "required key DB_URL missing value",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inDir(t, tc.file, nil)

			got, err := LoadSweeper()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestServerAddr(t *testing.T) {
	tests := map[string]struct {
		server Server
		want   string
	}{
		"PORT wins":             {server: Server{Port: "3000", HTTPAddr: ":8080"}, want: ":3000"},
		"no PORT":               {server: Server{HTTPAddr: ":9090"}, want: ":9090"},
		"PORT as a service URL": {server: Server{Port: "tcp://10.0.0.1:8080", HTTPAddr: ":8080"}, want: ":8080"},
		"PORT out of range":     {server: Server{Port: "70000", HTTPAddr: ":8080"}, want: ":8080"},
		"PORT of zero":          {server: Server{Port: "0", HTTPAddr: ":8080"}, want: ":8080"},
		"neither":               {server: Server{}, want: ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.server.Addr())
		})
	}
}

// TestExampleListsEveryRequiredKey keeps .env.example honest. Nothing loads it,
// so a new required setting would otherwise go missing until someone cloned the
// repo and could not start the server.
func TestExampleListsEveryRequiredKey(t *testing.T) {
	example, err := godotenv.Read(filepath.Join("..", "..", ".env.example"))
	require.NoError(t, err)

	for _, key := range requiredKeys(Server{}, Sweeper{}) {
		require.Contains(t, example, key, ".env.example does not mention %s", key)
	}
}

func requiredKeys(specs ...any) []string {
	var keys []string

	for _, spec := range specs {
		typ := reflect.TypeOf(spec)

		for i := range typ.NumField() {
			if field := typ.Field(i); field.Tag.Get("required") == "true" {
				keys = append(keys, field.Tag.Get("envconfig"))
			}
		}
	}

	return keys
}

// inDir moves the test into an empty directory holding at most a .env, so Load
// reads what the case wrote and nothing else.
func inDir(t *testing.T, file string, env map[string]string) {
	t.Helper()

	dir := t.TempDir()
	if file != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte(file), 0o600))
	}

	// Unset, not blank: godotenv skips a variable that exists, empty or not,
	// and envconfig counts one as supplied.
	for _, key := range allKeys {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}

	for key, value := range env {
		t.Setenv(key, value)
	}

	t.Chdir(dir)
}
