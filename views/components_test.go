package views

import (
	"strings"
	"testing"
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
