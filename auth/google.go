// auth/google.go
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	stateCookieName = "g_oauth"
	stateCookieTTL  = 10 * time.Minute
	googleUserinfo  = "https://openidconnect.googleapis.com/v1/userinfo"
)

// googleEndpoint is Google's OAuth2 endpoint, declared inline so we depend only
// on golang.org/x/oauth2 (importing .../oauth2/google would pull in
// cloud.google.com/go/compute/metadata).
var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// GoogleUser is the subset of Google's userinfo response we consume.
type GoogleUser struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// OAuthState is the data round-tripped through the signed state cookie.
type OAuthState struct {
	State    string `json:"s"`
	Next     string `json:"n"`
	Mode     string `json:"m"` // "login" | "link"
	Verifier string `json:"v"` // PKCE code verifier
}

// GoogleOAuth wraps the OAuth2 config and the userinfo endpoint (overridable in tests).
type GoogleOAuth struct {
	cfg         *oauth2.Config
	UserinfoURL string
}

// NewGoogleOAuth builds the Google OAuth2 client.
func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     googleEndpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
		UserinfoURL: googleUserinfo,
	}
}

// SetEndpoint overrides the OAuth2 endpoint (used in tests to point at a fake server).
func (g *GoogleOAuth) SetEndpoint(authURL, tokenURL string) {
	g.cfg.Endpoint = oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
}

// NewVerifier returns a fresh PKCE code verifier.
func NewVerifier() string { return oauth2.GenerateVerifier() }

// AuthCodeURL returns the consent URL carrying state and a PKCE S256 challenge.
func (g *GoogleOAuth) AuthCodeURL(state, verifier string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.S256ChallengeOption(verifier))
}

// Exchange swaps an authorization code (+ PKCE verifier) for a token.
func (g *GoogleOAuth) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	return g.cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
}

// FetchUser calls the userinfo endpoint with the token and decodes the profile.
func (g *GoogleOAuth) FetchUser(ctx context.Context, token *oauth2.Token) (GoogleUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.UserinfoURL, nil)
	if err != nil {
		return GoogleUser{}, err
	}
	resp, err := g.cfg.Client(ctx, token).Do(req)
	if err != nil {
		return GoogleUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GoogleUser{}, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var gu GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		return GoogleUser{}, err
	}
	return gu, nil
}

// SetStateCookie stores a signed OAuthState in a short-lived cookie. The value
// is base64(json) + "." + HMAC(secret, base64) using the existing CSRF primitive.
func SetStateCookie(w http.ResponseWriter, secret string, st OAuthState, secure bool) {
	payload, _ := json.Marshal(st)
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    b64 + "." + CSRFToken(secret, b64),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(stateCookieTTL),
		MaxAge:   int(stateCookieTTL.Seconds()),
	})
}

// ReadStateCookie verifies the signature and decodes the OAuthState.
func ReadStateCookie(r *http.Request, secret string) (OAuthState, bool) {
	c, err := r.Cookie(stateCookieName)
	if err != nil {
		return OAuthState{}, false
	}
	b64, mac, found := strings.Cut(c.Value, ".")
	if !found || !ValidCSRF(secret, b64, mac) {
		return OAuthState{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return OAuthState{}, false
	}
	var st OAuthState
	if err := json.Unmarshal(payload, &st); err != nil {
		return OAuthState{}, false
	}
	return st, true
}

// ClearStateCookie removes the state cookie.
func ClearStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
