package main

import (
	"fmt"
	"net/http"
)

// ------------------------------------------------------------------
//
//
// Type: RenderData
//
//
// ------------------------------------------------------------------

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

	// The trail of links to the current page
	Breadcrumbs []SiteLink
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
