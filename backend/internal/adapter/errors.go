package adapter

import (
	"errors"
)

var (
	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("resource not found")

	// ErrPreconditionFailed is returned when an ETag mismatch occurs.
	ErrPreconditionFailed = errors.New("precondition failed")

	// ErrUnauthorized is returned when the storage backend rejects the user's credentials
	// (e.g. expired or revoked OAuth refresh token).
	ErrUnauthorized = errors.New("storage authentication failed")
)
