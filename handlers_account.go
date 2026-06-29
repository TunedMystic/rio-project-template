// handlers_account.go
package main

import (
	"net/http"
	"strings"

	"app/auth"
	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
)

// requireCSRF verifies the _csrf form field against the current session. It
// writes 403 and returns false on mismatch.
func requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := auth.SessionFrom(r.Context())
	if !ok || !auth.ValidCSRF(Conf.AppSecret, sess.ID, r.FormValue("_csrf")) {
		w.WriteHeader(http.StatusForbidden)
		return false
	}
	return true
}

func accountView(r *http.Request, active string) views.AccountView {
	sess, _ := auth.SessionFrom(r.Context())
	return views.AccountView{
		Active: active,
		CSRF:   auth.CSRFToken(Conf.AppSecret, sess.ID),
		Flash:  r.URL.Query().Get("flash"),
		Error:  r.URL.Query().Get("err"),
	}
}

func HandleAccount(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		if r.Method == http.MethodPost {
			if !requireCSRF(w, r) {
				return nil
			}
			name := strings.TrimSpace(r.FormValue("name"))
			if err := store.UpdateUserName(r.Context(), user.ID, name); err != nil {
				return err
			}
			http.Redirect(w, r, "/account", http.StatusSeeOther)
			return nil
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Profile")
		av := accountView(r, "profile")
		return render(w, http.StatusOK, views.Profile(Conf.PageDataFor(account(r)), meta, av, user.Name, user.Email))
	}
	return rio.MakeHandler(fn)
}

func HandleSecurity(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		sess, _ := auth.SessionFrom(r.Context())
		sessions, err := store.ListUserSessions(r.Context(), user.ID)
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Security")
		av := accountView(r, "security")
		return render(w, http.StatusOK, views.Security(Conf.PageDataFor(account(r)), meta, av, sessions, sess.ID, user.GoogleID != ""))
	}
	return rio.MakeHandler(fn)
}

func HandleRevokeSession(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		id := r.FormValue("id")
		// Only allow revoking the caller's own sessions.
		if sess, err := store.SessionByID(r.Context(), id); err == nil && sess.UserID == user.ID {
			_ = store.DeleteSession(r.Context(), id)
		}
		http.Redirect(w, r, "/account/security", http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleRevokeAllSessions(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		sess, _ := auth.SessionFrom(r.Context())
		if err := store.DeleteUserSessions(r.Context(), user.ID, sess.ID); err != nil {
			return err
		}
		http.Redirect(w, r, "/account/security", http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleBilling() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Billing")
		return render(w, http.StatusOK, views.Billing(Conf.PageDataFor(account(r)), meta, accountView(r, "billing")))
	}
	return rio.MakeHandler(fn)
}

func HandleDeleteAccount(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		if r.Method == http.MethodPost {
			if !requireCSRF(w, r) {
				return nil
			}
			if !strings.EqualFold(strings.TrimSpace(r.FormValue("confirm_email")), user.Email) {
				http.Redirect(w, r, "/account/delete", http.StatusSeeOther)
				return nil
			}
			if err := store.DeleteUser(r.Context(), user.ID); err != nil {
				return err
			}
			auth.ClearSessionCookie(w, !Conf.Debug)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return nil
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Delete account")
		return render(w, http.StatusOK, views.Danger(Conf.PageDataFor(account(r)), meta, accountView(r, "danger"), user.Email))
	}
	return rio.MakeHandler(fn)
}
