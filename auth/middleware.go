// auth/middleware.go
package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"app/database"
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

// UserFrom returns the authenticated user from the request context.
func UserFrom(ctx context.Context) (database.User, bool) {
	u, ok := ctx.Value(userKey).(database.User)
	return u, ok
}

// SessionFrom returns the current session from the request context.
func SessionFrom(ctx context.Context) (database.Session, bool) {
	s, ok := ctx.Value(sessionKey).(database.Session)
	return s, ok
}

// LoadUser loads the current user/session into the request context when the
// session cookie resolves to a live (non-expired) session. Otherwise it passes
// through unauthenticated.
func LoadUser(store *database.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := SessionToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := store.SessionByID(r.Context(), HashToken(token))
			if err != nil || sess.ExpiresAt.Before(time.Now()) {
				next.ServeHTTP(w, r)
				return
			}
			user, err := store.UserByID(r.Context(), sess.UserID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userKey, user)
			ctx = context.WithValue(ctx, sessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireUser redirects to /login (preserving the destination) when there is no
// authenticated user.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
