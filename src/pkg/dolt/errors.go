package dolt

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrServerError  = errors.New("server error")
	ErrRateLimited  = errors.New("rate limited")
	ErrBadQuery     = errors.New("bad query")
)
