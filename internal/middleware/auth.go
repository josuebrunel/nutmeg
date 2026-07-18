package middleware

import (
	"github.com/josuebrunel/ezauth"
)

type AuthMiddleware struct {
	auth *ezauth.EzAuth
}

func NewAuthMiddleware(auth *ezauth.EzAuth) *AuthMiddleware {
	return &AuthMiddleware{auth: auth}
}
