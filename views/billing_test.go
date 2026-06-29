package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestPremiumAndGuide_Render(t *testing.T) {
	pd := testPageData()
	var b bytes.Buffer
	_ = Premium(pd, config.Meta{Title: "Premium"}).Render(&b)
	if !strings.Contains(b.String(), "Pro members") {
		t.Error("Premium page missing expected copy")
	}
	var b2 bytes.Buffer
	_ = Guide(pd, config.Meta{Title: "Guide"}).Render(&b2)
	if !strings.Contains(b2.String(), "guide") {
		t.Error("Guide page missing expected copy")
	}
}
