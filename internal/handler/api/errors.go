package api

import (
	"errors"
	"net/http"

	"github.com/josuebrunel/ezauth"
	"github.com/labstack/echo/v5"

	"nutmeg/internal/model"
)

// ErrorResponse is the JSON body returned by every API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeError maps a service/model-layer error to an HTTP status and writes
// the standard ErrorResponse envelope.
func writeError(c *echo.Context, err error) error {
	return c.JSON(statusFor(err), ErrorResponse{Error: err.Error()})
}

// badRequest writes err's message as a 400 — used for handwritten
// service-layer validation errors (e.g. MatchService.Create/Update) that
// aren't one of the model package's sentinel errors, so statusFor can't
// classify them, but are still always the caller's fault.
func badRequest(c *echo.Context, err error) error {
	return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, model.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrNotAuthorized):
		return http.StatusForbidden
	case errors.Is(err, model.ErrAlreadyExists),
		errors.Is(err, model.ErrAlreadyMember),
		errors.Is(err, model.ErrRequestAlreadyPending),
		errors.Is(err, model.ErrMemberNameTaken):
		return http.StatusConflict
	case errors.Is(err, model.ErrInvalidInput),
		errors.Is(err, model.ErrMemberNeedsEmail),
		errors.Is(err, model.ErrNoLinkedAccount):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// currentUserID extracts the authenticated user's id from context set by
// auth.AuthMiddleware (JWT Bearer). Unlike the HTML handlers,
// ezauth.GetUser(ctx) is NOT available here — AuthMiddleware only sets the
// user-id context key, never the full-user one. Handlers needing the full
// user object must follow up with auth.Repo.UserGetByID(ctx, userID).
func currentUserID(c *echo.Context, auth *ezauth.EzAuth) (string, error) {
	return auth.GetUserID(c.Request().Context())
}
