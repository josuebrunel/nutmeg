package model

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrNotAuthorized    = errors.New("not authorized")
	ErrAlreadyExists    = errors.New("already exists")
	ErrInvalidInput     = errors.New("invalid input")
	ErrMemberNeedsEmail = errors.New("member needs an email before they can be made admin")
	ErrMemberNameTaken  = errors.New("a member with this name already exists")
	ErrNoLinkedAccount  = errors.New("no account is registered with this email yet")

	ErrAlreadyMember         = errors.New("you're already a member of this group")
	ErrRequestAlreadyPending = errors.New("you already have a pending request to join this group")
)
