package service

import (
	"context"

	ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type GroupRepository interface {
	CreateGroup(ctx context.Context, g *model.Group) error
	GetGroup(ctx context.Context, id string) (*model.Group, error)
	ListGroups(ctx context.Context, userID string) ([]*model.Group, error)
	GetGroupsByIDs(ctx context.Context, ids []string) ([]*model.Group, error)
	UpdateGroup(ctx context.Context, g *model.Group) error
	DeleteGroup(ctx context.Context, id string) error
	AddMember(ctx context.Context, groupID, name string, phone, email *string, role string) error
	RemoveMember(ctx context.Context, groupID, memberID string) error
	ListMembers(ctx context.Context, groupID string) ([]repository.MemberInfo, error)
	GetMember(ctx context.Context, groupID, memberID string) (*model.GroupPlayer, error)
	MemberCount(ctx context.Context, groupID string) (int, error)
	UpdateMemberRole(ctx context.Context, groupID, memberID, role string) error
}

// AuthUserRepository is the subset of ezauth's user repository GroupService
// needs to look up and mutate a player's admin_groups AppMetadata.
type AuthUserRepository interface {
	UserGetByID(ctx context.Context, id string) (*ezauthmodels.User, error)
	UserGetByEmail(ctx context.Context, email string) (*ezauthmodels.User, error)
	UserUpdate(ctx context.Context, user *ezauthmodels.User) (*ezauthmodels.User, error)
}

type GroupService struct {
	repo     GroupRepository
	authRepo AuthUserRepository
}

func NewGroupService(repo GroupRepository, authRepo AuthUserRepository) *GroupService {
	return &GroupService{repo: repo, authRepo: authRepo}
}

// CanEdit reports whether userID may edit the group: either as the Owner
// (g.CreatedBy) or as a member promoted to admin via PromoteMember, tracked
// on their ezauth account's AppMetadata rather than on the roster row.
func (s *GroupService) CanEdit(ctx context.Context, g *model.Group, userID string) bool {
	if g.CreatedBy == userID {
		return true
	}
	user, err := s.authRepo.UserGetByID(ctx, userID)
	if err != nil {
		return false
	}
	return isGroupAdmin(user, g.ID)
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

// List returns the groups userID owns, plus any groups they've been
// promoted to admin on (so a promoted admin sees the group in their own
// "My Groups"/dashboard list, not just the Owner's).
func (s *GroupService) List(ctx context.Context, userID string) ([]*model.Group, error) {
	owned, err := s.repo.ListGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	user, err := s.authRepo.UserGetByID(ctx, userID)
	if err != nil {
		return owned, nil
	}
	adminIDs := adminGroupIDs(user)
	if len(adminIDs) == 0 {
		return owned, nil
	}

	adminGroups, err := s.repo.GetGroupsByIDs(ctx, adminIDs)
	if err != nil {
		return owned, nil
	}

	seen := make(map[string]bool, len(owned))
	result := make([]*model.Group, 0, len(owned)+len(adminGroups))
	for _, g := range owned {
		seen[g.ID] = true
		result = append(result, g)
	}
	for _, g := range adminGroups {
		if !seen[g.ID] {
			seen[g.ID] = true
			result = append(result, g)
		}
	}
	return result, nil
}

func (s *GroupService) Update(ctx context.Context, g *model.Group, userID string) error {
	if !s.CanEdit(ctx, g, userID) {
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
	if !s.CanEdit(ctx, g, actorID) {
		return model.ErrNotAuthorized
	}

	return s.repo.AddMember(ctx, groupID, name, phone, email, "member")
}

// AddMembers adds several roster members at once (comma-separated names from
// the UI). phone/email don't apply since they can't be shared across people.
func (s *GroupService) AddMembers(ctx context.Context, groupID string, names []string, actorID string) ([]string, error) {
	if len(names) == 0 {
		return nil, model.ErrInvalidInput
	}

	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return nil, model.ErrNotFound
	}
	if !s.CanEdit(ctx, g, actorID) {
		return nil, model.ErrNotAuthorized
	}

	added := make([]string, 0, len(names))
	for _, name := range names {
		if err := s.repo.AddMember(ctx, groupID, name, nil, nil, "member"); err != nil {
			return added, err
		}
		added = append(added, name)
	}
	return added, nil
}

func (s *GroupService) RemoveMember(ctx context.Context, groupID, memberID, actorID string) error {
	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return model.ErrNotFound
	}
	if !s.CanEdit(ctx, g, actorID) {
		return model.ErrNotAuthorized
	}

	_, err = s.repo.GetMember(ctx, groupID, memberID)
	if err != nil {
		return model.ErrNotFound
	}

	return s.repo.RemoveMember(ctx, groupID, memberID)
}

// PromoteMember grants a roster member admin rights on the group by linking
// their roster email to an ezauth account and recording the group ID on that
// account's AppMetadata. Owner-only: promoting/demoting admins is not itself
// delegable to other admins.
func (s *GroupService) PromoteMember(ctx context.Context, groupID, memberID, actorID string) error {
	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return model.ErrNotFound
	}
	if g.CreatedBy != actorID {
		return model.ErrNotAuthorized
	}

	member, err := s.repo.GetMember(ctx, groupID, memberID)
	if err != nil {
		return model.ErrNotFound
	}
	if member.Email == nil || *member.Email == "" {
		return model.ErrMemberNeedsEmail
	}

	user, err := s.authRepo.UserGetByEmail(ctx, *member.Email)
	if err != nil {
		return model.ErrNoLinkedAccount
	}

	addGroupAdmin(user, groupID)
	if _, err := s.authRepo.UserUpdate(ctx, user); err != nil {
		return err
	}

	return s.repo.UpdateMemberRole(ctx, groupID, memberID, "admin")
}

// DemoteMember reverses PromoteMember. Owner-only.
func (s *GroupService) DemoteMember(ctx context.Context, groupID, memberID, actorID string) error {
	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return model.ErrNotFound
	}
	if g.CreatedBy != actorID {
		return model.ErrNotAuthorized
	}

	member, err := s.repo.GetMember(ctx, groupID, memberID)
	if err != nil {
		return model.ErrNotFound
	}

	if member.Email != nil && *member.Email != "" {
		if user, err := s.authRepo.UserGetByEmail(ctx, *member.Email); err == nil {
			removeGroupAdmin(user, groupID)
			_, _ = s.authRepo.UserUpdate(ctx, user)
		}
	}

	return s.repo.UpdateMemberRole(ctx, groupID, memberID, "member")
}
