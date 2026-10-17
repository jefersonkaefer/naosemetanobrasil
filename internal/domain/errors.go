package domain

import "errors"

var (
	ErrInvalid         = errors.New("invalid input")
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("permission denied")
	ErrSelfReview      = errors.New("cannot review own profile")
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrExpired         = errors.New("challenge expired")
	ErrTransition      = errors.New("invalid state transition")
	ErrLimited         = errors.New("rate limit exceeded")
)
