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
		if token := CSRFToken(r); token == "" {
			t.Fatal("missing token")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "https://example.test/form", nil))
	cookies := getResponse.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookies = %#v", cookies)
	}

	post := httptest.NewRequest(http.MethodPost, "https://example.test/form", strings.NewReader(url.Values{}.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
