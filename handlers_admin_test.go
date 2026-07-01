package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"app/auth"
	"app/database"
)

func newHandlerTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/s.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return database.NewStore(db)
}

func TestAdminUsers_AdminSees200_NonAdmin404(t *testing.T) {
	store := newHandlerTestStore(t)
	sess, u := loggedInRequestSession(t, store)

	// Admin allowlist contains the user → 200.
	r, _ := loggedInWith(t, store, u, sess, http.MethodGet, "/admin/users", "")
	rec := httptest.NewRecorder()
	auth.RequireUser(auth.RequireAdmin([]string{u.Email})(HandleAdminUsers(store))).ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("admin status = %d, want 200", rec.Code)
	}

	// Empty allowlist → non-admin → 404.
	r2, _ := loggedInWith(t, store, u, sess, http.MethodGet, "/admin/users", "")
	rec2 := httptest.NewRecorder()
	auth.RequireUser(auth.RequireAdmin(nil)(HandleAdminUsers(store))).ServeHTTP(rec2, r2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("non-admin status = %d, want 404", rec2.Code)
	}
}

func TestAdminGrantEntitlement_AddsAndRedirects(t *testing.T) {
	store := newHandlerTestStore(t)
	sess, u := loggedInRequestSession(t, store)

	id := strconv.FormatInt(u.ID, 10)
	form := url.Values{}
	form.Set("_csrf", auth.CSRFToken(Conf.AppSecret, sess.ID))
	form.Set("product_key", "ebook")
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/admin/users/"+id+"/entitlements/grant", form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", id) // handler reads r.PathValue("id"); set it explicitly since we call the handler directly, not via the mux

	rec := httptest.NewRecorder()
	HandleAdminGrantEntitlement(store).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	has, err := store.HasEntitlement(r.Context(), u.ID, "ebook")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("entitlement was not granted")
	}
}
