package config

import "testing"

func TestThemeTokens_BothPresetsPopulated(t *testing.T) {
	for _, th := range []Theme{ThemeSlateIndigo, ThemeDusk} {
		tk := th.Tokens()
		if tk.ColorPrimary == "" || tk.ColorBackground == "" || tk.ColorText == "" {
			t.Errorf("theme %d: tokens have empty core colors", th)
		}
		if tk.FontWeightHeading != "800" {
			t.Errorf("theme %d: FontWeightHeading = %q, want 800", th, tk.FontWeightHeading)
		}
		if tk.RadiusBase != "0.625rem" {
			t.Errorf("theme %d: RadiusBase = %q, want 0.625rem", th, tk.RadiusBase)
		}
	}
}

func TestThemeTokens_SelectionSwapsPalette(t *testing.T) {
	light := ThemeSlateIndigo.Tokens()
	dark := ThemeDusk.Tokens()
	if light.ColorPrimary == dark.ColorPrimary {
		t.Error("expected the two presets to differ in ColorPrimary")
	}
	if light.ColorPrimary != "#4f46e5" {
		t.Errorf("SlateIndigo ColorPrimary = %q, want #4f46e5", light.ColorPrimary)
	}
	if dark.ColorPrimary != "#22d3ee" {
		t.Errorf("Dusk ColorPrimary = %q, want #22d3ee", dark.ColorPrimary)
	}
}

func TestThemeVars_IncludeExtendedNames(t *testing.T) {
	for _, th := range []Theme{ThemeSlateIndigo, ThemeDusk} {
		got := map[string]string{}
		for _, v := range th.Vars() {
			got[v.Name] = v.Value
		}
		for _, name := range []string{
			"--color-surface-raised", "--color-ring", "--color-on-danger",
			"--chart-1", "--chart-2", "--chart-3", "--chart-4",
		} {
			if got[name] == "" {
				t.Errorf("theme %d: missing extended var %s", th, name)
			}
		}
	}
}
