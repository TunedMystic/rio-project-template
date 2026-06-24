package config

import (
	"os"
	"path/filepath"

	"github.com/tunedmystic/rio/ui"
)

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
	SiteName    string
	Tokens      ui.Tokens
	HeaderLinks []Link
	FooterLinks []Link
}

// Config holds the product configuration. ProjectName is the per-clone seam.
type Config struct {
	ProjectName string
	SiteName    string
	SiteURL     string
	Description string
	Addr        string
	Debug       bool
	DBPath      string
	Tokens      ui.Tokens
	HeaderLinks []Link
	FooterLinks []Link
}

// New builds the Config. buildEnv comes from the main package's build-time var;
// "debug" selects development defaults.
func New(buildEnv string) Config {
	debug := buildEnv == "debug"

	c := Config{
		ProjectName: "riostarter", // <-- change this per product; sets the db file name
		SiteName:    "Rio Starter",
		SiteURL:     "https://riostarter.example.com",
		Description: "A starter built with rio. Clone it, set ProjectName, ship.",
		Addr:        ":3000",
		Debug:       debug,
		Tokens:      defaultTokens(),
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
	return c
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
		SiteName:    c.SiteName,
		Tokens:      c.Tokens,
		HeaderLinks: c.HeaderLinks,
		FooterLinks: c.FooterLinks,
	}
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
		FontFamily:        `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:        "16px",
		FontSizeBase:      "18px",
		FontSizeLg:        "20px",
		FontSizeXl:        "24px",
		FontSize2xl:       "30px",
		// A calm, warm palette: a single deep-teal accent on warm-stone
		// neutrals, white cards floating on a soft cream canvas. Edit these
		// to rebrand the whole app — every component reads these variables.
		ColorPrimary:      "#0f766e", // deep teal
		OnPrimary:         "#ffffff",
		ColorSecondary:    "#57534e", // warm stone-600
		OnSecondary:       "#ffffff",
		ColorBackground:   "#f8f6f1", // warm cream canvas
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
