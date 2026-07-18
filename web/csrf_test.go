package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCSRFIssuesSecureDefaultsAndRejectsMissingToken(t *testing.T) {
	middleware, err := NewCSRF(CSRFConfig{AuthKey: bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := CSRFToken(r)
		if token == "" {
			t.Fatal("missing token")
		}
		w.Header().Set("X-Test-CSRF-Token", token)
		w.WriteHeader(http.StatusNoContent)
	}))
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "https://example.test/form", nil))
	cookies := getResponse.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookies = %#v", cookies)
	}
	token := getResponse.Header().Get("X-Test-CSRF-Token")

	validForm := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader(url.Values{
		"csrf_token": {token},
	}.Encode()))
	validForm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validForm.Header.Set("Referer", "https://example.test/form")
	validForm.AddCookie(cookies[0])
	validFormResponse := httptest.NewRecorder()
	handler.ServeHTTP(validFormResponse, validForm)
	if validFormResponse.Code != http.StatusNoContent {
		t.Fatalf("valid form status = %d", validFormResponse.Code)
	}

	validJSON := httptest.NewRequest(http.MethodPost, "https://example.test/data", strings.NewReader("{}"))
	validJSON.Header.Set("Content-Type", "application/json")
	validJSON.Header.Set("Referer", "https://example.test/form")
	validJSON.Header.Set("X-CSRF-Token", token)
	validJSON.AddCookie(cookies[0])
	validJSONResponse := httptest.NewRecorder()
	handler.ServeHTTP(validJSONResponse, validJSON)
	if validJSONResponse.Code != http.StatusNoContent {
		t.Fatalf("valid JSON status = %d", validJSONResponse.Code)
	}

	post := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader(url.Values{}.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Referer", "https://example.test/form")
	post.AddCookie(cookies[0])
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusForbidden {
		t.Fatalf("status = %d", postResponse.Code)
	}
}

func TestCSRFRejectsWeakKey(t *testing.T) {
	if _, err := NewCSRF(CSRFConfig{AuthKey: []byte("weak")}); err == nil {
		t.Fatal("weak key was accepted")
	}
}
