package service

import (
	"context"
	"errors"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type GroupService struct {
	repo *repository.Repository
}

func NewGroupService(repo *repository.Repository) *GroupService {
	return &GroupService{repo: repo}
}

func (s *GroupService) Create(ctx context.Context, name string, description *string, userID string) (*model.Group, error) {
	g := &model.Group{
		Name:        name,
		Description: description,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateGroup(ctx, g); err != nil {
		return nil, err
	}
	if err := s.repo.AddMember(ctx, g.ID, userID, "admin"); err != nil {
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
	member, err := s.repo.GetMember(ctx, g.ID, userID)
	if err != nil {
		return model.ErrNotFound
	}
	if member.Role != "admin" {
		return model.ErrNotAuthorized
	}
	return s.repo.UpdateGroup(ctx, g)
}

func (s *GroupService) Delete(ctx context.Context, id, userID string) error {
	member, err := s.repo.GetMember(ctx, id, userID)
	if err != nil {
		return model.ErrNotFound
	}
	if member.Role != "admin" {
		return model.ErrNotAuthorized
	}
	return s.repo.DeleteGroup(ctx, id)
}

func (s *GroupService) Members(ctx context.Context, groupID string) ([]repository.MemberInfo, error) {
	return s.repo.ListMembers(ctx, groupID)
}

func (s *GroupService) AddMember(ctx context.Context, groupID, userID, actorID string) error {
	if userID == "" {
		return model.ErrInvalidInput
	}
	member, err := s.repo.GetMember(ctx, groupID, actorID)
	if err != nil {
		return model.ErrNotFound
	}
	if member.Role != "admin" {
		return model.ErrNotAuthorized
	}
	return s.repo.AddMember(ctx, groupID, userID, "member")
}

func (s *GroupService) RemoveMember(ctx context.Context, groupID, userID, actorID string) error {
	if userID == actorID {
		return errors.New("cannot remove yourself")
	}
	member, err := s.repo.GetMember(ctx, groupID, actorID)
	if err != nil {
		return model.ErrNotFound
	}
	if member.Role != "admin" {
		return model.ErrNotAuthorized
	}
	return s.repo.RemoveMember(ctx, groupID, userID)
}
