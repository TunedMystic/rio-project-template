package views

import (
	"strings"
	"testing"

	"github.com/tunedmystic/rio/ui"
)

func TestActivityFeed_RendersItems(t *testing.T) {
	items := []activityItem{
		{Icon: "check", Title: "Invoice paid", Meta: "Acme Inc. · $1,200", Time: "2h ago", Variant: ui.BadgeSuccess},
		{Icon: "message", Title: "New comment", Meta: "Ada Lovelace", Time: "5h ago", Variant: ui.BadgeNeutral},
	}
	html := render(activityFeed(items))
	for _, want := range []string{"Invoice paid", "New comment", "2h ago", "5h ago"} {
		if !strings.Contains(html, want) {
			t.Errorf("activityFeed missing %q", want)
		}
	}
	// Timeline rail + round dot markers.
	if !strings.Contains(html, "border-l") {
		t.Error("activityFeed missing timeline rail")
	}
	if !strings.Contains(html, "rounded-full") {
		t.Error("activityFeed missing round dot marker")
	}
	// Tinted-fill dot uses color-mix so the icon is legible in both themes.
	if !strings.Contains(html, "color-mix") {
		t.Error("activityFeed dot missing color-mix tinted background")
	}
	if !strings.Contains(html, "var(--color-success)") {
		t.Error("activityFeed dot missing success token color")
	}
}
