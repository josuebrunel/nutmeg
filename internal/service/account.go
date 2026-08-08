package service

import (
	"context"
	"fmt"

	ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"
	ezauthservice "github.com/josuebrunel/ezauth/pkg/service"

	"nutmeg/internal/model"
)

// AccountAuthService is the subset of ezauth's Auth service AccountService
// needs to authenticate and mutate a user's account.
type AccountAuthService interface {
	UserAuthenticate(ctx context.Context, req ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error)
	UserUpdatePassword(ctx context.Context, user *ezauthmodels.User, password string) (*ezauthmodels.User, error)
}

type AccountService struct {
	authRepo AuthUserRepository
	authSvc  AccountAuthService
}

func NewAccountService(authRepo AuthUserRepository, authSvc AccountAuthService) *AccountService {
	return &AccountService{authRepo: authRepo, authSvc: authSvc}
}

// ChangePassword updates userID's password after verifying currentPassword
// against their account and that newPassword/confirmPassword match. OAuth
// accounts (no local password) are rejected outright.
func (s *AccountService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword, confirmPassword string) error {
	user, err := s.authRepo.UserGetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("change password: fetch user: %w", err)
	}

	if user.IsOAuth() {
		return model.ErrOAuthPasswordChange
	}

	if newPassword != confirmPassword {
		return model.ErrPasswordMismatch
	}

	if _, err := s.authSvc.UserAuthenticate(ctx, ezauthservice.RequestBasicAuth{Email: user.Email, Password: currentPassword}); err != nil {
		return model.ErrCurrentPasswordIncorrect
	}

	if _, err := s.authSvc.UserUpdatePassword(ctx, user, newPassword); err != nil {
		return fmt.Errorf("change password: update: %w", err)
	}
	return nil
}
