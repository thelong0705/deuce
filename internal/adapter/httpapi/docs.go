package httpapi

import (
	"net/http"
	"strconv"

	"github.com/thelong0705/deuce/api"
)

// swaggerUIVersion pins the CDN bundle; unpinned, a Swagger UI release could
// change the docs page without anything in this repo changing.
const swaggerUIVersion = "5.17.14"

var docsPage = []byte(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>deuce API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger"></div>
    <script src="https://unpkg.com/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui-bundle.js" crossorigin></script>
    <script>
      window.onload = () => {
        SwaggerUIBundle({
          url: "/openapi.yaml",
          dom_id: "#swagger",
          // The session cookie is httpOnly, so "try it out" has to send
          // credentials for anything past sign-in to work.
          requestInterceptor: (req) => {
            req.credentials = "same-origin"
            return req
          },
        })
      }
    </script>
  </body>
</html>
`)

func (s *Server) openAPI(w http.ResponseWriter, _ *http.Request) {
	writeStatic(w, "application/yaml", api.OpenAPISpec)
}

func (s *Server) docs(w http.ResponseWriter, _ *http.Request) {
	writeStatic(w, "text/html; charset=utf-8", docsPage)
}

func writeStatic(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
