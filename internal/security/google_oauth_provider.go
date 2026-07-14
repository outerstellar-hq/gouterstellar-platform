package security

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GoogleOAuthProvider implements OAuthProvider against Google's OAuth 2.0 /
// OpenID Connect endpoints. It uses only the standard library net/http client
// (no external SDK) for the authorization-code exchange and userinfo lookup.
type GoogleOAuthProvider struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

// NewGoogleOAuthProvider constructs a provider. redirectURI is the application
// callback URL; it is also overridden per-request from the handler so multiple
// deployments can share one provider instance.
func NewGoogleOAuthProvider(clientID, clientSecret, redirectURI string) *GoogleOAuthProvider {
	return &GoogleOAuthProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *GoogleOAuthProvider) Name() string { return "google" }

// AuthorizationURL builds the Google consent-screen URL the browser is
// redirected to. The requested scopes cover OpenID plus email and profile so
// the returned userinfo includes subject, email, and display name.
func (p *GoogleOAuthProvider) AuthorizationURL(state, redirectURI string) string {
	if redirectURI == "" {
		redirectURI = p.redirectURI
	}
	q := url.Values{}
	q.Set("client_id", p.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	return googleAuthURL + "?" + q.Encode()
}

// googleTokenResponse models the JSON returned by the token endpoint.
type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// googleUserInfoResponse models the subset of userinfo fields we rely on.
type googleUserInfoResponse struct {
	Sub   string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// ExchangeCode swaps an authorization code for tokens and fetches the Google
// userinfo profile, returning the normalized OAuthUserInfo.
func (p *GoogleOAuthProvider) ExchangeCode(code, state, redirectURI string) (*OAuthUserInfo, error) {
	if code == "" {
		return nil, &OAuthException{Provider: "google", Message: "missing authorization code"}
	}
	if redirectURI == "" {
		redirectURI = p.redirectURI
	}

	token, err := p.exchangeCodeForToken(code, redirectURI)
	if err != nil {
		return nil, err
	}

	userInfo, err := p.fetchUserInfo(token.AccessToken)
	if err != nil {
		return nil, err
	}

	result := &OAuthUserInfo{Subject: userInfo.Sub}
	if userInfo.Email != "" {
		email := userInfo.Email
		result.Email = &email
	}
	if userInfo.Name != "" {
		name := userInfo.Name
		result.DisplayName = &name
	}
	return result, nil
}

func (p *GoogleOAuthProvider) exchangeCodeForToken(code, redirectURI string) (*googleTokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	resp, err := p.httpClient.PostForm(googleTokenURL, form)
	if err != nil {
		return nil, &OAuthException{Provider: "google", Message: "token request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &OAuthException{Provider: "google", Message: "read token response: " + err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &OAuthException{Provider: "google", Message: fmt.Sprintf("token endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, &OAuthException{Provider: "google", Message: "parse token response: " + err.Error()}
	}
	if tokenResp.AccessToken == "" {
		return nil, &OAuthException{Provider: "google", Message: "token response missing access_token"}
	}
	return &tokenResp, nil
}

func (p *GoogleOAuthProvider) fetchUserInfo(accessToken string) (*googleUserInfoResponse, error) {
	req, err := http.NewRequest(http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, &OAuthException{Provider: "google", Message: "build userinfo request: " + err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &OAuthException{Provider: "google", Message: "userinfo request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &OAuthException{Provider: "google", Message: "read userinfo response: " + err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &OAuthException{Provider: "google", Message: fmt.Sprintf("userinfo endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	var info googleUserInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, &OAuthException{Provider: "google", Message: "parse userinfo response: " + err.Error()}
	}
	if info.Sub == "" {
		return nil, &OAuthException{Provider: "google", Message: "userinfo response missing subject id"}
	}
	return &info, nil
}

var _ OAuthProvider = (*GoogleOAuthProvider)(nil)
