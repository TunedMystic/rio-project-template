package main

import (
	"embed"
	"log"

	"app/config"
	"app/database"

	"github.com/tunedmystic/rio"
)

// Build-time variables, injected via -ldflags.
var (
	BuildDate = "build-date"
	BuildHash = "build-hash"
	BuildEnv  = "production"
)

//go:embed all:static
var staticFS embed.FS

// Conf is the application configuration.
var Conf = config.New(BuildEnv)

func main() {
	db, err := database.Open(Conf.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := database.MigrateUp(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := database.NewStore(db)

	s := rio.NewServer()
	s.Handle("/", HandleHome())
	s.Handle("/messages", HandleMessages(store))
	s.Handle("/about", HandleAbout())
	s.Handle("/privacy-policy", HandlePrivacyPolicy())
	s.Handle("/version", HandleVersion())
	s.Handle("/healthz", HandleHealth(db))
	s.Handle("/static/", HandleStatic())

	log.Fatal(s.Serve(Conf.Addr))
}
