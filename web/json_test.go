package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/outerstellar-hq/gouterstellar-platform/web"
)

func TestDecodeJSONAcceptsOneKnownObject(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("  {\"name\":\"Ada\"}\n"))
	var body struct {
		Name string `json:"name"`
	}
	if err := web.DecodeJSON(httptest.NewRecorder(), request, 64, &body); err != nil {
		t.Fatal(err)
	}
	if body.Name != "Ada" {
		t.Fatalf("name = %q, want Ada", body.Name)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada","admin":true}`))
	var body struct {
		Name string `json:"name"`
	}
	err := web.DecodeJSON(httptest.NewRecorder(), request, 64, &body)
	if err == nil || !strings.Contains(err.Error(), `unknown field "admin"`) {
		t.Fatalf("error = %v, want unknown-field error", err)
	}
}

func TestDecodeJSONRejectsAdditionalValue(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}{"name":"Grace"}`))
	var body struct {
		Name string `json:"name"`
	}
	err := web.DecodeJSON(httptest.NewRecorder(), request, 64, &body)
	if !errors.Is(err, web.ErrMultipleJSONValues) {
		t.Fatalf("error = %v, want %v", err, web.ErrMultipleJSONValues)
	}
}

func TestDecodeJSONRejectsTrailingMalformedData(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"} trailing`))
	var body struct {
		Name string `json:"name"`
	}
	err := web.DecodeJSON(httptest.NewRecorder(), request, 64, &body)
	if err == nil || !strings.Contains(err.Error(), "trailing JSON data") {
		t.Fatalf("error = %v, want trailing-data error", err)
	}
}

func TestDecodeJSONReportsBodyLimit(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}`))
	var body struct {
		Name string `json:"name"`
	}
	err := web.DecodeJSON(httptest.NewRecorder(), request, 8, &body)
	var sizeError *http.MaxBytesError
	if !errors.As(err, &sizeError) {
		t.Fatalf("error = %v, want *http.MaxBytesError", err)
	}
	if sizeError.Limit != 8 {
		t.Fatalf("limit = %d, want 8", sizeError.Limit)
	}
}

func TestDecodeJSONRequiresPositiveLimit(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	err := web.DecodeJSON(httptest.NewRecorder(), request, 0, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("error = %v, want positive-limit error", err)
	}
}
