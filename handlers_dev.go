package main

import (
	"io"
	"net/http"

	"app/views"

	"github.com/tunedmystic/rio"
)

// devEmail is one previewable email: a slug, a display title, and a render func
// that produces (subject, html, text) from the email context + baked sample data.
type devEmail struct {
	Name   string
	Title  string
	Render func(ec views.EmailContext) (subject, html, text string)
}

// devEmails is the catalog of previewable emails, with sample data baked in.
func devEmails() []devEmail {
	sampleLink := Conf.BaseURL + "/auth/verify?token=SAMPLE_TOKEN"
	return []devEmail{
		{Name: "login", Title: "Login magic link", Render: func(ec views.EmailContext) (string, string, string) {
			return views.LoginEmail(ec, sampleLink)
		}},
		{Name: "notification", Title: "Generic notification", Render: func(ec views.EmailContext) (string, string, string) {
			return views.NotificationEmail(ec, "Your report is ready",
				"Your weekly report has finished generating and is ready to view.",
				"View report", Conf.BaseURL+"/account")
		}},
	}
}

// HandleDevEmails lists the previewable emails (dev only).
func HandleDevEmails() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		ec := emailContext()
		items := make([]views.EmailPreviewLink, 0)
		for _, de := range devEmails() {
			subject, _, _ := de.Render(ec)
			items = append(items, views.EmailPreviewLink{Name: de.Name, Title: de.Title, Subject: subject})
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Email previews")
		return render(w, http.StatusOK, views.DevEmailsIndex(Conf.PageDataFor(account(r)), meta, items))
	}
	return rio.MakeHandler(fn)
}

// HandleDevEmailPreview renders one email's HTML (or ?format=text) with sample
// data (dev only).
func HandleDevEmailPreview() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		name := r.PathValue("name")
		for _, de := range devEmails() {
			if de.Name != name {
				continue
			}
			_, html, text := de.Render(emailContext())
			if r.URL.Query().Get("format") == "text" {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = io.WriteString(w, text)
				return nil
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, html)
			return nil
		}
		http.NotFound(w, r)
		return nil
	}
	return rio.MakeHandler(fn)
}
