package main

import (
	"net/http"
	"strings"

	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
	"github.com/tunedmystic/rio/dom"
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
			return render(w, http.StatusNotFound, views.NotFound(Conf.PageData(), meta))
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "")
		return render(w, http.StatusOK, views.Home(Conf.PageData(), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleMessages(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == http.MethodPost {
			body := strings.TrimSpace(r.FormValue("body"))
			if body != "" {
				if err := store.CreateMessage(r.Context(), body); err != nil {
					return err
				}
			}
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
			return nil
		}

		msgs, err := store.ListMessages(r.Context())
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
		return render(w, http.StatusOK, views.Messages(Conf.PageData(), meta, msgs))
	}
	return rio.MakeHandler(fn)
}

func HandleAbout() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "About")
		return render(w, http.StatusOK, views.About(Conf.PageData(), meta))
	}
	return rio.MakeHandler(fn)
}

func HandlePrivacyPolicy() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Privacy Policy")
		return render(w, http.StatusOK, views.PrivacyPolicy(Conf.PageData(), meta))
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

func HandleStatic() http.Handler {
	cache := rio.CacheControlWithAge(1_209_600) // 2 weeks
	return cache(rio.FileServer(staticFS))
}
