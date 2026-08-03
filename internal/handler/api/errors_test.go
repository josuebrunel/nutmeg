package api

import (
	"errors"
	"net/http"
	"testing"

	"nutmeg/internal/assert"
	"nutmeg/internal/model"
)

func TestStatusFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"NotFound", model.ErrNotFound, http.StatusNotFound},
		{"NotAuthorized", model.ErrNotAuthorized, http.StatusForbidden},
		{"AlreadyExists", model.ErrAlreadyExists, http.StatusConflict},
		{"AlreadyMember", model.ErrAlreadyMember, http.StatusConflict},
		{"RequestAlreadyPending", model.ErrRequestAlreadyPending, http.StatusConflict},
		{"MemberNameTaken", model.ErrMemberNameTaken, http.StatusConflict},
		{"InvalidInput", model.ErrInvalidInput, http.StatusBadRequest},
		{"MemberNeedsEmail", model.ErrMemberNeedsEmail, http.StatusBadRequest},
		{"NoLinkedAccount", model.ErrNoLinkedAccount, http.StatusBadRequest},
		{"unrecognized error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Eq(t, statusFor(tc.err), tc.want)
		})
	}
}
