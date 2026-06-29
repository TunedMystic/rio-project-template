// auth/middleware_test.go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"app/database"
)

func storeWithUser(t *testing.T) (*database.Store, database.User) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatal(err)
	}
	s := database.NewStore(db)
	u, _ := s.CreateUser(context.Background(), "u@example.com", "U")
	return s, u
}

func TestLoadUser_PopulatesContext(t *testing.T) {
	s, u := storeWithUser(t)
	token, hash, _ := GenerateToken()
	_ = s.CreateSession(context.Background(), hash, u.ID, time.Now().Add(time.Hour), "", "")

	var seen database.User
	var ok bool
	h := LoadUser(s)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, ok = UserFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: token})
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !ok || seen.ID != u.ID {
		t.Fatalf("user not loaded: ok=%v seen=%+v", ok, seen)
	}
}

func TestRequireUser_RedirectsAnon(t *testing.T) {
	rec := httptest.NewRecorder()
	guarded := RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not run for anon")
	}))
	guarded.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/account", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?next=%2Faccount" {
		t.Errorf("Location = %q", loc)
	}
}

var _ = context.Background
