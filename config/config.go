package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

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

// Meta is per-page metadata for the document head.
type Meta struct {
	Title       string
	Description string
	Heading     string
	PageURL     string
}

// PageData is the subset of config the views need to render a page.
type PageData struct {
	SiteName     string
	AssetVersion string
	Tokens       ui.Tokens
	HeaderLinks  []Link
	FooterLinks  []Link
	Account      Account
}

// Config holds the product configuration. ProjectName is the per-clone seam.
type Config struct {
	ProjectName   string
	SiteName      string
	SiteURL       string
	Description   string
	Addr          string
	Debug         bool
	DBPath        string
	AssetVersion  string
	Tokens        ui.Tokens
	HeaderLinks   []Link
	FooterLinks   []Link
	BaseURL       string
	AppSecret     string
	PostmarkToken string
	EmailFrom     string
	TrustProxy    bool
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
		Tokens:       defaultTokens(),
		HeaderLinks: []Link{
			{Text: "Messages", Href: "/messages"},
			{Text: "About", Href: "/about"},
		},
		FooterLinks: []Link{
			{Text: "Home", Href: "/"},
			{Text: "About", Href: "/about"},
			{Text: "Privacy Policy", Href: "/privacy-policy"},
		},
	}
	c.DBPath = DBPath(c.ProjectName, debug)
	c.BaseURL = baseURLFromEnv(c.Addr)
	c.AppSecret = appSecretFromEnv(debug)
	c.PostmarkToken = os.Getenv("POSTMARK_TOKEN")
	c.EmailFrom = cmpOr(os.Getenv("EMAIL_FROM"), "noreply@localhost")
	c.TrustProxy = isTruthy(os.Getenv("TRUST_PROXY"))
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
// DB_DIR (default /data in prod, the working dir in dev), the file is
// <projectName>.db — keeping each project's database unique on a shared volume.
func DBPath(projectName string, debug bool) string {
	dir := os.Getenv("DB_DIR")
	if dir == "" {
		dir = "/data"
		if debug {
			dir = "."
		}
	}
	return filepath.Join(dir, projectName+".db")
}

// PageData returns the view-facing subset of the config.
func (c Config) PageData() PageData {
	return PageData{
		SiteName:     c.SiteName,
		AssetVersion: c.AssetVersion,
		Tokens:       c.Tokens,
		HeaderLinks:  c.HeaderLinks,
		FooterLinks:  c.FooterLinks,
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

// isTruthy returns true when v is "1", "true", or "yes" (case-insensitive).
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes":
		return true
	}
	return false
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

// defaultTokens is the starter brand. Products edit this literal.
func defaultTokens() ui.Tokens {
	return ui.Tokens{
		FontFamily:   `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:   "16px",
		FontSizeBase: "18px",
		FontSizeLg:   "20px",
		FontSizeXl:   "24px",
		FontSize2xl:  "30px",
		// A calm, warm palette: a single deep-teal accent on warm-stone
		// neutrals, white cards floating on a soft cream canvas. Edit these
		// to rebrand the whole app — every component reads these variables.
		ColorPrimary:      "#0d9488", // vibrant teal
		OnPrimary:         "#ffffff",
		ColorSecondary:    "#57534e", // warm stone-600
		OnSecondary:       "#ffffff",
		ColorBackground:   "#fcfbf9", // near-white canvas (a whisper of warm)
		ColorSurface:      "#ffffff", // white cards
		ColorText:         "#1c1917", // stone-900
		ColorTextMuted:    "#78716c", // stone-500
		ColorBorder:       "#e7e5e4", // stone-200
		ColorSuccess:      "#15803d",
		ColorWarning:      "#b45309",
		ColorDanger:       "#be123c",
		ColorInfo:         "#0e7490",
		RadiusBase:        "0.75rem",
		FontWeightHeading: "700",
	}
}
