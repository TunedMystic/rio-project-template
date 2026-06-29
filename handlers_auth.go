// handlers_auth.go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
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

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
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
		if limiter.Allow(emailAddr + "|" + clientIP(r, Conf.TrustProxy)) {
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
			time.Now().Add(auth.SessionTTL), r.UserAgent(), clientIP(r, Conf.TrustProxy)); err != nil {
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

// googleSignIn resolves a Google identity to a user: by google_id, else by
// verified email (linking it), else by creating the user. Unverified emails
// are rejected.
func googleSignIn(ctx context.Context, store *database.Store, gu auth.GoogleUser) (database.User, error) {
	if !gu.EmailVerified {
		return database.User{}, fmt.Errorf("google email not verified")
	}
	if u, err := store.UserByGoogleID(ctx, gu.Sub); err == nil {
		return u, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return database.User{}, err
	}

	u, err := store.UserByEmail(ctx, gu.Email)
	if err == nil {
		if err := store.SetUserGoogleID(ctx, u.ID, gu.Sub); err != nil {
			return database.User{}, err
		}
		if u.Name == "" && gu.Name != "" {
			_ = store.UpdateUserName(ctx, u.ID, gu.Name)
		}
		return store.UserByID(ctx, u.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return database.User{}, err
	}

	u, err = store.CreateUser(ctx, gu.Email, gu.Name)
	if err != nil {
		return database.User{}, err
	}
	if err := store.SetUserGoogleID(ctx, u.ID, gu.Sub); err != nil {
		return database.User{}, err
	}
	u.GoogleID = gu.Sub
	return u, nil
}

// googleLink attaches a Google identity to the current user, rejecting a
// google_id already linked to a different account.
func googleLink(ctx context.Context, store *database.Store, current database.User, gu auth.GoogleUser) error {
	if !gu.EmailVerified {
		return fmt.Errorf("google email not verified")
	}
	if existing, err := store.UserByGoogleID(ctx, gu.Sub); err == nil {
		if existing.ID != current.ID {
			return fmt.Errorf("That Google account is already linked to another user")
		}
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := store.SetUserGoogleID(ctx, current.ID, gu.Sub); err != nil {
		return err
	}
	if current.Name == "" && gu.Name != "" {
		_ = store.UpdateUserName(ctx, current.ID, gu.Name)
	}
	return nil
}

// HandleGoogleLogin starts the OAuth flow: it stores a signed state cookie and
// redirects to Google's consent screen. mode=link (only honored for a
// signed-in user) attaches Google to the current account on callback.
func HandleGoogleLogin(oauth *auth.GoogleOAuth) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		mode := "login"
		if r.URL.Query().Get("mode") == "link" {
			if _, ok := auth.UserFrom(r.Context()); ok {
				mode = "link"
			}
		}
		next := auth.SafeNext(r.URL.Query().Get("next"))
		state, _, err := auth.GenerateToken()
		if err != nil {
			return err
		}
		verifier := auth.NewVerifier()
		auth.SetStateCookie(w, Conf.AppSecret,
			auth.OAuthState{State: state, Next: next, Mode: mode, Verifier: verifier}, !Conf.Debug)
		http.Redirect(w, r, oauth.AuthCodeURL(state, verifier), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

// HandleGoogleCallback completes the OAuth flow: verify state, exchange the
// code, fetch the profile, then either link to the current user or sign in.
func HandleGoogleCallback(store *database.Store, oauth *auth.GoogleOAuth) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		st, ok := auth.ReadStateCookie(r, Conf.AppSecret)
		auth.ClearStateCookie(w, !Conf.Debug)
		if !ok || r.URL.Query().Get("state") != st.State {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return nil
		}

		token, err := oauth.Exchange(r.Context(), r.URL.Query().Get("code"), st.Verifier)
		if err != nil {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Sign-in failed")
			return render(w, http.StatusOK, views.VerifyError(Conf.PageDataFor(account(r)), meta))
		}
		gu, err := oauth.FetchUser(r.Context(), token)
		if err != nil {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Sign-in failed")
			return render(w, http.StatusOK, views.VerifyError(Conf.PageDataFor(account(r)), meta))
		}

		// Link mode: attach to the already-signed-in user.
		if st.Mode == "link" {
			if cur, ok := auth.UserFrom(r.Context()); ok {
				if err := googleLink(r.Context(), store, cur, gu); err != nil {
					http.Redirect(w, r, "/account/security?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
					return nil
				}
				http.Redirect(w, r, "/account/security?flash="+url.QueryEscape("Google connected"), http.StatusSeeOther)
				return nil
			}
			// Session vanished mid-flow → fall through and sign in.
		}

		user, err := googleSignIn(r.Context(), store, gu)
		if err != nil {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Sign-in failed")
			return render(w, http.StatusOK, views.VerifyError(Conf.PageDataFor(account(r)), meta))
		}

		sessTok, sessHash, err := auth.GenerateToken()
		if err != nil {
			return err
		}
		if err := store.CreateSession(r.Context(), sessHash, user.ID,
			time.Now().Add(auth.SessionTTL), r.UserAgent(), clientIP(r, Conf.TrustProxy)); err != nil {
			return err
		}
		auth.SetSessionCookie(w, sessTok, !Conf.Debug)
		http.Redirect(w, r, auth.SafeNext(st.Next), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}
