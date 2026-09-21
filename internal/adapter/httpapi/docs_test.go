package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/thelong0705/deuce/api"
)

// spec is the shape of openapi.yaml this test cares about: which operations
// exist.
type spec struct {
	Paths map[string]map[string]struct {
		Summary string `yaml:"summary"`
	} `yaml:"paths"`
}

// operations returns "METHOD /path" for everything the specification
// documents.
func (s spec) operations() []string {
	var out []string

	for path, methods := range s.Paths {
		for method := range methods {
			out = append(out, strings.ToUpper(method)+" "+path)
		}
	}

	return out
}

// routes returns "METHOD /path" for everything the router serves, with chi's
// trailing slash on nested routes removed so the two sides are comparable.
func routes(t *testing.T) []string {
	t.Helper()

	var out []string

	err := chi.Walk(
		deps{}.handler(t).(*chi.Mux),
		func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			out = append(out, method+" "+strings.TrimSuffix(route, "/"))
			return nil
		},
	)
	require.NoError(t, err)

	return out
}

func readSpec(t *testing.T) spec {
	t.Helper()

	var got spec
	require.NoError(t, yaml.Unmarshal(api.OpenAPISpec, &got))

	return got
}

// A route nobody documented and an operation nobody serves are both bugs, and
// neither shows up in any other test.
func TestOpenAPISpecMatchesTheRouter(t *testing.T) {
	require.ElementsMatch(t, routes(t), readSpec(t).operations())
}

func TestOpenAPISpecDescribesEveryOperation(t *testing.T) {
	for path, methods := range readSpec(t).Paths {
		for method, op := range methods {
			t.Run(strings.ToUpper(method)+" "+path, func(t *testing.T) {
				require.NotEmpty(t, op.Summary, "every operation needs a summary")
			})
		}
	}
}

func TestServesTheSpec(t *testing.T) {
	rec := do(t, deps{}, http.MethodGet, "/openapi.yaml", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))
	require.Equal(t, api.OpenAPISpec, rec.Body.Bytes())
}

func TestServesTheDocsPage(t *testing.T) {
	rec := do(t, deps{}, http.MethodGet, "/docs", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, rec.Body.String(), "/openapi.yaml")
}
