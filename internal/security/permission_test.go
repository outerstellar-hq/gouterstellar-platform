package security

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/rygel/gouterstellar-platform/internal/model"
)

func TestRoleBasedPermissionResolver(t *testing.T) {
	resolver := NewRoleBasedPermissionResolver()

	t.Run("admin gets superuser permission", func(t *testing.T) {
		admin := &model.User{ID: uuid.New(), Role: model.RoleAdmin}
		perms := resolver.PermissionsFor(admin)
		assert.Len(t, perms, 1)
		assert.True(t, perms[0].Implies(model.Permission{Domain: "any", Action: "any", Instance: "any"}))
		assert.Equal(t, "*", perms[0].Domain)
		assert.Equal(t, "*", perms[0].Action)
	})

	t.Run("user gets scoped permissions", func(t *testing.T) {
		user := &model.User{ID: uuid.New(), Role: model.RoleUser}
		perms := resolver.PermissionsFor(user)
		assert.Len(t, perms, 5)

		assert.True(t, perms[0].Implies(model.Permission{Domain: "message", Action: "read"}))
		assert.True(t, perms[1].Implies(model.Permission{Domain: "contact", Action: "write"}))
		assert.True(t, perms[2].Implies(model.Permission{Domain: "sync", Action: "read"}))
		assert.True(t, perms[3].Implies(model.Permission{Domain: "notification", Action: "read"}))
		assert.False(t, perms[3].Implies(model.Permission{Domain: "notification", Action: "write"}))
		assert.True(t, perms[4].Implies(model.Permission{Domain: "profile", Action: "update"}))
	})
}
