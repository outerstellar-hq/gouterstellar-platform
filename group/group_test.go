package group

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct{ operation, name, description string }

func (f *fakeStore) CreateGroup(_ context.Context, _, name, description string) (string, error) {
	f.operation, f.name, f.description = "create", name, description
	return "group-id", nil
}
func (f *fakeStore) JoinGroup(context.Context, string, string) error {
	f.operation = "join"
	return nil
}
func (f *fakeStore) LeaveGroup(context.Context, string, string) error {
	f.operation = "leave"
	return nil
}

func TestGroupPolicy(t *testing.T) {
	store := &fakeStore{}
	service, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	id, err := service.CreateGroup(context.Background(), "actor", "  Learners  ", "  Practice together  ")
	if err != nil || id != "group-id" || store.name != "Learners" || store.description != "Practice together" {
		t.Fatalf("create: id=%q name=%q description=%q err=%v", id, store.name, store.description, err)
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
}
