package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrInvalid       = errors.New("invalid")
	ErrUnauthorized  = errors.New("unauthorized")
)
