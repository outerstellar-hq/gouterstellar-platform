package security

import "github.com/rygel/gouterstellar-platform/internal/model"

type PermissionResolver interface {
	PermissionsFor(user *model.User) []model.Permission
	Allowed(user *model.User, perm model.Permission) bool
}

type roleBasedResolver struct{}

func NewRoleBasedPermissionResolver() PermissionResolver {
	return &roleBasedResolver{}
}

func (r *roleBasedResolver) PermissionsFor(user *model.User) []model.Permission {
	if user.Role == model.RoleAdmin {
		return []model.Permission{
			{Domain: "*", Action: "*", Instance: "*"},
		}
	}
	return []model.Permission{
		{Domain: "message", Action: "*", Instance: "*"},
		{Domain: "contact", Action: "*", Instance: "*"},
		{Domain: "sync", Action: "*", Instance: "*"},
		{Domain: "notification", Action: "read", Instance: "*"},
		{Domain: "profile", Action: "*", Instance: "*"},
	}
}

// Allowed reports whether the user holds a permission that implies perm.
func (r *roleBasedResolver) Allowed(user *model.User, perm model.Permission) bool {
	if user == nil {
		return false
	}
	for _, granted := range r.PermissionsFor(user) {
		if granted.Implies(perm) {
			return true
		}
	}
	return false
}
