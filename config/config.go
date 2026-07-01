package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tunedmystic/rio/ui"
)

// Account is the current-user info the nav needs (empty when logged out).
type Account struct {
	LoggedIn bool
	Name     string
	Email    string
}

// Link is an anchor link used in nav/footer.
type Link struct {
	Text string
	Href string
}

// ProductKind distinguishes a recurring subscription from a one-time purchase.
type ProductKind int

const (
	Subscription ProductKind = iota
	OneTime
)

// Product is a sellable item in the catalog. PriceID comes from env; an empty
// PriceID means the product is not available (button hidden).
type Product struct {
	Key     string // stable id used in URLs/entitlements, e.g. "pro", "ebook"
	Name    string
	Kind    ProductKind
	PriceID string
}

// Available reports whether the product has a configured Stripe price.
func (p Product) Available() bool { return p.PriceID != "" }

// Meta is per-page metadata for the document head.
type Meta struct {
	Title       string
	Description string
	Heading     string
	PageURL     string
}

// PageData is the subset of config the views need to render a page.
type PageData struct {
	SiteName      string
	AssetVersion  string
	Tokens        ui.Tokens
	ThemeVars     []ThemeVar
	HeaderLinks   []Link
	FooterLinks   []Link
	Account       Account
	GoogleEnabled bool
}

// Config holds the product configuration. ProjectName is the per-clone seam.
type Config struct {
	ProjectName         string
	SiteName            string
	SiteURL             string
	Description         string
	Addr                string
	Debug               bool
	DBPath              string
	AssetVersion        string
	Tokens              ui.Tokens
	Theme               Theme
	HeaderLinks         []Link
	FooterLinks         []Link
	BaseURL             string
	AppSecret           string
	PostmarkToken       string
	EmailFrom           string
	TrustProxy          bool
	GoogleClientID      string
	GoogleClientSecret  string
	StripeSecretKey     string
	StripeWebhookSecret string
	Products            []Product

	// Operational
	SessionCleanupInterval time.Duration
	TokenCleanupInterval   time.Duration
	ErrorWebhookURL        string
	AdminEmails            []string
}

// New builds the Config. buildEnv comes from the main package's build-time var
// ("debug" selects development defaults); buildHash versions static assets so a
// deploy busts the browser cache.
func New(buildEnv, buildHash string) Config {
	debug := buildEnv == "debug"

	c := Config{
		ProjectName:  "riostarter", // <-- change this per product; sets the db file name
		SiteName:     "Rio Starter",
		SiteURL:      "https://riostarter.example.com",
		Description:  "A starter built with rio. Clone it, set ProjectName, ship.",
		Addr:         addrFromEnv(),
		Debug:        debug,
		AssetVersion: buildHash,
		Theme:        ThemeSlateIndigo, // <-- compile-time preset; set ThemeDusk for dark
		HeaderLinks: []Link{
			{Text: "Messages", Href: "/messages"},
			{Text: "About", Href: "/about"},
		},
		FooterLinks: []Link{
			{Text: "Home", Href: "/"},
			{Text: "About", Href: "/about"},
			{Text: "Kit", Href: "/kit"},
			{Text: "Privacy Policy", Href: "/privacy-policy"},
			{Text: "Terms", Href: "/terms"},
		},
	}
	c.Tokens = c.Theme.Tokens()
	c.DBPath = DBPath(c.ProjectName, debug)
	c.BaseURL = baseURLFromEnv(c.Addr)
	c.AppSecret = appSecretFromEnv(debug)
	c.PostmarkToken = os.Getenv("POSTMARK_TOKEN")
	c.EmailFrom = cmpOr(os.Getenv("EMAIL_FROM"), "noreply@localhost")
	c.TrustProxy = isTruthy(os.Getenv("TRUST_PROXY"))
	c.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	c.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	c.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	c.StripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	// Product catalog — the per-clone seam. Edit this list per product; each
	// price id comes from env so the same binary works across environments.
	c.Products = []Product{
		{Key: "pro", Name: "Pro", Kind: Subscription, PriceID: os.Getenv("STRIPE_PRICE_PRO")},
		{Key: "ebook", Name: "E-book", Kind: OneTime, PriceID: os.Getenv("STRIPE_PRICE_EBOOK")},
	}
	c.SessionCleanupInterval = durationFromEnv("SESSION_CLEANUP_INTERVAL", time.Hour)
	c.TokenCleanupInterval = durationFromEnv("TOKEN_CLEANUP_INTERVAL", time.Hour)
	c.ErrorWebhookURL = os.Getenv("ERROR_WEBHOOK_URL")
	c.AdminEmails = csvEnv("ADMIN_EMAILS")
	return c
}

// addrFromEnv resolves the listen address: ADDR (full host:port) wins, else
// PORT (":<port>"), else the :3000 default.
func addrFromEnv() string {
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":3000"
}

// DBPath derives the SQLite file path from the project name. The directory is
// DB_DIR (default /data in prod, ./data in dev), the file is <projectName>.db —
// keeping each project's database unique on a shared volume. The directory is
// created on open if missing (see database.Open).
func DBPath(projectName string, debug bool) string {
	dir := os.Getenv("DB_DIR")
	if dir == "" {
		dir = "/data"
		if debug {
			dir = "data"
		}
	}
	return filepath.Join(dir, projectName+".db")
}

// PageData returns the view-facing subset of the config.
func (c Config) PageData() PageData {
	return PageData{
		SiteName:      c.SiteName,
		AssetVersion:  c.AssetVersion,
		Tokens:        c.Tokens,
		ThemeVars:     c.Theme.Vars(),
		HeaderLinks:   c.HeaderLinks,
		FooterLinks:   c.FooterLinks,
		GoogleEnabled: c.GoogleEnabled(),
	}
}

// PageDataFor returns view data including the current-user account info.
func (c Config) PageDataFor(a Account) PageData {
	pd := c.PageData()
	pd.Account = a
	return pd
}

// baseURLFromEnv resolves the absolute base URL for links. BASE_URL wins; else
// http://localhost<addr-port> for dev convenience.
func baseURLFromEnv(addr string) string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return v
	}
	port := strings.TrimPrefix(addr, ":")
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		port = addr[i+1:]
	}
	return "http://localhost:" + port
}

// appSecretFromEnv returns APP_SECRET. In dev it falls back to a known default
// (with a warning); in prod it returns "" so the caller can fail fast.
func appSecretFromEnv(debug bool) string {
	if v := os.Getenv("APP_SECRET"); v != "" {
		return v
	}
	if debug {
		log.Println("WARNING: APP_SECRET unset; using an insecure dev default")
		return "dev-only-insecure-secret-change-me"
	}
	return ""
}

func cmpOr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

// durationFromEnv parses key as a Go duration (e.g. "1h", "30m"), falling back
// to def when unset or invalid (logging a warning in the invalid case).
func durationFromEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("WARNING: invalid %s=%q; using %s", key, v, def)
		return def
	}
	return d
}

// csvEnv reads key as a comma-separated list, trimming and lowercasing each
// item and dropping empties. Returns an empty slice when unset.
func csvEnv(key string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.ToLower(strings.TrimSpace(p)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// isTruthy returns true when v is "1", "true", or "yes" (case-insensitive).
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// GoogleEnabled reports whether Google OAuth login is configured.
func (c Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

// NewMeta builds per-page metadata, defaulting title/description from the config.
func (c Config) NewMeta(pageURL, heading string) Meta {
	title := c.SiteName
	if heading != "" {
		title = heading + " - " + c.SiteName
	}
	return Meta{
		Title:       title,
		Description: c.Description,
		Heading:     heading,
		PageURL:     pageURL,
	}
}

// StripeEnabled reports whether billing is configured.
func (c Config) StripeEnabled() bool { return c.StripeSecretKey != "" }

// ProductByKey finds a catalog product by its key.
func (c Config) ProductByKey(key string) (Product, bool) {
	for _, p := range c.Products {
		if p.Key == key {
			return p, true
		}
	}
	return Product{}, false
}
