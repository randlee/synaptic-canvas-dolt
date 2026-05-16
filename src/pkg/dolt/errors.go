package dolt

import "errors"

var (
	ErrNotFound           = errors.New("dolt: not found")
	ErrUnauthorized       = errors.New("dolt: unauthorized")
	ErrServerError        = errors.New("dolt: server error")
	ErrRateLimited        = errors.New("dolt: rate limited")
	ErrBadQuery           = errors.New("dolt: bad query")
	ErrUnsupportedBackend = errors.New("dolt: unsupported backend")
)
