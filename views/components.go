package views

import (
	"fmt"
	"strings"

	"app/config"

	"github.com/tunedmystic/rio/dom"
)

// navbar is the top bar: a monogram + site name on the left, links on the
// right. Links stay quiet (muted) until hovered, when they pick up the accent.
func navbar(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.HeaderLinks)+2)
	links = append(links, dom.Class("flex items-center gap-6"))
	for _, l := range pd.HeaderLinks {
		links = append(links, navLink(l))
	}
	if pd.Account.LoggedIn {
		links = append(links, accountMenu(pd.Account))
	} else {
		links = append(links, navLink(config.Link{Text: "Log in", Href: "/login"}))
	}
	return dom.Header(
		dom.Class("border-b border-[var(--color-border)] bg-[#f8f5ee]"),
		dom.Div(
			dom.Class("mx-auto flex w-full max-w-5xl items-center justify-between px-5 py-4"),
			brand(pd),
			dom.Nav(links...),
		),
	)
}

// accountMenu is the monogram avatar that opens a no-JS dropdown (native
// <details>) with account links and Log out.
func accountMenu(a config.Account) dom.Node {
	label := a.Email
	if a.Name != "" {
		label = a.Name
	}
	primary := "Account"
	if a.Name != "" {
		primary = a.Name
	}
	return dom.Details(
		dom.Class("relative"),
		// The summary IS the avatar; list-none + marker hide the disclosure triangle.
		dom.Summary(
			dom.Class("flex h-8 w-8 cursor-pointer list-none items-center justify-center rounded-full bg-[var(--color-primary)] text-[var(--color-on-primary)] text-[length:var(--font-size-sm)] font-bold marker:hidden [&::-webkit-details-marker]:hidden"),
			dom.Title(label),
			dom.Text(initial(label)),
		),
		dom.Div(
			dom.Class("absolute right-0 z-20 mt-2 w-56 overflow-hidden rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] py-1 shadow-lg"),
			dom.Div(
				dom.Class("border-b border-[var(--color-border)] px-4 py-3"),
				dom.P(dom.Class("truncate text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text)]"), dom.Text(primary)),
				dom.P(dom.Class("truncate text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(a.Email)),
			),
			menuItem("/account", "Account settings"),
			menuItem("/account/billing", "Billing"),
			dom.Hr(dom.Class("my-1 border-[var(--color-border)]")),
			menuItem("/logout", "Log out"),
		),
	)
}

// menuItem is one link row inside the account dropdown.
func menuItem(href, label string) dom.Node {
	return dom.A(
		dom.Class("block px-4 py-2 text-[length:var(--font-size-sm)] text-[var(--color-text)] transition-colors hover:bg-[var(--color-primary)]/8 hover:text-[var(--color-primary)]"),
		dom.Href(href),
		dom.Text(label),
	)
}

// brand is the monogram tile + wordmark, linking home.
func brand(pd config.PageData) dom.Node {
	return dom.A(
		dom.Class("flex items-center gap-2.5"),
		dom.Href("/"),
		dom.Div(
			dom.Class("flex h-8 w-8 items-center justify-center rounded-[var(--radius-base)] bg-[var(--color-primary)] text-[var(--color-on-primary)] text-[length:var(--font-size-sm)] font-bold"),
			dom.Text(initial(pd.SiteName)),
		),
		dom.Span(
			dom.Class("font-semibold tracking-tight text-[var(--color-text)]"),
			dom.Text(pd.SiteName),
		),
	)
}

func navLink(l config.Link) dom.Node {
	return dom.A(
		dom.Class("text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-primary)]"),
		dom.Href(l.Href),
		dom.Text(l.Text),
	)
}

// footer is centered: dot-separated muted links above a quiet maker's line.
func footer(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.FooterLinks)*2)
	for i, l := range pd.FooterLinks {
		if i > 0 {
			links = append(links, dom.Span(dom.Class("text-[var(--color-border)]"), dom.Text("•")))
		}
		links = append(links, footerLink(l))
	}

	return dom.Footer(
		dom.Class("mt-16 border-t border-[var(--color-border)] py-10"),
		dom.Div(
			dom.Class("mx-auto flex w-full max-w-5xl flex-col items-center gap-4 px-5 text-center"),
			dom.Nav(withClass("flex flex-wrap items-center justify-center gap-x-4 gap-y-2", links)...),
			dom.P(
				dom.Class("flex items-center justify-center gap-1.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
				dom.Text("Made with"),
				dom.Span(dom.Class("text-[var(--color-danger)]"), icon("heart", 15)),
				dom.Text("using "+pd.SiteName),
			),
		),
	)
}

func footerLink(l config.Link) dom.Node {
	return dom.A(
		dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)] underline-offset-4 transition-colors hover:text-[var(--color-text)] hover:underline"),
		dom.Href(l.Href),
		dom.Text(l.Text),
	)
}

// pageHeader is the band at the top of a page: a bold title and a one-line
// muted subtitle, separated from the content by a hairline.
func pageHeader(title, subtitle string) dom.Node {
	return dom.Header(
		dom.Class("border-b border-[var(--color-border)] py-12"),
		shell(
			dom.H1(
				dom.Class("text-[length:var(--font-size-2xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
				dom.Text(title),
			),
			dom.P(
				dom.Class("mt-2 text-[var(--color-text-muted)]"),
				dom.Text(subtitle),
			),
		),
	)
}

// ruledHeading is a section title underlined by a soft accent rule — gives a
// card or content group a crisp, labeled top.
func ruledHeading(title string) dom.Node {
	return dom.Div(
		dom.Class("border-b border-[var(--color-primary)]/30 pb-3"),
		dom.H2(
			dom.Class("text-[length:var(--font-size-xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
			dom.Text(title),
		),
	)
}

// featureRow is the signature element: a soft accent icon tile, a title and
// muted description, and a chevron — the whole row is a single link that lifts
// on hover.
func featureRow(iconName, title, desc, href string) dom.Node {
	return dom.A(
		dom.Class("group block"),
		dom.Href(href),
		dom.Div(
			dom.Class("flex items-center gap-4 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm transition duration-200 hover:-translate-y-0.5 hover:shadow-md"),
			dom.Div(
				dom.Class("flex h-11 w-11 shrink-0 items-center justify-center rounded-[var(--radius-base)] bg-[var(--color-primary)]/10 text-[var(--color-primary)]"),
				icon(iconName, 22),
			),
			dom.Div(
				dom.Class("min-w-0 flex-1"),
				dom.Div(
					dom.Class("font-semibold tracking-tight text-[var(--color-text)]"),
					dom.Text(title),
				),
				dom.Div(
					dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
					dom.Text(desc),
				),
			),
			dom.Div(
				dom.Class("shrink-0 text-[var(--color-text-muted)] transition-all duration-200 group-hover:translate-x-0.5 group-hover:text-[var(--color-primary)]"),
				icon("chevron-right", 20),
			),
		),
	)
}

// deviceLabel turns a raw User-Agent string into a friendly "Browser · OS"
// label for the active-sessions list, falling back to a generic description
// when it can't be recognized.
func deviceLabel(ua string) string {
	if strings.TrimSpace(ua) == "" {
		return "Unknown device"
	}
	browser := "Browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	os := ""
	switch {
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}
	if os != "" {
		return browser + " · " + os
	}
	return browser
}

// submitButton renders a submit button styled like a ui primary button.
// ui.Button hardcodes type="button"; if submit buttons recur across products,
// promote a submit/type option into rio/ui (rule of three).
func submitButton(label string) dom.Node {
	return dom.Button(
		dom.Type("submit"),
		dom.Class("inline-flex items-center justify-center gap-2 rounded-[var(--radius-base)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold tracking-tight bg-[var(--color-primary)] text-[var(--color-on-primary)] shadow-sm transition hover:shadow-md hover:brightness-105 active:brightness-95 cursor-pointer"),
		dom.Text(label),
	)
}

// initial returns the first letter of s, uppercased, for the monogram.
func initial(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "·"
	}
	return strings.ToUpper(s[:1])
}

// icon returns an inline lucide-style SVG sized to size px, colored via
// currentColor so the parent's text color controls it.
func icon(name string, size int) dom.Node {
	const (
		head  = `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">`
		heart = `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 24 24" fill="currentColor" stroke="none" aria-hidden="true">`
	)

	var body string
	switch name {
	case "message":
		body = `<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>`
	case "layers":
		body = `<path d="M12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 12.18-9.17 4.16a2 2 0 0 1-1.66 0L2 12.18"/><path d="m22 17.18-9.17 4.16a2 2 0 0 1-1.66 0L2 17.18"/>`
	case "database":
		body = `<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/>`
	case "arrow-right":
		body = `<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>`
	case "chevron-right":
		body = `<path d="m9 18 6-6-6-6"/>`
	case "check":
		body = `<path d="M20 6 9 17l-5-5"/>`
	case "heart":
		return dom.Raw(fmt.Sprintf(heart, size, size) + `<path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>`)
	default:
		body = `<circle cx="12" cy="12" r="9"/>`
	}
	return dom.Raw(fmt.Sprintf(head, size, size) + body + `</svg>`)
}
