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
