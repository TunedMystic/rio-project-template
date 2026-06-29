package main

import (
	"database/sql"
	"io"
	"net/http"
	"strings"

	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/forms"
)

// render writes an HTML dom node with the given status.
func render(w http.ResponseWriter, status int, node dom.Node) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return node.Render(w)
}

func HandleHome() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		// Treat any unknown path under "/" as 404.
		if r.URL.Path != "/" {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Not found")
			return render(w, http.StatusNotFound, views.NotFound(Conf.PageDataFor(account(r)), meta))
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "")
		return render(w, http.StatusOK, views.Home(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleMessages(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == http.MethodPost {
			// Validate with rio/forms: required, and a sane max length.
			body := strings.TrimSpace(r.FormValue("body"))
			form := forms.New()
			form.CleanString("body", body, forms.StrRequired(), forms.StrLte(280))

			if !form.IsValid() {
				msgs, err := store.ListMessages(r.Context())
				if err != nil {
					return err
				}
				field := form.MustField("body")
				meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
				return render(w, http.StatusUnprocessableEntity,
					views.Messages(Conf.PageDataFor(account(r)), meta, msgs, field.Value(), field.Err().Error()))
			}

			if err := store.CreateMessage(r.Context(), form.CleanedString("body")); err != nil {
				return err
			}
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
			return nil
		}

		msgs, err := store.ListMessages(r.Context())
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
		return render(w, http.StatusOK, views.Messages(Conf.PageDataFor(account(r)), meta, msgs, "", ""))
	}
	return rio.MakeHandler(fn)
}

func HandleAbout() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "About")
		return render(w, http.StatusOK, views.About(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}

func HandlePrivacyPolicy() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Privacy Policy")
		return render(w, http.StatusOK, views.PrivacyPolicy(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleVersion() http.Handler {
	version := struct {
		BuildDate string
		BuildHash string
		BuildProd bool
	}{BuildDate: BuildDate, BuildHash: BuildHash, BuildProd: !Conf.Debug}

	fn := func(w http.ResponseWriter, r *http.Request) error {
		return rio.Json200(w, version)
	}
	return rio.MakeHandler(fn)
}

// HandleRobots serves a permissive robots.txt. Tighten or point at a sitemap
// per product.
func HandleRobots() http.Handler {
	const body = "User-agent: *\nAllow: /\n"
	fn := func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, body)
		return nil
	}
	return rio.MakeHandler(fn)
}

// HandleHealth reports service health by pinging the database. It returns 200
// when reachable and 503 otherwise — suitable for load balancers and orchestrators.
func HandleHealth(db *sql.DB) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if err := db.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "unavailable")
			return nil
		}
		_, _ = io.WriteString(w, "ok")
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleStatic() http.Handler {
	fileServer := rio.FileServer(staticFS)
	if Conf.Debug {
		// In dev, never cache: rebuilt CSS/assets show up on a normal reload.
		return noStore(fileServer)
	}
	return rio.CacheControlWithAge(1_209_600)(fileServer) // 2 weeks in prod
}

// noStore disables caching, so the browser always refetches the asset.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
