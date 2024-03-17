package main

import (
	"fmt"
	"net/http"

	"github.com/tunedmystic/rio"
)

// ------------------------------------------------------------------
//
//
// Main: Register Routes and Start Server
//
//
// ------------------------------------------------------------------

func main() {
	s := rio.NewServer()

	s.Handle("/", NotFound(rio.MakeHandler(HandleIndex)))
	s.Handle("/static/", CachePolicy(rio.FileServer(Static)))
	s.Handle("/version", rio.MakeHandler(HandleVersion()))
	s.Handle("/about", rio.MakeHandler(HandleAbout))
	s.Handle("/privacy-policy", rio.MakeHandler(HandlePrivacyPolicy))

	rio.Templates(Templates, rio.WithFuncMap(Funcs))
	s.Serve(Addr)
}

// ------------------------------------------------------------------
//
//
// Http Handlers
//
//
// ------------------------------------------------------------------

func HandleIndex(w http.ResponseWriter, r *http.Request) error {
	rd := NewRenderData(r)
	return rio.Render(w, "index", http.StatusOK, rd)
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

func HandleAbout(w http.ResponseWriter, r *http.Request) error {
	rd := NewRenderData(r)
	rd.MetaTitle = fmt.Sprintf("About %s - Learn More about Us and Our Story", SiteName)
	rd.MetaDescription = fmt.Sprintf("Welcome to %s! Learn more about our story and mission. Drop us a line if you have any questions or suggestions.", SiteName)
	rd.Heading = "About"
	return rio.Render(w, "about", http.StatusOK, rd)
}

func HandlePrivacyPolicy(w http.ResponseWriter, r *http.Request) error {
	rd := NewRenderData(r)
	rd.MetaTitle = fmt.Sprintf("Privacy Policy - Information and Collection for %v", SiteName)
	rd.MetaDescription = fmt.Sprintf("The privacy policy page for %s describes how we may collect information when you use our services.", SiteName)
	rd.Heading = "Privacy Policy"
	return rio.Render(w, "privacy-policy", http.StatusOK, rd)
}

/*
hr: bg-sky-500
link: text-sky-700
*/
