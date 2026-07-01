package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"app/auth"
	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
)

const adminPageSize = 25

// HandleAdminIndex redirects the admin root to the user list (the landing page).
func HandleAdminIndex() http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return nil
	})
}

// HandleAdminUsers renders the searchable, paginated user list.
func HandleAdminUsers(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		page := 1
		if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
			page = p
		}
		total, err := store.CountUsers(r.Context(), q)
		if err != nil {
			return err
		}
		numPages := (total + adminPageSize - 1) / adminPageSize
		if numPages < 1 {
			numPages = 1
		}
		if page > numPages {
			page = numPages
		}
		users, err := store.ListUsers(r.Context(), q, adminPageSize, (page-1)*adminPageSize)
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Admin · Users")
		return render(w, http.StatusOK, views.AdminUsers(Conf.PageDataFor(account(r)), meta, q, users, page, numPages))
	})
}

// HandleAdminUserDetail renders one user's detail page.
func HandleAdminUserDetail(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		u, err := store.UserByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		ents, err := store.ListEntitlements(r.Context(), id)
		if err != nil {
			return err
		}
		sessions, err := store.ListUserSessions(r.Context(), id)
		if err != nil {
			return err
		}
		sess, _ := auth.SessionFrom(r.Context())
		v := views.AdminUserView{
			User:         u,
			Entitlements: ents,
			Sessions:     sessions,
			Products:     Conf.Products,
			CSRF:         auth.CSRFToken(Conf.AppSecret, sess.ID),
			Flash:        r.URL.Query().Get("flash"),
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Admin · User")
		return render(w, http.StatusOK, views.AdminUserDetail(Conf.PageDataFor(account(r)), meta, v))
	})
}

// HandleAdminGrantEntitlement grants a catalog product to a user.
func HandleAdminGrantEntitlement(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		if _, err := store.UserByID(r.Context(), id); err != nil {
			http.NotFound(w, r)
			return nil
		}
		key := r.FormValue("product_key")
		if _, ok := Conf.ProductByKey(key); !ok {
			http.Redirect(w, r, adminUserURL(id, "Unknown product"), http.StatusSeeOther)
			return nil
		}
		if err := store.GrantEntitlement(r.Context(), id, key); err != nil {
			return err
		}
		logAdminAction(r, "grant_entitlement", id, key)
		http.Redirect(w, r, adminUserURL(id, "Granted "+key), http.StatusSeeOther)
		return nil
	})
}

// HandleAdminRevokeEntitlement removes an entitlement from a user.
func HandleAdminRevokeEntitlement(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		key := r.FormValue("product_key")
		if err := store.RevokeEntitlement(r.Context(), id, key); err != nil {
			return err
		}
		logAdminAction(r, "revoke_entitlement", id, key)
		http.Redirect(w, r, adminUserURL(id, "Revoked "+key), http.StatusSeeOther)
		return nil
	})
}

// HandleAdminRevokeSessions signs a user out of all sessions.
func HandleAdminRevokeSessions(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		if err := store.DeleteUserSessions(r.Context(), id, ""); err != nil {
			return err
		}
		logAdminAction(r, "revoke_sessions", id, "")
		http.Redirect(w, r, adminUserURL(id, "Sessions revoked"), http.StatusSeeOther)
		return nil
	})
}

// adminUserURL builds the detail URL with a flash message.
func adminUserURL(id int64, flash string) string {
	return "/admin/users/" + strconv.FormatInt(id, 10) + "?flash=" + url.QueryEscape(flash)
}

// logAdminAction records an admin mutation (actor + target) to the app logger.
func logAdminAction(r *http.Request, action string, targetID int64, detail string) {
	actor, _ := auth.UserFrom(r.Context())
	rio.LogInfo("admin action",
		slog.String("action", action),
		slog.String("actor", actor.Email),
		slog.Int64("target_user", targetID),
		slog.String("detail", detail),
	)
}
