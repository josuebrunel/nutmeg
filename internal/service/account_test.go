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
}

func (m *mockAccountAuthSvc) UserAuthenticate(ctx context.Context, req ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error) {
	return m.userAuthenticateFn(ctx, req)
}
func (m *mockAccountAuthSvc) UserUpdatePassword(ctx context.Context, user *ezauthmodels.User, password string) (*ezauthmodels.User, error) {
	return m.userUpdatePasswordFn(ctx, user, password)
}

func defaultAccountAuthSvc() *mockAccountAuthSvc {
	return &mockAccountAuthSvc{
		userAuthenticateFn: func(_ context.Context, _ ezauthservice.RequestBasicAuth) (*ezauthmodels.User, error) {
			return &ezauthmodels.User{ID: "user-1", Provider: "local"}, nil
		},
		userUpdatePasswordFn: func(_ context.Context, user *ezauthmodels.User, _ string) (*ezauthmodels.User, error) {
			return user, nil
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
		svc := NewAccountService(auth, defaultAccountAuthSvc())
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "new-password")
		assert.ErrIs(t, err, model.ErrOAuthPasswordChange)
	})

	t.Run("rejects mismatched new passwords", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = localUser
		svc := NewAccountService(auth, defaultAccountAuthSvc())
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
		svc := NewAccountService(auth, authSvc)
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
		svc := NewAccountService(auth, authSvc)
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "new-password")
		assert.ErrIs(t, err, wantErr)
	})

	t.Run("updates the password on success", func(t *testing.T) {
		auth := defaultAuthMock()
		auth.userGetByIDFn = localUser
		svc := NewAccountService(auth, defaultAccountAuthSvc())
		err := svc.ChangePassword(context.Background(), "user-1", "current", "new-password", "new-password")
		assert.NoErr(t, err)
	})
}
