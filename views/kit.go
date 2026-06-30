package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// kitSection wraps a group under a ruled heading with vertical rhythm.
func kitSection(title string, body ...dom.Node) dom.Node {
	inner := make([]dom.Node, 0, len(body)+2)
	inner = append(inner, dom.Class("flex flex-col gap-6"), ruledHeading(title))
	inner = append(inner, body...)
	return dom.Section(dom.Class("py-10"), shell(dom.Div(inner...)))
}

// Kit renders the public component showcase under the active theme.
func Kit(pd config.PageData, meta config.Meta) dom.Node {
	tableRows := []tableRow{
		{Cells: []string{"INV-1001", "Acme Inc."}, Status: "Paid", Variant: ui.BadgeSuccess},
		{Cells: []string{"INV-1002", "Globex"}, Status: "Pending", Variant: ui.BadgeWarning},
		{Cells: []string{"INV-1003", "Initech"}, Status: "Overdue", Variant: ui.BadgeDanger},
		{Cells: []string{"INV-1004", "Umbrella"}, Status: "Draft", Variant: ui.BadgeNeutral},
	}
	features := []featureItem{
		{Icon: "layers", Title: "Composable kit", Blurb: "Build pages from focused, token-driven parts."},
		{Icon: "message", Title: "Server-rendered", Blurb: "No JS framework; fast, semantic HTML."},
		{Icon: "check", Title: "Accessible", Blurb: "WCAG AA contrast and keyboard-reachable controls."},
	}
	plans := []plan{
		{Name: "Starter", Price: "$0", Period: "/mo", Features: []string{"1 project", "Community support"}, CTA: ui.ButtonLink(ui.ButtonSecondary, "#", "Choose Starter")},
		{Name: "Pro", Price: "$29", Period: "/mo", Features: []string{"Unlimited projects", "Priority support", "Analytics"}, Highlighted: true, CTA: ui.ButtonLink(ui.ButtonPrimary, "#", "Choose Pro")},
		{Name: "Team", Price: "$99", Period: "/mo", Features: []string{"Everything in Pro", "SSO", "Audit log"}, CTA: ui.ButtonLink(ui.ButtonSecondary, "#", "Choose Team")},
	}
	faqs := []faqItem{
		{Q: "How do I switch themes?", A: "Set Theme: ThemeDusk in config.New and rebuild."},
		{Q: "Do the charts need JavaScript?", A: "No — they are pure server-rendered inline SVG."},
	}

	return Page(pd, meta,
		pageHeader("Component Kit", "Every component in the design system, under the active theme."),

		kitSection("Foundations",
			colorSwatches(),
			typeScale(),
			buttonSet(),
			statusBadges(),
			avatarGroup([]string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Edsger Dijkstra"}),
		),

		kitSection("Data & dashboard",
			dom.Div(
				dom.Class("grid gap-4 sm:grid-cols-2 lg:grid-cols-3"),
				metricCard("Revenue", "$48.2k", 12.5, []int{12, 14, 13, 18, 22, 20, 26}),
				metricCard("Active users", "3,182", 4.1, []int{30, 28, 33, 31, 35, 40, 44}),
				metricCard("Churn", "1.2%", -0.6, []int{9, 8, 8, 7, 6, 5, 5}),
			),
			dataTable([]string{"Invoice", "Customer", "Status", ""}, tableRows, "1–10 of 240"),
			dom.Div(
				dom.Class("grid gap-6 lg:grid-cols-2"),
				dom.Div(
					dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm"),
					dom.Div(dom.Class("mb-4 font-semibold text-[var(--color-text)]"), dom.Text("Weekly signups")),
					barChart([]int{8, 14, 10, 18, 12, 22, 16}),
				),
				dom.Div(
					dom.Class("flex flex-col gap-5 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm"),
					usageMeter("Storage", 18, 50),
					usageMeter("API calls", 82000, 100000),
					usageMeter("Seats", 7, 10),
				),
			),
			emptyState("layers", "No projects yet", "Create your first project to see it here.", ui.ButtonLink(ui.ButtonPrimary, "#", "New project")),
		),

		kitSection("Marketing & layout",
			hero("Design system",
				"A data-forward starter you can ship today",
				"Two themes, a full component kit, and a landing page — all server-rendered.",
				ui.ButtonLink(ui.ButtonPrimary, "#", "Get started"),
				ghostLink("#", "View on GitHub"),
				svgPanel()),
			logoCloud([]string{"Acme", "Globex", "Initech", "Umbrella", "Hooli"}),
			featureHighlight("Fast", "Instant feedback", "Edit a token and the whole app re-skins.", false),
			featureGrid(features),
			pricingTable(plans),
			testimonial("This template saved us a week of setup.", "Ada Lovelace", "CTO, Acme"),
			faq(faqs),
			ctaBand("Ready to build?", "Clone the template and ship your idea.", ui.ButtonLink(ui.ButtonPrimary, "#", "Start now")),
		),

		kitSection("Feedback & forms",
			dom.Div(
				dom.Class("flex flex-col gap-3"),
				ui.Alert(ui.AlertInfo, dom.Text("Heads up — this is an informational alert.")),
				ui.Alert(ui.AlertSuccess, dom.Text("Saved — your changes were applied.")),
				ui.Alert(ui.AlertWarning, dom.Text("Careful — your trial ends soon.")),
				ui.Alert(ui.AlertError, dom.Text("Error — something went wrong.")),
			),
			tabStrip([]tabItem{{"overview", "Overview"}, {"activity", "Activity"}, {"settings", "Settings"}}, "overview"),
			formShowcase(),
		),
	)
}
