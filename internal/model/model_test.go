package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConflictStrategyFromString(t *testing.T) {
	assert.Equal(t, ConflictMine, ConflictStrategyFromString("mine"))
	assert.Equal(t, ConflictServer, ConflictStrategyFromString("server"))
	assert.Equal(t, ConflictServer, ConflictStrategyFromString("unknown"))
	assert.Equal(t, ConflictServer, ConflictStrategyFromString(""))
}

func TestPermissionParse(t *testing.T) {
	assert.Equal(t, Permission{Domain: "admin", Action: "*", Instance: "*"}, ParsePermission("admin"))
	assert.Equal(t, Permission{Domain: "message", Action: "read", Instance: "*"}, ParsePermission("message:read"))
	assert.Equal(t, Permission{Domain: "message", Action: "read", Instance: "123"}, ParsePermission("message:read:123"))
}

func TestPermissionImplies(t *testing.T) {
	super := Permission{Domain: "*", Action: "*", Instance: "*"}
	assert.True(t, super.Implies(Permission{Domain: "message", Action: "read", Instance: "123"}))

	domain := Permission{Domain: "message", Action: "*", Instance: "*"}
	assert.True(t, domain.Implies(Permission{Domain: "message", Action: "read"}))
	assert.False(t, domain.Implies(Permission{Domain: "contact", Action: "read"}))

	specific := Permission{Domain: "message", Action: "read", Instance: "123"}
	assert.False(t, specific.Implies(Permission{Domain: "message", Action: "read", Instance: "456"}))
}

func TestPermissionString(t *testing.T) {
	assert.Equal(t, "admin", Permission{Domain: "admin", Action: "*", Instance: "*"}.String())
	assert.Equal(t, "message:read", Permission{Domain: "message", Action: "read", Instance: "*"}.String())
	assert.Equal(t, "message:read:123", Permission{Domain: "message", Action: "read", Instance: "123"}.String())
}

func TestNewPaginationMetadata(t *testing.T) {
	md := NewPaginationMetadata(1, 10, 25)
	assert.Equal(t, 1, md.CurrentPage)
	assert.Equal(t, 3, md.TotalPages)
	assert.False(t, md.HasPrevious)
	assert.True(t, md.HasNext)

	md2 := NewPaginationMetadata(2, 10, 25)
	assert.True(t, md2.HasPrevious)
	assert.True(t, md2.HasNext)

	md3 := NewPaginationMetadata(3, 10, 25)
	assert.True(t, md3.HasPrevious)
	assert.False(t, md3.HasNext)
}

func TestErrorTypes(t *testing.T) {
	assert.Contains(t, (&MessageNotFoundError{SyncID: "abc"}).Error(), "abc")
	assert.Contains(t, (&ContactNotFoundError{SyncID: "def"}).Error(), "def")
	assert.Contains(t, (&UsernameAlreadyExistsError{Username: "bob"}).Error(), "bob")
	assert.Equal(t, "Session has expired", (&SessionExpiredError{}).Error())
	assert.Contains(t, (&ValidationError{Errors: []string{"a", "b"}}).Error(), "a")
}

func TestUserToSummary(t *testing.T) {
	u := User{Username: "alice", Email: "alice@test.com", Role: RoleAdmin, Enabled: true}
	s := u.ToSummary()
	assert.Equal(t, "alice", s.Username)
	assert.Equal(t, "ADMIN", s.Role)
	assert.True(t, s.Enabled)
}

func TestMessageSummaryUpdatedAtLabel(t *testing.T) {
	m := MessageSummary{UpdatedAtEpochMs: 1700000000000}
	label := m.UpdatedAtLabel()
	assert.NotEmpty(t, label)
	assert.Contains(t, label, "-")
	assert.Contains(t, label, ":")
}
