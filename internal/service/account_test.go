package service

import (
	"context"
	"errors"
	"testing"

	ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"
	ezauthservice "github.com/josuebrunel/ezauth/pkg/service"

	"nutmeg/internal/assert"
	"nutmeg/internal/model"
)

type mockAccountAuthSvc struct {
	userAuthenticateFn   func(ctx context.Context, req ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error)
	userUpdatePasswordFn func(ctx context.Context, user *ezauthmodels.User, password string) (*ezauthmodels.User, error)
	userDeleteFn         func(ctx context.Context, id string) error
}

func (m *mockAccountAuthSvc) UserAuthenticate(ctx context.Context, req ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error) {
	return m.userAuthenticateFn(ctx, req)
}
func (m *mockAccountAuthSvc) UserUpdatePassword(ctx context.Context, user *ezauthmodels.User, password string) (*ezauthmodels.User, error) {
	return m.userUpdatePasswordFn(ctx, user, password)
}
func (m *mockAccountAuthSvc) UserDelete(ctx context.Context, id string) error {
	return m.userDeleteFn(ctx, id)
}

func defaultAccountAuthSvc() *mockAccountAuthSvc {
	return &mockAccountAuthSvc{
		userAuthenticateFn: func(_ context.Context, _ ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error) {
			return &ezauthmodels.User{ID: "user-1", Provider: "local"}, nil
		},
		userUpdatePasswordFn: func(_ context.Context, user *ezauthmodels.User, _ string) (*ezauthmodels.User, error) {
			return user, nil
		},
		userDeleteFn: func(_ context.Context, _ string) error {
			return nil
		},
	}
}

type mockAccountRepo struct {
	listGroupsFn                    func(ctx context.Context, userID string) ([]*model.Group, error)
	deletePendingJoinRequestsByUser func(ctx context.Context, userID string) error
}

func (m *mockAccountRepo) ListGroups(ctx context.Context, userID string) ([]*model.Group, error) {
	return m.listGroupsFn(ctx, userID)
}
func (m *mockAccountRepo) DeletePendingJoinRequestsByUser(ctx context.Context, userID string) error {
	return m.deletePendingJoinRequestsByUser(ctx, userID)
}

func defaultAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		listGroupsFn: func(_ context.Context, _ string) ([]*model.Group, error) {
			return nil, nil
		},
		deletePendingJoinRequestsByUser: func(_ context.Context, _ string) error {
			return nil
		},
	}
}

func TestChangePassword(t *testing.T) {
	localUser := func(_ context.Context, id string) (*ezauthmodels.User, error) {
		return &ezauthmodels.User{ID: id, Email: id + "@example.com", Provider: "local"}, nil
	}

	t.Run("rejects OAuth accounts", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = func(_ context.Context, id string) (*ezauthmodels.User, error) {
			return &ezauthmodels.User{ID: id, Provider: "google"}, nil
		}
		svc := NewAccountService(defaultAccountRepo(), auth, defaultAccountAuthSvc())
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "new-password")
		assert.ErrIs(t, err, model.ErrOAuthPasswordChange)
	})

	t.Run("rejects mismatched new passwords", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = localUser
		svc := NewAccountService(defaultAccountRepo(), auth, defaultAccountAuthSvc())
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "different")
		assert.ErrIs(t, err, model.ErrPasswordMismatch)
	})

	t.Run("rejects an incorrect current password", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = localUser
		authSvc := defaultAccountAuthSvc()
		authSvc.userAuthenticateFn = func(_ context.Context, _ ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error) {
			return nil, errors.New("invalid credentials")
		}
		svc := NewAccountService(defaultAccountRepo(), auth, authSvc)
		err := svc.ChangePassword(context.Background(), "user-1", "wrong", "new-password", "new-password")
		assert.ErrIs(t, err, model.ErrCurrentPasswordIncorrect)
	})

	t.Run("propagates the update failure", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = localUser
		authSvc := defaultAccountAuthSvc()
		wantErr := errors.New("db write failed")
		authSvc.userUpdatePasswordFn = func(_ context.Context, _ *ezauthmodels.User, _ string) (*ezauthmodels.User, error) {
			return nil, wantErr
		}
		svc := NewAccountService(defaultAccountRepo(), auth, authSvc)
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "new-password")
		assert.ErrIs(t, err, wantErr)
	})

	t.Run("updates the password on success", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = localUser
		svc := NewAccountService(defaultAccountRepo(), auth, defaultAccountAuthSvc())
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "new-password")
		assert.NoErr(t, err)
	})
}

func TestOwnedGroupCount(t *testing.T) {
	repo := defaultAccountRepo()
	repo.listGroupsFn = func(_ context.Context, _ string) ([]*model.Group, error) {
		return []*model.Group{{ID: "g-1"}, {ID: "g-2"}}, nil
	}
	svc := NewAccountService(repo, defaultAuthMock(), defaultAccountAuthSvc())
	count, err := svc.OwnedGroupCount(context.Background(), "user-1")
	assert.NoErr(t, err)
	assert.Eq(t, count, 2)
}

func TestDeleteAccount(t *testing.T) {
	t.Run("blocks deletion when the user owns groups", func(t *testing.T) {
		repo := defaultAccountRepo()
		repo.listGroupsFn = func(_ context.Context, _ string) ([]*model.Group, error) {
			return []*model.Group{{ID: "g-1"}}, nil
		}
		svc := NewAccountService(repo, defaultAuthMock(), defaultAccountAuthSvc())
		err := svc.DeleteAccount(context.Background(), "user-1")
		assert.ErrIs(t, err, model.ErrOwnsGroups)
	})

	t.Run("propagates a failure to clear pending join requests", func(t *testing.T) {
		repo := defaultAccountRepo()
		wantErr := errors.New("db write failed")
		repo.deletePendingJoinRequestsByUser = func(_ context.Context, _ string) error {
			return wantErr
		}
		svc := NewAccountService(repo, defaultAuthMock(), defaultAccountAuthSvc())
		err := svc.DeleteAccount(context.Background(), "user-1")
		assert.ErrIs(t, err, wantErr)
	})

	t.Run("propagates a failure from ezauth's user deletion", func(t *testing.T) {
		authSvc := defaultAccountAuthSvc()
		wantErr := errors.New("db write failed")
		authSvc.userDeleteFn = func(_ context.Context, _ string) error {
			return wantErr
		}
		svc := NewAccountService(defaultAccountRepo(), defaultAuthMock(), authSvc)
		err := svc.DeleteAccount(context.Background(), "user-1")
		assert.ErrIs(t, err, wantErr)
	})

	t.Run("clears pending join requests then deletes the account", func(t *testing.T) {
		var cleared, deleted bool
		repo := defaultAccountRepo()
		repo.deletePendingJoinRequestsByUser = func(_ context.Context, _ string) error {
			cleared = true
			return nil
		}
		authSvc := defaultAccountAuthSvc()
		authSvc.userDeleteFn = func(_ context.Context, _ string) error {
			deleted = true
			return nil
		}
		svc := NewAccountService(repo, defaultAuthMock(), authSvc)
		err := svc.DeleteAccount(context.Background(), "user-1")
		assert.NoErr(t, err)
		assert.True(t, cleared)
		assert.True(t, deleted)
	})
}
