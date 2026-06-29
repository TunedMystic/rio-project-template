package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func render(n interface{ Render(w *bytes.Buffer) error }) string {
	var b bytes.Buffer
	_ = n.Render(&b)
	return b.String()
}

func testPageData() config.PageData {
	c := config.New("debug", "v1test")
	return c.PageData()
}

func TestPage_RendersHeadAndChrome(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Hi - Rio Starter", Description: "d"}
	var b bytes.Buffer
	_ = Page(pd, meta, nil).Render(&b)
	html := b.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"<title>Hi - Rio Starter</title>",
		"<style>",                                // StyleVars block
		"--color-primary:",                       // a token variable
		`href="/static/css/styles.css?v=v1test"`, // versioned stylesheet link
		`rel="icon"`,                             // favicon link
		"</html>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Page output missing %q", want)
		}
	}
}

func TestMessages_ListsBodies(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Messages"}
	msgs := []database.Message{{ID: 1, Body: "first-msg"}, {ID: 2, Body: "second-msg"}}
	var b bytes.Buffer
	_ = Messages(pd, meta, msgs, "", "").Render(&b)
	html := b.String()

	if !strings.Contains(html, "first-msg") || !strings.Contains(html, "second-msg") {
		t.Error("Messages output missing message bodies")
	}
	if !strings.Contains(html, `action="/messages"`) {
		t.Error("Messages output missing the create form")
	}
}
