// views/auth_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestLogin_RendersForm(t *testing.T) {
	pd := testPageData()
	var b bytes.Buffer
	_ = Login(pd, config.Meta{Title: "Log in"}, "you@example.com", "bad email", "/account/security").Render(&b)
	html := b.String()
	for _, want := range []string{
		`action="/login"`,
		`name="email"`,
		`value="you@example.com"`, // preserves entered email
		"bad email",               // shows the error
		`name="next"`,             // carries next through
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Login missing %q", want)
		}
	}
}

func TestNav_ShowsLoginWhenAnon(t *testing.T) {
	pd := testPageData() // Account.LoggedIn == false
	var b bytes.Buffer
	_ = Page(pd, config.Meta{Title: "x"}, nil).Render(&b)
	if !strings.Contains(b.String(), `href="/login"`) {
		t.Error("anon nav should show a login link")
	}
}

func TestNav_ShowsAccountWhenLoggedIn(t *testing.T) {
	c := config.New("debug", "h")
	pd := c.PageDataFor(config.Account{LoggedIn: true, Name: "Sam", Email: "sam@example.com"})
	var b bytes.Buffer
	_ = Page(pd, config.Meta{Title: "x"}, nil).Render(&b)
	html := b.String()
	if !strings.Contains(html, `href="/account"`) {
		t.Error("logged-in nav should link to /account")
	}
	if !strings.Contains(html, `href="/logout"`) {
		t.Error("logged-in nav account menu should offer Log out")
	}
}

func TestLogin_ShowsGoogleWhenEnabled(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "x")
	t.Setenv("GOOGLE_CLIENT_SECRET", "y")
	pd := config.New("debug", "h").PageData()
	var b bytes.Buffer
	_ = Login(pd, config.Meta{Title: "Log in"}, "", "", "/account").Render(&b)
	if !strings.Contains(b.String(), `href="/auth/google/login"`) {
		t.Error("expected Google login button when enabled")
	}
}

func TestLogin_HidesGoogleWhenDisabled(t *testing.T) {
	pd := testPageData() // GoogleEnabled == false
	var b bytes.Buffer
	_ = Login(pd, config.Meta{Title: "Log in"}, "", "", "/account").Render(&b)
	if strings.Contains(b.String(), `/auth/google/login`) {
		t.Error("Google button should be hidden when disabled")
	}
}
