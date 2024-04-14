package main

import (
	"embed"
	"net/http"
	"strings"
)

// ------------------------------------------------------------------
//
//
// Type: Config
//
//
// ------------------------------------------------------------------

// Config stores the necessary info about the application.
type Config struct {
	SiteName        string
	SiteHost        string
	SiteTagline     string
	SiteDescription string
	SiteTitle       string
	SiteUrl         string
	SiteEmail       string
	Addr            string
	Debug           bool
	Image           Image
	HomeLink        Link
	HeaderLinks     []Link
	FooterLinks     []Link
}

var Conf = NewConfig()

func NewConfig() Config {
	c := Config{}
	c.SiteName = "Rio Starter"
	c.SiteHost = "riostarter.example.com"
	c.SiteTagline = "The golang project template"
	c.SiteDescription = "The golang project template, built with Rio! Get up and running quickly. Deploy when you're ready to ship!"
	c.SiteTitle = join(c.SiteName, " - ", c.SiteTagline)
	c.SiteUrl = join("https://", c.SiteHost)
	c.SiteEmail = join("admin@", c.SiteHost)
	c.Addr = ":3000"
	c.Debug = BuildEnv == "debug"

	c.Image = Image{
		Url:    "/static/img/meta-img.webp",
		Type:   "webp",
		Alt:    c.SiteTitle,
		Width:  "800",
		Height: "450",
	}

	c.HomeLink = Link{Text: "Home", Href: "/"}

	c.HeaderLinks = []Link{
		{Text: "About", Href: "/about"},
	}

	c.FooterLinks = []Link{
		{Text: "Home", Href: "/"},
		{Text: "About", Href: "/about"},
		{Text: "Privacy Policy", Href: "/privacy-policy"},
	}

	if c.Debug {
		c.SiteHost = join("localhost", c.Addr)
		c.SiteUrl = join("http://localhost", c.Addr)
		c.SiteEmail = join("admin@localhost", c.Addr)
	}

	return c
}

func (c Config) NewRenderData(r *http.Request) RenderData {
	pageUrl := ""
	if r != nil {
		pageUrl = join(c.SiteUrl, r.URL.RequestURI())
	}

	return RenderData{
		Conf:            c,
		Image:           c.Image,
		MetaTitle:       c.SiteTitle,
		MetaDescription: c.SiteDescription,
		Heading:         c.SiteTagline,
		PageUrl:         pageUrl,
	}
}

// ------------------------------------------------------------------
//
//
// Type: RenderData
//
//
// ------------------------------------------------------------------

// RenderData represents data for template rendering.
type RenderData struct {
	MetaTitle       string
	MetaDescription string
	Heading         string
	PageUrl         string
	NotFound        bool
	Conf            Config
	Image           Image
	Breadcrumbs     []Link
}

// ------------------------------------------------------------------
//
//
// Type: Link
//
//
// ------------------------------------------------------------------

// Link represents an anchor link.
type Link struct {
	Text string
	Href string
}

// ------------------------------------------------------------------
//
//
// Type: Image
//
//
// ------------------------------------------------------------------

// Image represents an image media object.
type Image struct {
	Url    string
	Alt    string
	Type   string
	Width  string
	Height string
}

// ------------------------------------------------------------------
//
//
// Embed Filesystems
//
//
// ------------------------------------------------------------------

//go:embed "static"
var Static embed.FS

//go:embed "templates"
var Templates embed.FS

// ------------------------------------------------------------------
//
//
// Build-Time Variables
//
//
// ------------------------------------------------------------------

// Values injected at build time.
var (
	BuildDate = "build-date"
	BuildHash = "build-hash"
	BuildEnv  = "production"
)

// ------------------------------------------------------------------
//
//
// Helper functions
//
//
// ------------------------------------------------------------------

func join(s ...string) string {
	return strings.Join(s, "")
}
