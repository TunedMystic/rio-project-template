package main

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/tunedmystic/rio"
)

// ------------------------------------------------------------------
//
//
// App Settings
//
//
// ------------------------------------------------------------------

var (
	SiteName        = "Rio Starter"
	SiteHost        = "riostarter.example.com"
	SiteTagline     = "The golang project template"
	SiteDescription = "The golang project template, built with Rio! Get up and running quickly. Deploy when you're ready to ship!"
	SiteImageUrl    = "/static/img/meta-img.webp"
	SiteImageType   = "webp"
	SiteImageWidth  = 800
	SiteImageHeight = 450
)

var (
	SiteTitle    = fmt.Sprintf("%s - %s", SiteName, SiteTagline)
	SiteUrl      = fmt.Sprintf("https://%s", SiteHost)
	SiteEmail    = fmt.Sprintf("admin@%s", SiteHost)
	SiteImageAlt = SiteTitle
)

// Template map for HTML templates.
var Funcs template.FuncMap

// ------------------------------------------------------------------
//
//
// Type: SiteLink
//
//
// ------------------------------------------------------------------

// SiteLink stores data for an anchor link.
type SiteLink struct {
	Text string
	Href string
}

var NavbarLinks = []SiteLink{
	{Text: "Home", Href: "/"},
	{Text: "About", Href: "/about"},
	// {Text: "Privacy", Href: "/privacy-policy"},
	// {Text: "Sitemap", Href: "/sitemap"},
	// {Text: "Robots", Href: "/robots"},
}

var FooterLinks = []SiteLink{
	{Text: "Home", Href: "/"},
	{Text: "About", Href: "/about"},
	{Text: "Privacy Policy", Href: "/privacy-policy"},
}

// ------------------------------------------------------------------
//
//
// System Settings
//
//
// ------------------------------------------------------------------

var (
	Debug     = BuildEnv == "debug"
	Addr      = ":3000"
	LocalHost = fmt.Sprintf("localhost%s", Addr)
)

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
// Init: Override settings and initialize template funcmap.
//
//
// ------------------------------------------------------------------

func init() {
	if Debug {
		SiteHost = LocalHost
		SiteUrl = fmt.Sprintf("http://%s", LocalHost)
		SiteEmail = fmt.Sprintf("admin@%s", LocalHost)
	}

	Funcs = template.FuncMap{
		"Debug":           rio.WrapBool(Debug),
		"SiteHost":        rio.WrapString(SiteHost),
		"SiteUrl":         rio.WrapString(SiteUrl),
		"SiteName":        rio.WrapString(SiteName),
		"SiteTagline":     rio.WrapString(SiteTagline),
		"SiteEmail":       rio.WrapString(SiteEmail),
		"SiteImageUrl":    rio.WrapString(SiteImageUrl),
		"SiteImageType":   rio.WrapString(SiteImageType),
		"SiteImageAlt":    rio.WrapString(SiteImageAlt),
		"SiteImageWidth":  rio.WrapInt(SiteImageWidth),
		"SiteImageHeight": rio.WrapInt(SiteImageHeight),
		"NavbarLinks": func() []SiteLink {
			return NavbarLinks
		},
		"FooterLinks": func() []SiteLink {
			return FooterLinks
		},
		"eq": func(x, y interface{}) bool {
			return x == y
		},
		"sub": func(y, x int) int {
			return x - y
		},
	}
}
