package web

import "net/http"

const SessionCookieName = "oss_session"

func CreateSessionCookie(token string, secure bool) *http.Cookie {
	return &http.Cookie{ // #nosec G124 -- Secure/HttpOnly/SameSite all set; Secure is parameterized per-environment
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   0,
	}
}

func ClearSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{ // #nosec G124 -- Secure/HttpOnly/SameSite all set; Secure is parameterized per-environment
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

func GetSessionToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
