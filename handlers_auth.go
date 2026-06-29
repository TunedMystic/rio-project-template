// handlers_auth.go
package main

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"app/auth"
	"app/config"
	"app/database"
	"app/email"
	"app/views"

	"github.com/tunedmystic/rio"
	"github.com/tunedmystic/rio/forms"
)

const loginTokenTTL = 15 * time.Minute

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	return host
}

func HandleLogin(store *database.Store, sender email.Sender, limiter *auth.Limiter) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		next := auth.SafeNext(r.URL.Query().Get("next"))
		if r.Method != http.MethodPost {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Log in")
			return render(w, http.StatusOK, views.Login(Conf.PageDataFor(account(r)), meta, "", "", next))
		}

		next = auth.SafeNext(r.FormValue("next"))
		emailAddr := strings.TrimSpace(r.FormValue("email"))
		form := forms.New()
		form.CleanString("email", emailAddr, forms.StrRequired(), forms.StrEmail())
		if !form.IsValid() {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Log in")
			field := form.MustField("email")
			return render(w, http.StatusUnprocessableEntity,
				views.Login(Conf.PageDataFor(account(r)), meta, field.Value(), field.Err().Error(), next))
		}

		// Rate-limit, then (best-effort) issue + send. Always show the same page.
		if limiter.Allow(emailAddr + "|" + clientIP(r)) {
			if token, hash, err := auth.GenerateToken(); err == nil {
				if err := store.CreateToken(r.Context(), hash, emailAddr, time.Now().Add(loginTokenTTL)); err == nil {
					link := Conf.BaseURL + "/auth/verify?token=" + token
					if next != "/account" {
						link += "&next=" + url.QueryEscape(next)
					}
					body := "Click to log in (expires in 15 minutes):\n\n" + link
					if err := sender.Send(r.Context(), emailAddr, "Your login link", body); err != nil {
						rio.LogError(err)
					}
				}
			}
		}
		http.Redirect(w, r, "/login/sent?email="+url.QueryEscape(emailAddr), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleLoginSent() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Check your email")
		return render(w, http.StatusOK,
			views.LoginSent(Conf.PageDataFor(account(r)), meta, r.URL.Query().Get("email")))
	}
	return rio.MakeHandler(fn)
}

func HandleVerify(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		token := r.URL.Query().Get("token")
		tok, ok, err := store.ConsumeToken(r.Context(), auth.HashToken(token))
		if err != nil {
			return err
		}
		if !ok || tok.ExpiresAt.Before(time.Now()) {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Link expired")
			return render(w, http.StatusOK, views.VerifyError(Conf.PageDataFor(account(r)), meta))
		}

		// Find-or-create the user (unified signup).
		user, err := store.UserByEmail(r.Context(), tok.Email)
		if err != nil {
			if user, err = store.CreateUser(r.Context(), tok.Email, ""); err != nil {
				return err
			}
		}

		// Create the session.
		sessTok, sessHash, err := auth.GenerateToken()
		if err != nil {
			return err
		}
		if err := store.CreateSession(r.Context(), sessHash, user.ID,
			time.Now().Add(auth.SessionTTL), r.UserAgent(), clientIP(r)); err != nil {
			return err
		}
		auth.SetSessionCookie(w, sessTok, !Conf.Debug)

		http.Redirect(w, r, auth.SafeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleLogout(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if sess, ok := auth.SessionFrom(r.Context()); ok {
			_ = store.DeleteSession(r.Context(), sess.ID)
		}
		auth.ClearSessionCookie(w, !Conf.Debug)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

// account builds the nav account view-model from the request context.
func account(r *http.Request) config.Account {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		return config.Account{}
	}
	return config.Account{LoggedIn: true, Name: u.Name, Email: u.Email}
}
