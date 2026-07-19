package service

import (
	"context"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type GroupRepository interface {
	CreateGroup(ctx context.Context, g *model.Group) error
	GetGroup(ctx context.Context, id string) (*model.Group, error)
	ListGroups(ctx context.Context, userID string) ([]*model.Group, error)
	UpdateGroup(ctx context.Context, g *model.Group) error
	DeleteGroup(ctx context.Context, id string) error
	AddMember(ctx context.Context, groupID, name string, phone, email *string, role string) error
	RemoveMember(ctx context.Context, groupID, memberID string) error
	ListMembers(ctx context.Context, groupID string) ([]repository.MemberInfo, error)
	GetMember(ctx context.Context, groupID, memberID string) (*model.GroupPlayer, error)
	MemberCount(ctx context.Context, groupID string) (int, error)
}

type GroupService struct {
	repo GroupRepository
}

func NewGroupService(repo GroupRepository) *GroupService {
	return &GroupService{repo: repo}
}

func (s *GroupService) Create(ctx context.Context, name string, description *string, userID string, creatorName, creatorEmail string) (*model.Group, error) {
	g := &model.Group{
		Name:        name,
		Description: description,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateGroup(ctx, g); err != nil {
		return nil, err
	}
	if err := s.repo.AddMember(ctx, g.ID, creatorName, nil, &creatorEmail, "admin"); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *GroupService) Get(ctx context.Context, id string) (*model.Group, error) {
	return s.repo.GetGroup(ctx, id)
}

func (s *GroupService) List(ctx context.Context, userID string) ([]*model.Group, error) {
	return s.repo.ListGroups(ctx, userID)
}

func (s *GroupService) Update(ctx context.Context, g *model.Group, userID string) error {
	if g.CreatedBy != userID {
		return model.ErrNotAuthorized
	}
	return s.repo.UpdateGroup(ctx, g)
}

func (s *GroupService) Delete(ctx context.Context, id, userID string) error {
	g, err := s.repo.GetGroup(ctx, id)
	if err != nil {
		return model.ErrNotFound
	}
	if g.CreatedBy != userID {
		return model.ErrNotAuthorized
	}
	return s.repo.DeleteGroup(ctx, id)
}

func (s *GroupService) Members(ctx context.Context, groupID string) ([]repository.MemberInfo, error) {
	return s.repo.ListMembers(ctx, groupID)
}

func (s *GroupService) AddMember(ctx context.Context, groupID, name string, phone, email *string, actorID string) error {
	if name == "" {
		return model.ErrInvalidInput
	}

	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return model.ErrNotFound
	}
	if g.CreatedBy != actorID {
		return model.ErrNotAuthorized
	}

	return s.repo.AddMember(ctx, groupID, name, phone, email, "member")
}

func (s *GroupService) RemoveMember(ctx context.Context, groupID, memberID, actorID string) error {
	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return model.ErrNotFound
	}
	if g.CreatedBy != actorID {
		return model.ErrNotAuthorized
	}

	_, err = s.repo.GetMember(ctx, groupID, memberID)
	if err != nil {
		return model.ErrNotFound
	}

	return s.repo.RemoveMember(ctx, groupID, memberID)
}
