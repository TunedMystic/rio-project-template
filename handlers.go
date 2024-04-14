package main

import (
	"net/http"

	"github.com/tunedmystic/rio"
)

func HandleIndex() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		rd := Conf.NewRenderData(r)
		return rio.Render(w, "index", http.StatusOK, rd)
	}

	notFound := func(next rio.HandlerFunc) rio.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			if r.URL.Path != "/" {
				rd := Conf.NewRenderData(r)
				rd.NotFound = true
				return rio.Render(w, "404", http.StatusNotFound, rd)
			}
			return next(w, r)
		}
	}

	return rio.MakeHandler(notFound(fn))
}

func HandleStatic() http.Handler {
	age := 1_209_600 // 2 weeks
	cache := rio.CacheControlWithAge(age)
	return cache(rio.FileServer(Static))
}

func HandleVersion() http.Handler {
	version := struct {
		BuildDate string
		BuildHash string
		BuildProd bool
	}{
		BuildDate: BuildDate,
		BuildHash: BuildHash,
		BuildProd: !Conf.Debug,
	}

	fn := func(w http.ResponseWriter, r *http.Request) error {
		return rio.Json200(w, version)
	}

	return rio.MakeHandler(fn)
}

func HandleAbout() http.Handler {
	MetaTitle := join("About ", Conf.SiteName, " - Learn More about Us and Our Story")
	MetaDescription := join("Welcome to ", Conf.SiteName, "! Learn more about our story and mission. Drop us a line if you have any questions or suggestions.")
	Heading := "About"

	fn := func(w http.ResponseWriter, r *http.Request) error {
		rd := Conf.NewRenderData(r)
		rd.MetaTitle = MetaTitle
		rd.MetaDescription = MetaDescription
		rd.Heading = Heading

		return rio.Render(w, "about", http.StatusOK, rd)
	}

	return rio.MakeHandler(fn)
}

func HandlePrivacyPolicy() http.Handler {
	MetaTitle := join("Privacy Policy - Information and Collection for ", Conf.SiteName)
	MetaDescription := join("The privacy policy page for ", Conf.SiteName, " describes how we may collect information when you use our services.")
	Heading := "Privacy Policy"

	fn := func(w http.ResponseWriter, r *http.Request) error {
		rd := Conf.NewRenderData(r)
		rd.MetaTitle = MetaTitle
		rd.MetaDescription = MetaDescription
		rd.Heading = Heading

		return rio.Render(w, "privacy-policy", http.StatusOK, rd)
	}

	return rio.MakeHandler(fn)
}
