package membership

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	operation        string
	groupName        string
	groupDescription string
}

func (f *fakeStore) ApproveMember(_ context.Context, _ string) (bool, error) {
	f.operation = "approve"
	return true, nil
}
func (f *fakeStore) SetMemberDisabled(_ context.Context, _ string, disabled bool) (bool, error) {
	if disabled {
		f.operation = "disable"
	} else {
		f.operation = "enable"
	}
	return true, nil
}
func (f *fakeStore) SetMemberRole(_ context.Context, _, role string) (bool, error) {
	f.operation = "role:" + role
	return true, nil
}
func (f *fakeStore) CreateGroup(_ context.Context, _, name, description string) (string, error) {
	f.operation, f.groupName, f.groupDescription = "create", name, description
	return "group-id", nil
}
func (f *fakeStore) JoinGroup(_ context.Context, _, _ string) error { f.operation = "join"; return nil }
func (f *fakeStore) LeaveGroup(_ context.Context, _, _ string) error {
	f.operation = "leave"
	return nil
}

func TestAccountChanges(t *testing.T) {
	store := &fakeStore{}
	service, err := New(store, "member", "admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		action     AccountAction
		role, want string
	}{
		{Approve, "", "approve"}, {Disable, "", "disable"}, {Enable, "", "enable"}, {SetRole, "admin", "role:admin"},
	} {
		changed, err := service.ChangeAccount(context.Background(), "actor", "target", test.action, test.role)
		if err != nil || !changed || store.operation != test.want {
			t.Fatalf("%s: changed=%t operation=%q err=%v", test.action, changed, store.operation, err)
		}
	}
	store.operation = ""
	if _, err := service.ChangeAccount(context.Background(), "same", "same", Disable, ""); !errors.Is(err, ErrSelfChange) || store.operation != "" {
		t.Fatalf("self change: %v, %q", err, store.operation)
	}
	if _, err := service.ChangeAccount(context.Background(), "actor", "target", SetRole, "owner"); !errors.Is(err, ErrInvalidInput) || store.operation != "" {
		t.Fatalf("invalid role: %v, %q", err, store.operation)
	}
}

func TestGroupPolicy(t *testing.T) {
	store := &fakeStore{}
	service, err := New(store, "member")
	if err != nil {
		t.Fatal(err)
	}
	id, err := service.CreateGroup(context.Background(), "actor", "  Learners  ", "  Practice together  ")
	if err != nil || id != "group-id" || store.groupName != "Learners" || store.groupDescription != "Practice together" {
		t.Fatalf("create: id=%q name=%q description=%q err=%v", id, store.groupName, store.groupDescription, err)
	}
	store.operation = ""
	if _, err := service.CreateGroup(context.Background(), "actor", strings.Repeat("x", 101), "description"); !errors.Is(err, ErrInvalidInput) || store.operation != "" {
		t.Fatalf("invalid group: %v, %q", err, store.operation)
	}
	if err := service.JoinGroup(context.Background(), "group", "member"); err != nil || store.operation != "join" {
		t.Fatalf("join: %v, %q", err, store.operation)
	}
	if err := service.LeaveGroup(context.Background(), "group", "member"); err != nil || store.operation != "leave" {
		t.Fatalf("leave: %v, %q", err, store.operation)
	}
	store.operation = ""
	if err := service.JoinGroup(context.Background(), "", "member"); !errors.Is(err, ErrInvalidInput) || store.operation != "" {
		t.Fatalf("invalid join: %v, %q", err, store.operation)
	}
}
