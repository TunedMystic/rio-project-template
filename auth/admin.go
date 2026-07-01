package auth

import (
	"net/http"
	"net/url"
	"strings"
)

// IsAdmin reports whether email is in the admins allowlist (case- and
// whitespace-insensitive). An empty allowlist always denies.
func IsAdmin(email string, admins []string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return false
	}
	for _, a := range admins {
		if strings.ToLower(strings.TrimSpace(a)) == e {
			return true
		}
	}
	return false
}

// RequireAdmin gates a handler to allowlisted admins. Logged-out users are
// redirected to /login; authenticated non-admins get a 404 so the admin surface
// is not advertised. Intended to wrap a handler already behind RequireUser, but
// is safe on its own.
func RequireAdmin(admins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFrom(r.Context())
			if !ok {
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
				return
			}
			if !IsAdmin(u.Email, admins) {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
