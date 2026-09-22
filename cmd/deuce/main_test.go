package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListenAddr(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		httpAddr string
		want     string
	}{
		{
			name: "the default when nothing is set",
			want: ":8080",
		},
		{
			name: "PORT wins",
			port: "9090",
			want: ":9090",
		},
		{
			name:     "PORT beats HTTP_ADDR",
			port:     "9090",
			httpAddr: ":7070",
			want:     ":9090",
		},
		{
			name:     "HTTP_ADDR when PORT is unset",
			httpAddr: ":7070",
			want:     ":7070",
		},
		{
			// Kubernetes sets this when a Service is named "port".
			name:     "a URL in PORT is ignored",
			port:     "tcp://10.0.0.1:8080",
			httpAddr: ":7070",
			want:     ":7070",
		},
		{
			name: "a port out of range is ignored",
			port: "70000",
			want: ":8080",
		},
		{
			name: "an empty PORT falls through",
			port: "",
			want: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", tt.port)
			t.Setenv("HTTP_ADDR", tt.httpAddr)

			require.Equal(t, tt.want, listenAddr())
		})
	}
}
