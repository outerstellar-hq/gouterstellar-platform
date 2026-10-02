package faq

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type testStore struct {
	entry   Entry
	query   string
	changed bool
}

func (s *testStore) ListFAQPublished(_ context.Context, query string) ([]Entry, error) {
	s.query = query
	return []Entry{s.entry}, nil
}
func (s *testStore) ListFAQAll(context.Context) ([]Entry, error)        { return []Entry{s.entry}, nil }
func (s *testStore) FAQByID(_ context.Context, _ string) (Entry, error) { return s.entry, nil }
func (s *testStore) CreateFAQ(_ context.Context, entry Entry) (string, error) {
	s.entry = entry
	return "new-id", nil
}
func (s *testStore) UpdateFAQ(_ context.Context, entry Entry) (bool, error) {
	s.entry = entry
	return s.changed, nil
}
func (s *testStore) DeleteFAQ(context.Context, string) (bool, error) { return s.changed, nil }

func TestSaveNormalizesAndBounds(t *testing.T) {
	store := &testStore{changed: true}
	service, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	id, err := service.Save(context.Background(), Entry{Question: "  How?  ", Answer: "  Practice.  ", Published: true})
	if err != nil || id != "new-id" || store.entry.Question != "How?" || store.entry.Answer != "Practice." {
		t.Fatalf("save: id=%q entry=%+v err=%v", id, store.entry, err)
	}
	store.entry = Entry{}
	if _, err := service.Save(context.Background(), Entry{Question: strings.Repeat("q", 201), Answer: "a"}); !errors.Is(err, ErrInvalid) || store.entry.Question != "" {
		t.Fatalf("invalid save: %v", err)
	}
	id, err = service.Save(context.Background(), Entry{ID: "existing", Question: "Q", Answer: "A"})
	if err != nil || id != "existing" {
		t.Fatalf("update: id=%q err=%v", id, err)
	}
	store.changed = false
	if _, err := service.Save(context.Background(), Entry{ID: "missing", Question: "Q", Answer: "A"}); !errors.Is(err, ErrMissing) {
		t.Fatalf("missing update: %v", err)
	}
}

func TestSearchAndDelete(t *testing.T) {
	store := &testStore{changed: true}
	service, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Search(context.Background(), "  learning  "); err != nil || store.query != "learning" {
		t.Fatalf("search: query=%q err=%v", store.query, err)
	}
	if _, err := service.Search(context.Background(), strings.Repeat("x", 101)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("long search: %v", err)
	}
	if err := service.Delete(context.Background(), "id"); err != nil {
		t.Fatal(err)
	}
	store.changed = false
	if err := service.Delete(context.Background(), "id"); !errors.Is(err, ErrMissing) {
		t.Fatalf("missing delete: %v", err)
	}
}
