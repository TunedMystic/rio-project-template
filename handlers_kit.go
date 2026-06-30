package main

import (
	"net/http"

	"app/views"

	"github.com/tunedmystic/rio"
)

// HandleKit renders the public component showcase / style guide.
func HandleKit() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Component Kit")
		return render(w, http.StatusOK, views.Kit(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}
