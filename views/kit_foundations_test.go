package views

import (
	"strings"
	"testing"
)

func TestColorSwatches_RendersTokenVars(t *testing.T) {
	html := render(colorSwatches())
	for _, want := range []string{"--color-primary", "--color-surface-raised", "--color-border"} {
		if !strings.Contains(html, want) {
			t.Errorf("colorSwatches missing swatch for %s", want)
		}
	}
}

func TestTypeScale_RendersSpecimens(t *testing.T) {
	html := render(typeScale())
	for _, want := range []string{"font-size-2xl", "font-size-base", "font-size-sm"} {
		if !strings.Contains(html, want) {
			t.Errorf("typeScale missing %s", want)
		}
	}
}

func TestButtonSet_RendersAllVariants(t *testing.T) {
	html := render(buttonSet())
	for _, want := range []string{"Primary", "Secondary", "Ghost", "Danger"} {
		if !strings.Contains(html, want) {
			t.Errorf("buttonSet missing %q button", want)
		}
	}
}

func TestStatusBadges_RendersVariants(t *testing.T) {
	html := render(statusBadges())
	for _, want := range []string{"Success", "Warning", "Danger", "Neutral", "Info"} {
		if !strings.Contains(html, want) {
			t.Errorf("statusBadges missing %q", want)
		}
	}
}

func TestAvatarGroup_RendersInitials(t *testing.T) {
	html := render(avatarGroup([]string{"Ada Lovelace", "Grace Hopper"}))
	for _, want := range []string{">A<", ">G<"} {
		if !strings.Contains(html, want) {
			t.Errorf("avatarGroup missing rendered initial %q", want)
		}
	}
}
