package main

import (
	"github.com/tunedmystic/rio"
)

func main() {
	s := rio.NewServer()
	s.Handle("/", rio.BasicHttp("index page"))
	s.Serve(":3000")
}
