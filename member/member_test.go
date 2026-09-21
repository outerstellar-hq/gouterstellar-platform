package member

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct{ operation string }

func (f *fakeStore) ApproveMember(context.Context, string) (bool, error) {
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

func TestNormalizeProfile(t *testing.T) {
	profile, err := NormalizeProfile(Profile{DisplayName: "  Alex  ", Bio: "  Hello  ", Public: true})
	if err != nil || profile.DisplayName != "Alex" || profile.Bio != "Hello" {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	if _, err := NormalizeProfile(Profile{Public: true}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("public profile without name: %v", err)
	}
}
