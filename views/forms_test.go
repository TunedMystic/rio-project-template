package views

import (
	"strings"
	"testing"
)

func TestHoneypot_RendersHiddenDecoyField(t *testing.T) {
	html := render(Honeypot())
	for _, want := range []string{`name="website"`, "position:absolute", `aria-hidden="true"`, `tabindex="-1"`} {
		if !strings.Contains(html, want) {
			t.Errorf("Honeypot output missing %q", want)
		}
	}
}
