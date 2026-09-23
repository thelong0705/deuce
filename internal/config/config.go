// Package config reads what the commands need from the environment.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// file is optional and never overwrites a variable the shell already set.
// .env.example is the committed reference for what belongs in it; deployments
// take their environment from the platform and ship without either.
const file = ".env"

// Server is what the API reads. Anything without a default has to be supplied,
// so a container started without its environment says which key is missing
// rather than quietly dialling localhost.
type Server struct {
	DBURL               string   `envconfig:"DB_URL" required:"true"`
	RedisAddr           string   `envconfig:"REDIS_ADDR" required:"true"`
	StripeSecretKey     string   `envconfig:"STRIPE_SECRET_KEY" required:"true"`
	StripeWebhookSecret string   `envconfig:"STRIPE_WEBHOOK_SECRET" required:"true"`
	CORSOrigins         []string `envconfig:"CORS_ORIGINS"`
	HTTPAddr            string   `envconfig:"HTTP_ADDR" default:":8080"`
	Port                string   `envconfig:"PORT"`
}

// Sweeper is what the worker reads.
type Sweeper struct {
	DBURL    string        `envconfig:"DB_URL" required:"true"`
	Interval time.Duration `envconfig:"SWEEP_INTERVAL" default:"1m"`
}

func LoadServer() (Server, error) {
	var c Server
	return c, load(&c)
}

func LoadSweeper() (Sweeper, error) {
	var c Sweeper
	return c, load(&c)
}

func load(spec any) error {
	if err := godotenv.Load(file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load %s: %w", file, err)
	}

	return envconfig.Process("", spec)
}

// Addr prefers PORT, which Cloud Run and similar platforms set to tell the
// container where to listen. Kubernetes injects PORT too when a Service is
// named "port", and its value is a URL rather than a number, so anything that
// is not a plain port number is ignored.
func (s Server) Addr() string {
	if s.Port == "" {
		return s.HTTPAddr
	}

	if n, err := strconv.Atoi(s.Port); err == nil && n > 0 && n < 65536 {
		return ":" + s.Port
	}

	slog.Warn("ignoring unusable PORT", "port", s.Port)

	return s.HTTPAddr
}
