package views

import (
	"strings"
	"testing"

	"github.com/tunedmystic/rio/ui"
)

func TestMetricCard_RendersValueAndSparkline(t *testing.T) {
	html := render(metricCard("Revenue", "$48.2k", 12.5, []int{3, 5, 4, 8, 7, 11}))
	if !strings.Contains(html, "$48.2k") {
		t.Error("metricCard missing value")
	}
	if !strings.Contains(html, "<svg") || !strings.Contains(html, "<polyline") {
		t.Error("metricCard missing SVG sparkline")
	}
	if !strings.Contains(html, "12.5") {
		t.Error("metricCard missing delta percentage")
	}
}

func TestMetricCard_NegativeDeltaUsesDownArrow(t *testing.T) {
	html := render(metricCard("Churn", "1.2%", -3.0, []int{9, 7, 6, 4}))
	if !strings.Contains(html, "▼") {
		t.Error("negative delta should render a down arrow")
	}
}

func TestDataTable_RendersBadgesAndPagination(t *testing.T) {
	rows := []tableRow{
		{Cells: []string{"INV-001", "Acme"}, Status: "Paid", Variant: ui.BadgeSuccess},
		{Cells: []string{"INV-002", "Globex"}, Status: "Overdue", Variant: ui.BadgeDanger},
	}
	html := render(dataTable([]string{"Invoice", "Customer", "Status", ""}, rows, "1–10 of 240", 5, 20, "#"))
	if !strings.Contains(html, "Paid") || !strings.Contains(html, "Overdue") {
		t.Error("dataTable missing status badges")
	}
	if !strings.Contains(html, "1–10 of 240") {
		t.Error("dataTable missing pagination range label")
	}
	if !strings.Contains(html, "<details") {
		t.Error("dataTable missing row-action <details> menu")
	}
	if !strings.Contains(html, "Prev") || !strings.Contains(html, "Next") {
		t.Error("dataTable missing Prev/Next controls")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("dataTable footer missing numbered pagination")
	}
}

func TestBarChart_RendersRects(t *testing.T) {
	html := render(barChart([]int{4, 8, 6, 10, 3}))
	if !strings.Contains(html, "<svg") || !strings.Contains(html, "<rect") {
		t.Error("barChart missing SVG bars")
	}
}

func TestUsageMeter_RendersValueText(t *testing.T) {
	html := render(usageMeter("Storage", 18, 50))
	if !strings.Contains(html, "18") || !strings.Contains(html, "50") {
		t.Error("usageMeter missing used/limit text")
	}
}

func TestEmptyState_RendersTitleAndCTA(t *testing.T) {
	html := render(emptyState("layers", "No projects yet", "Create your first project to get started.", nil))
	if !strings.Contains(html, "No projects yet") {
		t.Error("emptyState missing title")
	}
}

func TestPageWindow_CollapsesLongRanges(t *testing.T) {
	// current=5 of 20 → 1, ellipsis(0), 4, 5, 6, ellipsis(0), 20
	got := pageWindow(5, 20)
	want := []int{1, 0, 4, 5, 6, 0, 20}
	if len(got) != len(want) {
		t.Fatalf("pageWindow(5,20) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pageWindow(5,20) = %v, want %v", got, want)
		}
	}
}

func TestPageWindow_ShortRangeNoEllipsis(t *testing.T) {
	got := pageWindow(1, 3)
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("pageWindow(1,3) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pageWindow(1,3) = %v, want %v", got, want)
		}
	}
}

func TestPagination_MarksCurrentAndDisablesPrevAtStart(t *testing.T) {
	html := render(pagination(1, 5, "/list"))
	if !strings.Contains(html, "Prev") || !strings.Contains(html, "Next") {
		t.Error("pagination missing Prev/Next controls")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("pagination missing aria-current on the active page")
	}
	if !strings.Contains(html, `href="/list?page=2"`) {
		t.Error("pagination missing numbered page link")
	}
	// At page 1, Prev is disabled: rendered as a non-link with aria-disabled.
	if !strings.Contains(html, `aria-disabled="true"`) {
		t.Error("pagination should disable Prev at the first page")
	}
	if strings.Contains(html, `href="/list?page=0"`) {
		t.Error("pagination should not link to page 0")
	}
}
