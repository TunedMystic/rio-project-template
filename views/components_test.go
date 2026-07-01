package views

import (
	"strings"
	"testing"

	"app/config"
)

func TestNavbar_NoHardcodedCream(t *testing.T) {
	html := render(navbar(testPageData()))
	if strings.Contains(html, "#f8f5ee") {
		t.Error("navbar still contains hardcoded cream #f8f5ee")
	}
	if !strings.Contains(html, "bg-[var(--color-surface)]") {
		t.Error("navbar should use bg-[var(--color-surface)]")
	}
}

func TestNavbar_HasResponsiveDisclosure(t *testing.T) {
	html := render(navbar(testPageData()))
	if !strings.Contains(html, "<details") {
		t.Error("navbar missing <details> hamburger for small screens")
	}
	if !strings.Contains(html, "sm:hidden") || !strings.Contains(html, "hidden sm:flex") {
		t.Error("navbar missing responsive show/hide classes")
	}
}

func TestBreadcrumbs_RendersTrailWithCurrent(t *testing.T) {
	trail := []config.Link{
		{Text: "Home", Href: "/"},
		{Text: "Account", Href: "/account"},
		{Text: "Security", Href: "/account/security"},
	}
	html := render(breadcrumbs(trail))
	if !strings.Contains(html, `aria-label="Breadcrumb"`) {
		t.Error("breadcrumbs missing nav aria-label")
	}
	for _, want := range []string{"Home", "Account", "Security"} {
		if !strings.Contains(html, want) {
			t.Errorf("breadcrumbs missing crumb %q", want)
		}
	}
	// First crumb is a link; last crumb is the current page (not a link).
	if !strings.Contains(html, `href="/"`) {
		t.Error("breadcrumbs missing link on non-final crumb")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("breadcrumbs missing aria-current on final crumb")
	}
	if strings.Contains(html, `href="/account/security"`) {
		t.Error("final crumb should not be a link")
	}
}
