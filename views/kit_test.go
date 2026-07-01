package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestKit_RendersDashboardBreadcrumbsAndActivity(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Kit"}
	var b bytes.Buffer
	_ = Kit(pd, meta).Render(&b)
	html := b.String()

	if !strings.Contains(html, `aria-label="Breadcrumb"`) {
		t.Error("kit missing breadcrumbs demo")
	}
	if !strings.Contains(html, "Activity") {
		t.Error("kit missing activity feed")
	}
}

func TestKit_RendersAllSections(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Kit", Description: "d"}
	var b bytes.Buffer
	_ = Kit(pd, meta).Render(&b)
	html := b.String()

	for _, want := range []string{
		"<!DOCTYPE html>", // full page chrome
		"Foundations",
		"Data",
		"Marketing",
		"Feedback",
		"<svg",     // a chart rendered
		"<details", // an accordion / row menu rendered
		"1–10 of",  // data table pagination
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Kit output missing %q", want)
		}
	}
}
