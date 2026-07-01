package views

import (
	"strings"
	"testing"

	"app/config"

	"github.com/tunedmystic/rio/ui"
)

func testEmailContext() EmailContext {
	return EmailContext{
		SiteName: "Testco",
		Tokens: ui.Tokens{
			ColorPrimary:    "#4f46e5",
			OnPrimary:       "#ffffff",
			ColorText:       "#0f172a",
			ColorTextMuted:  "#64748b",
			ColorSurface:    "#ffffff",
			ColorBackground: "#f4f5f7",
			ColorBorder:     "#e2e8f0",
			RadiusBase:      "0.5rem",
		},
	}
}

func TestLoginEmail(t *testing.T) {
	subject, html, text := LoginEmail(testEmailContext(), "https://x/auth/verify?token=ABC")
	if subject == "" {
		t.Error("empty subject")
	}
	for _, want := range []string{"<!doctype html>", "Testco", "https://x/auth/verify?token=ABC", "#4f46e5", "style="} {
		if !strings.Contains(html, want) {
			t.Errorf("login html missing %q", want)
		}
	}
	if strings.Contains(html, `class="`) {
		t.Error("email html must not use CSS classes (inline styles only)")
	}
	if !strings.Contains(text, "https://x/auth/verify?token=ABC") {
		t.Error("login text missing link")
	}
}

func TestNotificationEmail_WithCTA(t *testing.T) {
	subject, html, text := NotificationEmail(testEmailContext(), "Heads up", "Something happened.", "View", "https://x/view")
	if subject != "Heads up" {
		t.Errorf("subject = %q, want Heads up", subject)
	}
	for _, want := range []string{"Heads up", "Something happened.", "https://x/view"} {
		if !strings.Contains(html, want) {
			t.Errorf("notification html missing %q", want)
		}
	}
	if !strings.Contains(text, "View: https://x/view") {
		t.Error("notification text missing CTA")
	}
}

func TestNotificationEmail_NoCTA(t *testing.T) {
	_, html, _ := NotificationEmail(testEmailContext(), "Hi", "No button here.", "", "")
	if strings.Contains(html, "text-decoration:none") {
		t.Error("should not render a CTA button when ctaLabel is empty")
	}
}

func TestDevEmailsIndex(t *testing.T) {
	items := []EmailPreviewLink{{Name: "login", Title: "Login", Subject: "Your login link"}}
	html := render(DevEmailsIndex(testPageData(), config.Meta{Title: "Emails"}, items))
	if !strings.Contains(html, `/dev/emails/login`) || !strings.Contains(html, "Login") {
		t.Error("index missing preview link")
	}
}
