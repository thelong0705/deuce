// Package apperr defines the error type that travels between layers.
//
// An Error carries a Kind that outer layers map to a transport status, a stable
// Code for clients to branch on, and a Message safe to show a user.
package apperr

import "errors"

// Kind groups errors by how a caller should react to them.
type Kind string

const (
	KindInvalid   Kind = "invalid"
	KindNotFound  Kind = "not_found"
	KindConflict  Kind = "conflict"
	KindForbidden Kind = "forbidden"
	KindInternal  Kind = "internal"
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// ErrInternal is the fallback for anything that is not an *Error.
var ErrInternal = New(KindInternal, "internal_error", "internal error")

// New returns an Error. Package-level error values are built with it.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// KindOf returns the Kind of the first *Error in err's chain, or KindInternal
// when there is none.
func KindOf(err error) Kind {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Kind
	}
	return KindInternal
}

// CodeOf returns the Code of the first *Error in err's chain, or an empty
// string when there is none.
func CodeOf(err error) string {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
