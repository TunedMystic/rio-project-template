// views/account_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func TestProfile_RendersTabsAndForm(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "profile", CSRF: "csrf-token", Flash: "Saved"}
	var b bytes.Buffer
	_ = Profile(pd, config.Meta{Title: "Profile"}, av, "Sam", "sam@example.com").Render(&b)
	html := b.String()
	for _, want := range []string{
		`href="/account"`,          // profile tab
		`href="/account/security"`, // security tab
		`href="/account/billing"`,  // billing tab
		`href="/account/delete"`,   // danger tab
		`value="csrf-token"`,       // hidden CSRF
		`value="Sam"`,              // editable name
		"sam@example.com",          // email shown
		"Saved",                    // flash
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Profile missing %q", want)
		}
	}
}

func TestSecurity_ListsSessions(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "security", CSRF: "c"}
	sessions := []database.Session{{ID: "cur", IP: "1.1.1.1"}, {ID: "other", IP: "2.2.2.2"}}
	var b bytes.Buffer
	_ = Security(pd, config.Meta{Title: "Security"}, av, sessions, "cur").Render(&b)
	html := b.String()
	if !strings.Contains(html, "This device") {
		t.Error("current session not marked")
	}
	if !strings.Contains(html, `action="/account/sessions/revoke-all"`) {
		t.Error("missing sign-out-everywhere form")
	}
}
