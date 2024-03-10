package main

import (
	"net/http"

	"github.com/tunedmystic/rio"
)

func main() {
	s := rio.NewServer()

	s.Handle("/", NotFound(rio.MakeHandler(HandleIndex)))
	s.Handle("/static/", CachePolicy(rio.FileServer(Static)))
	s.Handle("/version.json", rio.MakeHandler(HandleVersion()))

	rio.Templates(Templates, rio.WithFuncMap(Funcs))
	s.Serve(Addr)
}

func HandleVersion() rio.HandlerFunc {
	v := map[string]any{
		"BuildDate": BuildDate,
		"BuildHash": BuildHash,
		"BuildProd": !Debug,
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		return rio.Json200(w, v)
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) error {
	rd := NewRenderData(r)
	return rio.Render(w, "index", http.StatusOK, rd)
}
