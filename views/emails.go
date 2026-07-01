package views

import (
	"bytes"

	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// EmailContext is the branding an email needs, built from the app config.
type EmailContext struct {
	SiteName string
	Tokens   ui.Tokens
}

// renderString renders a dom node tree to a string.
func renderString(n dom.Node) string {
	var b bytes.Buffer
	_ = n.Render(&b)
	return b.String()
}

// emailDoc renders the layout node and prepends an HTML5 doctype.
func emailDoc(n dom.Node) string {
	return "<!doctype html>\n" + renderString(n)
}

// emailLayout builds the email HTML document: a centered card with a brand
// header, the inner content, and a muted footer. All styling is inline and
// table-based for email-client compatibility.
func emailLayout(ec EmailContext, inner ...dom.Node) dom.Node {
	t := ec.Tokens

	header := dom.Tr(dom.Td(
		dom.Style("padding:24px 28px;border-bottom:1px solid "+t.ColorBorder),
		dom.Span(dom.Style("font-weight:700;font-size:20px;color:"+t.ColorPrimary+";font-family:"+t.FontFamily), dom.Text(ec.SiteName)),
	))

	bodyTd := dom.Td(append([]dom.Node{dom.Style("padding:28px")}, inner...)...)

	footer := dom.Tr(dom.Td(
		dom.Style("padding:20px 28px;border-top:1px solid "+t.ColorBorder+";color:"+t.ColorTextMuted+";font-size:13px;font-family:"+t.FontFamily),
		dom.Text("Sent by "+ec.SiteName),
	))

	emailCard := dom.Table(
		dom.Role("presentation"), dom.Width("600"),
		dom.CreateAttr("cellpadding", "0"), dom.CreateAttr("cellspacing", "0"), dom.CreateAttr("border", "0"),
		dom.Style("max-width:600px;width:100%;background:"+t.ColorSurface+";border:1px solid "+t.ColorBorder+";border-radius:"+t.RadiusBase),
		header, dom.Tr(bodyTd), footer,
	)

	outer := dom.Table(
		dom.Role("presentation"), dom.Width("100%"),
		dom.CreateAttr("cellpadding", "0"), dom.CreateAttr("cellspacing", "0"), dom.CreateAttr("border", "0"),
		dom.Style("background:"+t.ColorBackground),
		dom.Tr(dom.Td(dom.CreateAttr("align", "center"), dom.Style("padding:24px"), emailCard)),
	)

	return dom.Html(
		dom.Head(
			dom.Meta(dom.Charset("utf-8")),
			dom.Meta(dom.Name("viewport"), dom.Content("width=device-width,initial-scale=1")),
			dom.TitleEl(dom.Text(ec.SiteName)),
		),
		dom.Body(dom.Style("margin:0;padding:0;background:"+t.ColorBackground+";font-family:"+t.FontFamily), outer),
	)
}

func emailHeading(ec EmailContext, text string) dom.Node {
	return dom.H1(
		dom.Style("margin:0 0 16px;font-size:24px;font-weight:700;color:"+ec.Tokens.ColorText+";font-family:"+ec.Tokens.FontFamily),
		dom.Text(text),
	)
}

func emailParagraph(ec EmailContext, text string) dom.Node {
	return dom.P(
		dom.Style("margin:0 0 16px;font-size:16px;line-height:1.6;color:"+ec.Tokens.ColorText+";font-family:"+ec.Tokens.FontFamily),
		dom.Text(text),
	)
}

func emailButton(ec EmailContext, href, label string) dom.Node {
	t := ec.Tokens
	return dom.P(
		dom.Style("margin:24px 0"),
		dom.A(
			dom.Href(href),
			dom.Style("display:inline-block;padding:13px 22px;background:"+t.ColorPrimary+";color:"+t.OnPrimary+";text-decoration:none;font-weight:600;font-size:16px;border-radius:"+t.RadiusBase+";font-family:"+t.FontFamily),
			dom.Text(label),
		),
	)
}

// LoginEmail is the magic-link sign-in email.
func LoginEmail(ec EmailContext, link string) (subject, html, text string) {
	subject = "Your login link"
	doc := emailLayout(ec,
		emailHeading(ec, "Sign in to "+ec.SiteName),
		emailParagraph(ec, "Click the button below to sign in. This link expires in 15 minutes."),
		emailButton(ec, link, "Log in"),
		emailParagraph(ec, "If the button doesn't work, copy and paste this link:"),
		dom.P(dom.Style("margin:0;font-size:14px;word-break:break-all;color:"+ec.Tokens.ColorTextMuted+";font-family:"+ec.Tokens.FontFamily), dom.Text(link)),
	)
	html = emailDoc(doc)
	text = "Sign in to " + ec.SiteName + "\n\nClick to log in (expires in 15 minutes):\n\n" + link + "\n"
	return subject, html, text
}

// NotificationEmail is the generic transactional template: a heading, a body
// paragraph, and an optional CTA button (empty ctaLabel -> no button). Copy this
// as the starting point for new transactional emails.
func NotificationEmail(ec EmailContext, heading, body, ctaLabel, ctaHref string) (subject, html, text string) {
	subject = heading
	inner := []dom.Node{
		emailHeading(ec, heading),
		emailParagraph(ec, body),
	}
	if ctaLabel != "" {
		inner = append(inner, emailButton(ec, ctaHref, ctaLabel))
	}
	html = emailDoc(emailLayout(ec, inner...))
	text = heading + "\n\n" + body + "\n"
	if ctaLabel != "" {
		text += "\n" + ctaLabel + ": " + ctaHref + "\n"
	}
	return subject, html, text
}

// EmailPreviewLink is one row in the dev email index.
type EmailPreviewLink struct {
	Name    string
	Title   string
	Subject string
}

// DevEmailsIndex renders the dev-only list of previewable emails (app chrome).
func DevEmailsIndex(pd config.PageData, meta config.Meta, items []EmailPreviewLink) dom.Node {
	rows := make([]dom.Node, 0, len(items))
	for _, it := range items {
		rows = append(rows, card(
			dom.Div(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(it.Title)),
			dom.Div(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Subject: "+it.Subject)),
			dom.Div(
				dom.Class("mt-3 flex gap-4 text-[length:var(--font-size-sm)]"),
				dom.A(dom.Class("text-[var(--color-primary)] hover:underline"), dom.Href("/dev/emails/"+it.Name), dom.Text("HTML preview")),
				dom.A(dom.Class("text-[var(--color-primary)] hover:underline"), dom.Href("/dev/emails/"+it.Name+"?format=text"), dom.Text("Text version")),
			),
		))
	}
	return Page(pd, meta,
		pageHeader("Email previews", "Dev-only: how each email renders with sample data."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("max-w-2xl space-y-4", rows)...))),
	)
}
