package service

import (
	"context"
	"errors"
	"testing"

	ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"

	"nutmeg/internal/assert"
	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type mockGroupRepo struct {
	createGroupFn      func(ctx context.Context, g *model.Group) error
	getGroupFn         func(ctx context.Context, id string) (*model.Group, error)
	listGroupsFn       func(ctx context.Context, userID string) ([]*model.Group, error)
	getGroupsByIDsFn   func(ctx context.Context, ids []string) ([]*model.Group, error)
	updateGroupFn      func(ctx context.Context, g *model.Group) error
	deleteGroupFn      func(ctx context.Context, id string) error
	addMemberFn        func(ctx context.Context, groupID, name string, phone, email *string, role string) error
	importMemberFn     func(ctx context.Context, groupID, name string, phone, email *string) error
	removeMemberFn     func(ctx context.Context, groupID, memberID string) error
	listMembersFn      func(ctx context.Context, groupID string) ([]repository.MemberInfo, error)
	getMemberFn        func(ctx context.Context, groupID, memberID string) (*model.GroupPlayer, error)
	memberCountFn      func(ctx context.Context, groupID string) (int, error)
	updateMemberRoleFn func(ctx context.Context, groupID, memberID, role string) error
	getMemberByEmailFn func(ctx context.Context, groupID, email string) (*model.GroupPlayer, error)

	createJoinRequestFn       func(ctx context.Context, groupID, userID, name, email string) error
	getPendingJoinRequestFn   func(ctx context.Context, groupID, userID string) (*model.JoinRequest, error)
	getJoinRequestFn          func(ctx context.Context, groupID, requestID string) (*model.JoinRequest, error)
	listPendingJoinRequestsFn func(ctx context.Context, groupID string) ([]repository.JoinRequestInfo, error)
	updateJoinRequestStatusFn func(ctx context.Context, requestID, status string) error
}

func (m *mockGroupRepo) CreateGroup(ctx context.Context, g *model.Group) error {
	return m.createGroupFn(ctx, g)
}
func (m *mockGroupRepo) GetGroup(ctx context.Context, id string) (*model.Group, error) {
	return m.getGroupFn(ctx, id)
}
func (m *mockGroupRepo) ListGroups(ctx context.Context, userID string) ([]*model.Group, error) {
	return m.listGroupsFn(ctx, userID)
}
func (m *mockGroupRepo) GetGroupsByIDs(ctx context.Context, ids []string) ([]*model.Group, error) {
	return m.getGroupsByIDsFn(ctx, ids)
}
func (m *mockGroupRepo) UpdateGroup(ctx context.Context, g *model.Group) error {
	return m.updateGroupFn(ctx, g)
}
func (m *mockGroupRepo) DeleteGroup(ctx context.Context, id string) error {
	return m.deleteGroupFn(ctx, id)
}
func (m *mockGroupRepo) AddMember(ctx context.Context, groupID, name string, phone, email *string, role string) error {
	return m.addMemberFn(ctx, groupID, name, phone, email, role)
}
func (m *mockGroupRepo) ImportMember(ctx context.Context, groupID, name string, phone, email *string) error {
	return m.importMemberFn(ctx, groupID, name, phone, email)
}
func (m *mockGroupRepo) RemoveMember(ctx context.Context, groupID, memberID string) error {
	return m.removeMemberFn(ctx, groupID, memberID)
}
func (m *mockGroupRepo) ListMembers(ctx context.Context, groupID string) ([]repository.MemberInfo, error) {
	return m.listMembersFn(ctx, groupID)
}
func (m *mockGroupRepo) GetMember(ctx context.Context, groupID, memberID string) (*model.GroupPlayer, error) {
	return m.getMemberFn(ctx, groupID, memberID)
}
func (m *mockGroupRepo) MemberCount(ctx context.Context, groupID string) (int, error) {
	return m.memberCountFn(ctx, groupID)
}
func (m *mockGroupRepo) UpdateMemberRole(ctx context.Context, groupID, memberID, role string) error {
	return m.updateMemberRoleFn(ctx, groupID, memberID, role)
}
func (m *mockGroupRepo) GetMemberByEmail(ctx context.Context, groupID, email string) (*model.GroupPlayer, error) {
	return m.getMemberByEmailFn(ctx, groupID, email)
}
func (m *mockGroupRepo) CreateJoinRequest(ctx context.Context, groupID, userID, name, email string) error {
	return m.createJoinRequestFn(ctx, groupID, userID, name, email)
}
func (m *mockGroupRepo) GetPendingJoinRequest(ctx context.Context, groupID, userID string) (*model.JoinRequest, error) {
	return m.getPendingJoinRequestFn(ctx, groupID, userID)
}
func (m *mockGroupRepo) GetJoinRequest(ctx context.Context, groupID, requestID string) (*model.JoinRequest, error) {
	return m.getJoinRequestFn(ctx, groupID, requestID)
}
func (m *mockGroupRepo) ListPendingJoinRequests(ctx context.Context, groupID string) ([]repository.JoinRequestInfo, error) {
	return m.listPendingJoinRequestsFn(ctx, groupID)
}
func (m *mockGroupRepo) UpdateJoinRequestStatus(ctx context.Context, requestID, status string) error {
	return m.updateJoinRequestStatusFn(ctx, requestID, status)
}

type mockAuthUserRepo struct {
	userGetByIDFn    func(ctx context.Context, id string) (*ezauthmodels.User, error)
	userGetByEmailFn func(ctx context.Context, email string) (*ezauthmodels.User, error)
	userUpdateFn     func(ctx context.Context, user *ezauthmodels.User) (*ezauthmodels.User, error)
}

func (m *mockAuthUserRepo) UserGetByID(ctx context.Context, id string) (*ezauthmodels.User, error) {
	return m.userGetByIDFn(ctx, id)
}
func (m *mockAuthUserRepo) UserGetByEmail(ctx context.Context, email string) (*ezauthmodels.User, error) {
	return m.userGetByEmailFn(ctx, email)
}
func (m *mockAuthUserRepo) UserUpdate(ctx context.Context, user *ezauthmodels.User) (*ezauthmodels.User, error) {
	return m.userUpdateFn(ctx, user)
}

func defaultAuthMock() *mockAuthUserRepo {
	return &mockAuthUserRepo{
		userGetByIDFn: func(_ context.Context, id string) (*ezauthmodels.User, error) {
			return &ezauthmodels.User{ID: id}, nil
		},
		userGetByEmailFn: func(_ context.Context, email string) (*ezauthmodels.User, error) {
			return nil, errors.New("not found")
		},
		userUpdateFn: func(_ context.Context, user *ezauthmodels.User) (*ezauthmodels.User, error) {
			return user, nil
		},
	}
}

func defaultMock() *mockGroupRepo {
	return &mockGroupRepo{
		createGroupFn: func(_ context.Context, g *model.Group) error {
			g.ID = "group-1"
			g.CreatedBy = "user-1"
			return nil
		},
		getGroupFn: func(_ context.Context, id string) (*model.Group, error) {
			return &model.Group{ID: id, Name: "test", CreatedBy: "user-1"}, nil
		},
		listGroupsFn: func(_ context.Context, userID string) ([]*model.Group, error) {
			return nil, nil
		},
		updateGroupFn: func(_ context.Context, g *model.Group) error {
			return nil
		},
		deleteGroupFn: func(_ context.Context, id string) error {
			return nil
		},
		addMemberFn: func(_ context.Context, groupID, name string, phone, email *string, role string) error {
			return nil
		},
		removeMemberFn: func(_ context.Context, groupID, memberID string) error {
			return nil
		},
		listMembersFn: func(_ context.Context, groupID string) ([]repository.MemberInfo, error) {
			return nil, nil
		},
		getMemberFn: func(_ context.Context, groupID, memberID string) (*model.GroupPlayer, error) {
			return &model.GroupPlayer{Role: "admin", ID: memberID}, nil
		},
		memberCountFn: func(_ context.Context, groupID string) (int, error) {
			return 0, nil
		},
		getGroupsByIDsFn: func(_ context.Context, ids []string) ([]*model.Group, error) {
			return nil, nil
		},
		updateMemberRoleFn: func(_ context.Context, groupID, memberID, role string) error {
			return nil
		},
		getMemberByEmailFn: func(_ context.Context, groupID, email string) (*model.GroupPlayer, error) {
			return nil, errors.New("not found")
		},
		createJoinRequestFn: func(_ context.Context, groupID, userID, name, email string) error {
			return nil
		},
		getPendingJoinRequestFn: func(_ context.Context, groupID, userID string) (*model.JoinRequest, error) {
			return nil, errors.New("not found")
		},
		getJoinRequestFn: func(_ context.Context, groupID, requestID string) (*model.JoinRequest, error) {
			return &model.JoinRequest{ID: requestID, GroupID: groupID, Status: "pending"}, nil
		},
		listPendingJoinRequestsFn: func(_ context.Context, groupID string) ([]repository.JoinRequestInfo, error) {
			return nil, nil
		},
		updateJoinRequestStatusFn: func(_ context.Context, requestID, status string) error {
			return nil
		},
	}
}

func TestCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		g, err := svc.Create(context.Background(), "test group", nil, "user-1", "Test User", "test@email.com")
		assert.NoErr(t, err)
		assert.NotNil(t, g)
		assert.Eq(t, g.ID, "group-1")
		assert.Eq(t, g.Name, "test group")
	})

	t.Run("createGroupError", func(t *testing.T) {
		m := defaultMock()
		m.createGroupFn = func(_ context.Context, g *model.Group) error {
			return errors.New("db error")
		}
		svc := NewGroupService(m, defaultAuthMock())
		_, err := svc.Create(context.Background(), "test", nil, "user-1", "Test", "t@t.com")
		assert.NotNil(t, err)
	})

	t.Run("addMemberError", func(t *testing.T) {
		m := defaultMock()
		m.addMemberFn = func(_ context.Context, groupID, name string, phone, email *string, role string) error {
			return errors.New("db error")
		}
		svc := NewGroupService(m, defaultAuthMock())
		_, err := svc.Create(context.Background(), "test", nil, "user-1", "Test", "t@t.com")
		assert.NotNil(t, err)
	})
}

func TestGet(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		m := defaultMock()
		m.getGroupFn = func(_ context.Context, id string) (*model.Group, error) {
			return &model.Group{ID: id, Name: "found", CreatedBy: "user-1"}, nil
		}
		svc := NewGroupService(m, defaultAuthMock())
		g, err := svc.Get(context.Background(), "g-1")
		assert.NoErr(t, err)
		assert.Eq(t, g.Name, "found")
	})

	t.Run("notFound", func(t *testing.T) {
		m := defaultMock()
		m.getGroupFn = func(_ context.Context, id string) (*model.Group, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m, defaultAuthMock())
		_, err := svc.Get(context.Background(), "nonexistent")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}

func TestList(t *testing.T) {
	t.Run("hasGroups", func(t *testing.T) {
		m := defaultMock()
		groups := []*model.Group{
			{ID: "g-1", Name: "Group A", CreatedBy: "user-1"},
			{ID: "g-2", Name: "Group B", CreatedBy: "user-1"},
		}
		m.listGroupsFn = func(_ context.Context, userID string) ([]*model.Group, error) {
			return groups, nil
		}
		svc := NewGroupService(m, defaultAuthMock())
		result, err := svc.List(context.Background(), "user-1")
		assert.NoErr(t, err)
		assert.Eq(t, len(result), 2)
	})

	t.Run("empty", func(t *testing.T) {
		m := defaultMock()
		m.listGroupsFn = func(_ context.Context, userID string) ([]*model.Group, error) {
			return []*model.Group{}, nil
		}
		svc := NewGroupService(m, defaultAuthMock())
		result, err := svc.List(context.Background(), "user-1")
		assert.NoErr(t, err)
		assert.Eq(t, len(result), 0)
	})
}

func TestUpdate(t *testing.T) {
	t.Run("creatorCanUpdate", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.Update(context.Background(), &model.Group{ID: "g-1", CreatedBy: "user-1"}, "user-1")
		assert.NoErr(t, err)
	})

	t.Run("nonCreatorCannotUpdate", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.Update(context.Background(), &model.Group{ID: "g-1", CreatedBy: "user-1"}, "other-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})
}

func TestDelete(t *testing.T) {
	t.Run("creatorCanDelete", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.Delete(context.Background(), "g-1", "user-1")
		assert.NoErr(t, err)
	})

	t.Run("nonCreatorCannotDelete", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.Delete(context.Background(), "g-1", "other-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})

	t.Run("notFound", func(t *testing.T) {
		m := defaultMock()
		m.getGroupFn = func(_ context.Context, id string) (*model.Group, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.Delete(context.Background(), "nonexistent", "user-1")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}

func TestAddMember(t *testing.T) {
	t.Run("creatorCanAdd", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.AddMember(context.Background(), "g-1", "New Player", nil, nil, "user-1")
		assert.NoErr(t, err)
	})

	t.Run("emptyName", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.AddMember(context.Background(), "g-1", "", nil, nil, "user-1")
		assert.ErrIs(t, err, model.ErrInvalidInput)
	})

	t.Run("nonCreatorCannotAdd", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.AddMember(context.Background(), "g-1", "New Player", nil, nil, "other-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})
}

func TestRemoveMember(t *testing.T) {
	t.Run("creatorCanRemove", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.RemoveMember(context.Background(), "g-1", "member-1", "user-1")
		assert.NoErr(t, err)
	})

	t.Run("nonCreatorCannotRemove", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.RemoveMember(context.Background(), "g-1", "member-1", "other-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})

	t.Run("memberNotFound", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, memberID string) (*model.GroupPlayer, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m, defaultAuthMock())
		err := svc.RemoveMember(context.Background(), "g-1", "nonexistent", "user-1")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}
