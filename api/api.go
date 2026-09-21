// Package api holds the service's public contract.
package api

import _ "embed"

// OpenAPISpec is the specification the server serves at /openapi.yaml, embedded
// so a running server and its documentation cannot come from different places.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
