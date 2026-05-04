package security

import "github.com/rygel/gouterstellar-platform/internal/model"

type PermissionResolver interface {
	PermissionsFor(user *model.User) []model.Permission
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
