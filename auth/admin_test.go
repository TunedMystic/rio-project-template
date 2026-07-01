package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/database"
)

func TestIsAdmin(t *testing.T) {
	admins := []string{"root@example.com", "ops@example.com"}
	cases := []struct {
		email string
		want  bool
	}{
		{"root@example.com", true},
		{"  ROOT@Example.com ", true}, // normalized
		{"nope@example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsAdmin(c.email, admins); got != c.want {
			t.Errorf("IsAdmin(%q) = %v, want %v", c.email, got, c.want)
		}
	}
	if IsAdmin("root@example.com", nil) {
		t.Error("empty allowlist must deny")
	}
}

// withUser returns a request whose context carries u, as LoadUser would.
func withUser(u database.User) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
}

func TestRequireAdmin(t *testing.T) {
	admins := []string{"root@example.com"}
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Run("admin passes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		RequireAdmin(admins)(probe).ServeHTTP(rec, withUser(database.User{Email: "root@example.com"}))
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("non-admin gets 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		RequireAdmin(admins)(probe).ServeHTTP(rec, withUser(database.User{Email: "user@example.com"}))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("logged-out redirects to login", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/admin/users", nil) // no user in context
		RequireAdmin(admins)(probe).ServeHTTP(rec, r)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want 303", rec.Code)
		}
	})
}
