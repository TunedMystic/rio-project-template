package main

import (
	"github.com/tunedmystic/rio"
)

func main() {
	rio.Templates(Templates)

	s := rio.NewServer()

	s.Handle("/", HandleIndex())
	s.Handle("/static/", HandleStatic())
	s.Handle("/version", HandleVersion())
	s.Handle("/about", HandleAbout())
	s.Handle("/privacy-policy", HandlePrivacyPolicy())

	s.Serve(Conf.Addr)
}
