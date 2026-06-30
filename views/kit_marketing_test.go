package views

import (
	"strings"
	"testing"

	"github.com/tunedmystic/rio/dom"
)

func TestHero_RendersHeadlineAndCTAs(t *testing.T) {
	html := render(hero("New", "Ship faster", "A starter that scales.",
		dom.Text("Get started"), dom.Text("Learn more"), svgPanel()))
	for _, want := range []string{"Ship faster", "A starter that scales.", "Get started", "Learn more"} {
		if !strings.Contains(html, want) {
			t.Errorf("hero missing %q", want)
		}
	}
}

func TestFeatureGrid_RendersAllItems(t *testing.T) {
	items := []featureItem{
		{Icon: "layers", Title: "Composable", Blurb: "Build from parts."},
		{Icon: "message", Title: "Realtime", Blurb: "Always fresh."},
	}
	html := render(featureGrid(items))
	for _, want := range []string{"Composable", "Realtime", "Build from parts.", "Always fresh."} {
		if !strings.Contains(html, want) {
			t.Errorf("featureGrid missing %q", want)
		}
	}
}

func TestPricingTable_RendersPlansAndHighlight(t *testing.T) {
	plans := []plan{
		{Name: "Starter", Price: "$0", Period: "/mo", Features: []string{"1 project"}, CTA: dom.Text("Choose")},
		{Name: "Pro", Price: "$29", Period: "/mo", Features: []string{"Unlimited"}, Highlighted: true, CTA: dom.Text("Choose")},
	}
	html := render(pricingTable(plans))
	for _, want := range []string{"Starter", "Pro", "$0", "$29", "1 project", "Unlimited"} {
		if !strings.Contains(html, want) {
			t.Errorf("pricingTable missing %q", want)
		}
	}
	if !strings.Contains(html, "Popular") {
		t.Error("pricingTable highlighted plan missing 'Popular' marker")
	}
}

func TestFaq_RendersDetails(t *testing.T) {
	html := render(faq([]faqItem{{Q: "Is it free?", A: "Yes, to start."}}))
	if !strings.Contains(html, "<details") || !strings.Contains(html, "<summary") {
		t.Error("faq should render <details>/<summary>")
	}
	if !strings.Contains(html, "Is it free?") || !strings.Contains(html, "Yes, to start.") {
		t.Error("faq missing question/answer text")
	}
}

func TestLogoCloud_RendersLabels(t *testing.T) {
	html := render(logoCloud([]string{"Acme", "Globex"}))
	if !strings.Contains(html, "Acme") || !strings.Contains(html, "Globex") {
		t.Error("logoCloud missing labels")
	}
}

func TestFeatureHighlight_RendersTitle(t *testing.T) {
	html := render(featureHighlight("Speed", "Instant feedback", "See changes live.", true))
	if !strings.Contains(html, "Instant feedback") || !strings.Contains(html, "See changes live.") {
		t.Error("featureHighlight missing content")
	}
}

func TestCtaBand_RendersTitleAndCTA(t *testing.T) {
	html := render(ctaBand("Ready?", "Start in minutes.", "git clone you/app", dom.Text("Sign up")))
	for _, want := range []string{"Ready?", "Start in minutes.", "git clone you/app", "Sign up"} {
		if !strings.Contains(html, want) {
			t.Errorf("ctaBand missing %q", want)
		}
	}
}

func TestTestimonial_RendersQuoteAndAuthor(t *testing.T) {
	html := render(testimonial("It just works.", "Ada", "CTO, Acme"))
	for _, want := range []string{"It just works.", "Ada", "CTO, Acme"} {
		if !strings.Contains(html, want) {
			t.Errorf("testimonial missing %q", want)
		}
	}
}
