package model

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrNotAuthorized  = errors.New("not authorized")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInvalidInput   = errors.New("invalid input")
)
