package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

var errTaken = apperr.New(apperr.KindConflict, "email_taken", "email already registered")

func TestErrorMessage(t *testing.T) {
	require.Equal(t, "email already registered", errTaken.Error())
}

func TestErrorsIsMatchesTheSentinel(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "the sentinel itself", err: errTaken, want: true},
		{name: "wrapped once", err: fmt.Errorf("create user: %w", errTaken), want: true},
		{name: "wrapped twice", err: fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", errTaken)), want: true},
		{name: "a different apperr", err: apperr.New(apperr.KindConflict, "email_taken", "email already registered")},
		{name: "a plain error", err: errors.New("boom")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, errors.Is(tt.err, errTaken))
		})
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want apperr.Kind
	}{
		{name: "an apperr", err: errTaken, want: apperr.KindConflict},
		{name: "a wrapped apperr", err: fmt.Errorf("create user: %w", errTaken), want: apperr.KindConflict},
		{name: "a plain error", err: errors.New("boom"), want: apperr.KindInternal},
		{name: "nil", err: nil, want: apperr.KindInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, apperr.KindOf(tt.err))
		})
	}
}

func TestCodeOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "an apperr", err: errTaken, want: "email_taken"},
		{name: "a wrapped apperr", err: fmt.Errorf("create user: %w", errTaken), want: "email_taken"},
		{name: "a plain error", err: errors.New("boom"), want: ""},
		{name: "nil", err: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, apperr.CodeOf(tt.err))
		})
	}
}

func TestKindsAreDistinct(t *testing.T) {
	kinds := []apperr.Kind{
		apperr.KindInvalid,
		apperr.KindNotFound,
		apperr.KindConflict,
		apperr.KindUnauthorized,
		apperr.KindForbidden,
		apperr.KindInternal,
	}

	seen := make(map[apperr.Kind]bool, len(kinds))
	for _, k := range kinds {
		require.False(t, seen[k], "duplicate kind %q", k)
		seen[k] = true
	}
}

func TestKindOfUnauthorized(t *testing.T) {
	err := apperr.New(apperr.KindUnauthorized, "no_session", "not signed in")

	require.Equal(t, apperr.KindUnauthorized, apperr.KindOf(err))
	require.Equal(t, apperr.KindUnauthorized, apperr.KindOf(fmt.Errorf("wrapped: %w", err)))
}
