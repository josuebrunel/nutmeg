package service

import (
	"context"
	"errors"
	"testing"

	"nutmeg/internal/assert"
	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type mockGroupRepo struct {
	createGroupFn func(ctx context.Context, g *model.Group) error
	getGroupFn    func(ctx context.Context, id string) (*model.Group, error)
	listGroupsFn  func(ctx context.Context, userID string) ([]*model.Group, error)
	updateGroupFn func(ctx context.Context, g *model.Group) error
	deleteGroupFn func(ctx context.Context, id string) error
	addMemberFn   func(ctx context.Context, groupID, userID, role string) error
	removeMemberFn func(ctx context.Context, groupID, userID string) error
	listMembersFn func(ctx context.Context, groupID string) ([]repository.MemberInfo, error)
	getMemberFn   func(ctx context.Context, groupID, userID string) (*model.GroupPlayer, error)
	memberCountFn func(ctx context.Context, groupID string) (int, error)
	getUserByEmailFn func(ctx context.Context, email string) (string, error)
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
func (m *mockGroupRepo) UpdateGroup(ctx context.Context, g *model.Group) error {
	return m.updateGroupFn(ctx, g)
}
func (m *mockGroupRepo) DeleteGroup(ctx context.Context, id string) error {
	return m.deleteGroupFn(ctx, id)
}
func (m *mockGroupRepo) AddMember(ctx context.Context, groupID, userID, role string) error {
	return m.addMemberFn(ctx, groupID, userID, role)
}
func (m *mockGroupRepo) RemoveMember(ctx context.Context, groupID, userID string) error {
	return m.removeMemberFn(ctx, groupID, userID)
}
func (m *mockGroupRepo) ListMembers(ctx context.Context, groupID string) ([]repository.MemberInfo, error) {
	return m.listMembersFn(ctx, groupID)
}
func (m *mockGroupRepo) GetMember(ctx context.Context, groupID, userID string) (*model.GroupPlayer, error) {
	return m.getMemberFn(ctx, groupID, userID)
}
func (m *mockGroupRepo) MemberCount(ctx context.Context, groupID string) (int, error) {
	return m.memberCountFn(ctx, groupID)
}
func (m *mockGroupRepo) GetUserByEmail(ctx context.Context, email string) (string, error) {
	return m.getUserByEmailFn(ctx, email)
}

func defaultMock() *mockGroupRepo {
	return &mockGroupRepo{
		createGroupFn: func(_ context.Context, g *model.Group) error {
			g.ID = "group-1"
			return nil
		},
		getGroupFn: func(_ context.Context, id string) (*model.Group, error) {
			return nil, model.ErrNotFound
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
		addMemberFn: func(_ context.Context, groupID, userID, role string) error {
			return nil
		},
		removeMemberFn: func(_ context.Context, groupID, userID string) error {
			return nil
		},
		listMembersFn: func(_ context.Context, groupID string) ([]repository.MemberInfo, error) {
			return nil, nil
		},
		getMemberFn: func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return &model.GroupPlayer{Role: "admin"}, nil
		},
		memberCountFn: func(_ context.Context, groupID string) (int, error) {
			return 0, nil
		},
		getUserByEmailFn: func(_ context.Context, email string) (string, error) {
			return "user-1", nil
		},
	}
}

func adminMember() *model.GroupPlayer {
	return &model.GroupPlayer{Role: "admin"}
}

func memberMember() *model.GroupPlayer {
	return &model.GroupPlayer{Role: "member"}
}

func TestCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m)
		g, err := svc.Create(context.Background(), "test group", nil, "user-1")
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
		svc := NewGroupService(m)
		_, err := svc.Create(context.Background(), "test", nil, "user-1")
		assert.NotNil(t, err)
	})

	t.Run("addMemberError", func(t *testing.T) {
		m := defaultMock()
		m.addMemberFn = func(_ context.Context, groupID, userID, role string) error {
			return errors.New("db error")
		}
		svc := NewGroupService(m)
		_, err := svc.Create(context.Background(), "test", nil, "user-1")
		assert.NotNil(t, err)
	})
}

func TestGet(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		m := defaultMock()
		m.getGroupFn = func(_ context.Context, id string) (*model.Group, error) {
			return &model.Group{ID: id, Name: "found"}, nil
		}
		svc := NewGroupService(m)
		g, err := svc.Get(context.Background(), "g-1")
		assert.NoErr(t, err)
		assert.Eq(t, g.Name, "found")
	})

	t.Run("notFound", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m)
		_, err := svc.Get(context.Background(), "nonexistent")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}

func TestList(t *testing.T) {
	t.Run("hasGroups", func(t *testing.T) {
		m := defaultMock()
		groups := []*model.Group{
			{ID: "g-1", Name: "Group A"},
			{ID: "g-2", Name: "Group B"},
		}
		m.listGroupsFn = func(_ context.Context, userID string) ([]*model.Group, error) {
			return groups, nil
		}
		svc := NewGroupService(m)
		result, err := svc.List(context.Background(), "user-1")
		assert.NoErr(t, err)
		assert.Eq(t, len(result), 2)
	})

	t.Run("empty", func(t *testing.T) {
		m := defaultMock()
		m.listGroupsFn = func(_ context.Context, userID string) ([]*model.Group, error) {
			return []*model.Group{}, nil
		}
		svc := NewGroupService(m)
		result, err := svc.List(context.Background(), "user-1")
		assert.NoErr(t, err)
		assert.Eq(t, len(result), 0)
	})
}

func TestUpdate(t *testing.T) {
	t.Run("adminCanUpdate", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return adminMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.Update(context.Background(), &model.Group{ID: "g-1"}, "admin-user")
		assert.NoErr(t, err)
	})

	t.Run("memberCannotUpdate", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return memberMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.Update(context.Background(), &model.Group{ID: "g-1"}, "member-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})

	t.Run("nonMemberCannotUpdate", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m)
		err := svc.Update(context.Background(), &model.Group{ID: "g-1"}, "unknown")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}

func TestDelete(t *testing.T) {
	t.Run("adminCanDelete", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return adminMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.Delete(context.Background(), "g-1", "admin-user")
		assert.NoErr(t, err)
	})

	t.Run("memberCannotDelete", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return memberMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.Delete(context.Background(), "g-1", "member-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})

	t.Run("nonMemberCannotDelete", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m)
		err := svc.Delete(context.Background(), "g-1", "unknown")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}

func TestAddMember(t *testing.T) {
	t.Run("adminCanAdd", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return adminMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.AddMember(context.Background(), "g-1", "new-user", "admin-user")
		assert.NoErr(t, err)
	})

	t.Run("emptyUserID", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m)
		err := svc.AddMember(context.Background(), "g-1", "", "admin-user")
		assert.ErrIs(t, err, model.ErrInvalidInput)
	})

	t.Run("memberCannotAdd", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return memberMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.AddMember(context.Background(), "g-1", "new-user", "member-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})

	t.Run("nonMemberCannotAdd", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m)
		err := svc.AddMember(context.Background(), "g-1", "new-user", "unknown")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}

func TestRemoveMember(t *testing.T) {
	t.Run("adminCanRemove", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return adminMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.RemoveMember(context.Background(), "g-1", "other-user", "admin-user")
		assert.NoErr(t, err)
	})

	t.Run("cannotRemoveSelf", func(t *testing.T) {
		m := defaultMock()
		svc := NewGroupService(m)
		err := svc.RemoveMember(context.Background(), "g-1", "user-1", "user-1")
		assert.NotNil(t, err)
		assert.StrContains(t, err.Error(), "cannot remove yourself")
	})

	t.Run("memberCannotRemove", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return memberMember(), nil
		}
		svc := NewGroupService(m)
		err := svc.RemoveMember(context.Background(), "g-1", "other-user", "member-user")
		assert.ErrIs(t, err, model.ErrNotAuthorized)
	})

	t.Run("nonMemberCannotRemove", func(t *testing.T) {
		m := defaultMock()
		m.getMemberFn = func(_ context.Context, groupID, userID string) (*model.GroupPlayer, error) {
			return nil, model.ErrNotFound
		}
		svc := NewGroupService(m)
		err := svc.RemoveMember(context.Background(), "g-1", "other-user", "unknown")
		assert.ErrIs(t, err, model.ErrNotFound)
	})
}
