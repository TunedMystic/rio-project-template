package main

import (
	"net/http"

	"github.com/tunedmystic/rio"
)

// NotFound is a middleware which renders a 404 Not Found error page
// if the request path is not "/".
func NotFound(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			rd := NewRenderData(r)
			rd.NotFound = true

			if err := rio.Render(w, "404", http.StatusNotFound, rd); err != nil {
				rio.LogError(err)
				rio.Http500(w)
			}
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
