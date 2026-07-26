package service

import (
	"slices"

	ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"
)

const adminGroupsMetaKey = "admin_groups"

// adminGroupIDs reads the admin_groups list from AppMetadata. It has to handle
// two shapes: []string (set earlier in the same process, before persisting)
// and []any (what a freshly-fetched user's JSONB-backed AppMetadata decodes
// into, since GetAppMeta's plain type assertion never matches []string after
// a DB round trip).
func adminGroupIDs(u *ezauthmodels.User) []string {
	val, ok := u.AppMetadata[adminGroupsMetaKey]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				ids = append(ids, s)
			}
		}
		return ids
	default:
		return nil
	}
}

func isGroupAdmin(u *ezauthmodels.User, groupID string) bool {
	return slices.Contains(adminGroupIDs(u), groupID)
}

func addGroupAdmin(u *ezauthmodels.User, groupID string) {
	ids := adminGroupIDs(u)
	if !slices.Contains(ids, groupID) {
		ids = append(ids, groupID)
	}
	u.SetAppMeta(adminGroupsMetaKey, ids)
}

func removeGroupAdmin(u *ezauthmodels.User, groupID string) {
	ids := adminGroupIDs(u)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != groupID {
			out = append(out, id)
		}
	}
	u.SetAppMeta(adminGroupsMetaKey, out)
}
