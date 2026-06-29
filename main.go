package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app/auth"
	"app/config"
	"app/database"
	"app/email"

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
var Conf = config.New(BuildEnv, BuildHash)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run wires up the app and serves until a shutdown signal arrives, then closes
// the database cleanly. Returning an error (instead of log.Fatal everywhere)
// keeps deferred cleanup running on the way out.
func run() error {
	db, err := database.Open(Conf.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := database.MigrateUp(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if !Conf.Debug && Conf.AppSecret == "" {
		return fmt.Errorf("APP_SECRET must be set in production")
	}

	store := database.NewStore(db)
	sender := email.New(Conf.PostmarkToken, Conf.EmailFrom)
	loginLimiter := auth.NewLimiter(5, 15*time.Minute)

	s := rio.NewServer()
	s.Use(auth.LoadUser(store)) // server-wide: populate the current user

	s.Handle("/", HandleHome())
	s.Handle("/messages", HandleMessages(store))
	s.Handle("/about", HandleAbout())
	s.Handle("/privacy-policy", HandlePrivacyPolicy())
	s.Handle("/version", HandleVersion())
	s.Handle("/healthz", HandleHealth(db))
	s.Handle("/robots.txt", HandleRobots())

	// Auth
	s.Handle("/login", HandleLogin(store, sender, loginLimiter))
	s.Handle("/login/sent", HandleLoginSent())
	s.Handle("/auth/verify", HandleVerify(store))
	s.Handle("/logout", HandleLogout(store))

	// Account (authenticated)
	s.Handle("/account", auth.RequireUser(HandleAccount(store)))
	s.Handle("/account/security", auth.RequireUser(HandleSecurity(store)))
	s.Handle("/account/sessions/revoke", auth.RequireUser(HandleRevokeSession(store)))
	s.Handle("/account/sessions/revoke-all", auth.RequireUser(HandleRevokeAllSessions(store)))
	s.Handle("/account/billing", auth.RequireUser(HandleBilling()))
	s.Handle("/account/delete", auth.RequireUser(HandleDeleteAccount(store)))

	s.Handle("/static/", HandleStatic())

	// Cancel the context on Ctrl-C or SIGTERM (e.g. `docker stop`) so the
	// server drains in-flight requests before the deferred db.Close runs.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", Conf.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", Conf.Addr, err)
	}

	log.Printf("listening on %s", Conf.Addr)
	return serve(ctx, newHTTPServer(Conf.Addr, s.Handler()), ln)
}
