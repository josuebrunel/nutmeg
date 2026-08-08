package service

import (
	"context"
	"fmt"

	ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"
	ezauthservice "github.com/josuebrunel/ezauth/pkg/service"

	"nutmeg/internal/model"
)

// AccountAuthService is the subset of ezauth's Auth service AccountService
// needs to authenticate, mutate, and remove a user's account.
type AccountAuthService interface {
	UserAuthenticate(ctx context.Context, req ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error)
	UserUpdatePassword(ctx context.Context, user *ezauthmodels.User, password string) (*ezauthmodels.User, error)
	UserDelete(ctx context.Context, id string) error
}

// AccountRepository is the subset of the app repository AccountService needs
// to enforce account-deletion rules and clean up app data tied to a user.
type AccountRepository interface {
	ListGroups(ctx context.Context, userID string) ([]*model.Group, error)
	DeletePendingJoinRequestsByUser(ctx context.Context, userID string) error
}

type AccountService struct {
	repo     AccountRepository
	authRepo AuthUserRepository
	authSvc  AccountAuthService
}

func NewAccountService(repo AccountRepository, authRepo AuthUserRepository, authSvc AccountAuthService) *AccountService {
	return &AccountService{repo: repo, authRepo: authRepo, authSvc: authSvc}
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

// OwnedGroupCount reports how many groups userID owns — the account page
// uses this to show a "delete or transfer your groups first" notice instead
// of the delete-account form when it's non-zero.
func (s *AccountService) OwnedGroupCount(ctx context.Context, userID string) (int, error) {
	owned, err := s.repo.ListGroups(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("owned group count: %w", err)
	}
	return len(owned), nil
}

// DeleteAccount permanently removes userID's account. It refuses to do so
// while the user still owns any group — group ownership has no transfer
// mechanism yet, so deleting an owner out from under a group's roster would
// orphan it for every other member. Once clear, it cancels the user's own
// pending join requests (an admin approving a request for a deleted account
// would have nothing to act on) and removes the account via ezauth. Matches
// and group rosters/stats the user is a member of, but doesn't own, are
// left intact as shared group history.
func (s *AccountService) DeleteAccount(ctx context.Context, userID string) error {
	owned, err := s.repo.ListGroups(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete account: list groups: %w", err)
	}
	if len(owned) > 0 {
		return model.ErrOwnsGroups
	}

	if err := s.repo.DeletePendingJoinRequestsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete account: clear pending join requests: %w", err)
	}

	if err := s.authSvc.UserDelete(ctx, userID); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}
