package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestHome_RendersLandingSections(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Home", Description: "d"}
	var b bytes.Buffer
	_ = Home(pd, meta).Render(&b)
	html := b.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		pd.SiteName, // hero references the product
		"Pricing",   // pricing section heading
		"<details",  // FAQ accordion
		"<svg",      // a chart or hero visual
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Home output missing %q", want)
		}
	}
}

func TestTerms_RendersHeading(t *testing.T) {
	pd := testPageData()
	var b bytes.Buffer
	_ = Terms(pd, config.Meta{Title: "Terms"}).Render(&b)
	html := b.String()
	if !strings.Contains(html, "Terms of Service") {
		t.Error("Terms output missing heading")
	}
}

func TestPages_RendersGroupsAndLinks(t *testing.T) {
	groups := []PageGroup{
		{Title: "Public", Links: []PageLink{
			{Label: "About", Href: "/about"},
			{Label: "Component Kit", Href: "/kit"},
		}},
		{Title: "Account", Links: []PageLink{
			{Label: "Account", Href: "/account", Note: "requires login"},
		}},
	}
	html := render(Pages(testPageData(), config.Meta{Title: "Pages"}, groups))
	for _, want := range []string{"Public", "Account", `href="/about"`, `href="/kit"`, `href="/account"`, "requires login"} {
		if !strings.Contains(html, want) {
			t.Errorf("Pages output missing %q", want)
		}
	}
}
