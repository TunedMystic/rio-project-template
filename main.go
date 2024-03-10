package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/tunedmystic/rio"
)

//go:embed "static"
var Static embed.FS

//go:embed "templates"
var Templates embed.FS

// Values injected at build time.
var (
	BuildDate  = "build-date"
	BuildHash  = "build-hash"
	BuildDebug = "true"
)

var (
	Debug     = BuildDebug == "true"
	Addr      = ":3000"
	LocalHost = fmt.Sprintf("localhost%s", Addr)
)

var (
	SiteName        = "Rush"
	SiteHost        = "rush.dev"
	SiteTagline     = "The golang project template!"
	SiteDescription = "The golang project template, built with Rio! Get up and running quickly. Deploy when you're ready to ship!"
	SiteImageUrl    = "/static/img/meta-img.webp"
	SiteImageType   = "webp"
	SiteImageAlt    = SiteTitle
	SiteImageWidth  = 800
	SiteImageHeight = 450
)

var (
	SiteTitle = fmt.Sprintf("%s - %s", SiteName, SiteTagline)
	SiteUrl   = fmt.Sprintf("https://%s", SiteHost)
	SiteEmail = fmt.Sprintf("admin@%s", SiteHost)
)

var Funcs template.FuncMap

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
	}
}

// RenderData stores the necessary data for template rendering.
type RenderData struct {
	// The Url of the current page.
	PageUrl string

	// The NotFound error.
	NotFound bool

	// The title tag, meta title tag and og:title tag
	MetaTitle string

	// The meta description
	MetaDescription string

	// The page heading, h1
	Heading string
}

func NewRenderData(r *http.Request) RenderData {
	pageUrl := ""
	if r != nil {
		pageUrl = fmt.Sprintf("%s%s", SiteUrl, r.URL.RequestURI())
	}

	return RenderData{
		PageUrl:         pageUrl,
		MetaTitle:       SiteTitle,
		MetaDescription: SiteDescription,
		Heading:         SiteTagline,
	}
}

func main() {
	rio.Templates(Templates, rio.WithFuncMap(Funcs))

	s := rio.NewServer()

	s.Handle("/", NotFound(rio.MakeHandler(HandleIndex)))
	s.Handle("/version.json", rio.MakeHandler(HandleVersion()))
	s.Handle("/static/", rio.FileServer(Static))

	s.Serve(Addr)
}

func HandleVersion() rio.HandlerFunc {
	v := map[string]any{
		"Production": !Debug,
		"BuildDate":  BuildDate,
		"BuildHash":  BuildHash,
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		return rio.Json200(w, v)
	}
}

// NotFound is a middleware which renders a 404 Not Found error page
// if the request path is not "/".
func NotFound(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			rd := NewRenderData(r)
			rd.NotFound = true

			if err := rio.Render(w, "404", http.StatusNotFound, rd); err != nil {
				rio.LogError(err)
				rio.Http500(w)
			}
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func HandleIndex(w http.ResponseWriter, r *http.Request) error {
	rd := NewRenderData(r)
	return rio.Render(w, "index", http.StatusOK, rd)
}
