package views

import (
	"strings"
	"testing"
)

func TestFormShowcase_RendersFields(t *testing.T) {
	html := render(formShowcase())
	for _, want := range []string{"<form", "<input", "<textarea", "<select"} {
		if !strings.Contains(html, want) {
			t.Errorf("formShowcase missing %q", want)
		}
	}
}

func TestToggle_RendersCheckbox(t *testing.T) {
	html := render(toggle("notify", "Email notifications", true))
	if !strings.Contains(html, `type="checkbox"`) {
		t.Error("toggle should be a styled checkbox")
	}
	if !strings.Contains(html, "Email notifications") {
		t.Error("toggle missing label")
	}
	if !strings.Contains(html, "checked") {
		t.Error("toggle with on=true should be checked")
	}
}

func TestTabStrip_MarksActive(t *testing.T) {
	html := render(tabStrip([]tabItem{{"overview", "Overview"}, {"activity", "Activity"}}, "activity"))
	for _, want := range []string{"Overview", "Activity"} {
		if !strings.Contains(html, want) {
			t.Errorf("tabStrip missing %q", want)
		}
	}
	if !strings.Contains(html, "border-[var(--color-primary)]") {
		t.Error("tabStrip active tab should carry the primary underline")
	}
}
