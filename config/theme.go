package config

import "github.com/tunedmystic/rio/ui"

// Theme selects a compile-time visual preset. config.New sets one (default
// ThemeSlateIndigo); it flows through the token -> CSS-variable pipeline.
type Theme int

const (
	ThemeSlateIndigo Theme = iota // light, default
	ThemeDusk                     // dark
)

// ThemeVar is a single extended CSS custom property the data components need
// beyond the ui.Tokens set. The layout emits these into :root.
type ThemeVar struct {
	Name  string
	Value string
}

// Tokens returns the ui.Tokens for the preset.
func (t Theme) Tokens() ui.Tokens {
	switch t {
	case ThemeDusk:
		return themeDusk()
	default:
		return themeSlateIndigo()
	}
}

// Vars returns the extended CSS variables for the preset, in render order.
func (t Theme) Vars() []ThemeVar {
	switch t {
	case ThemeDusk:
		return []ThemeVar{
			{"--color-surface-raised", "#1e293b"},
			{"--color-ring", "#22d3ee"},
			{"--color-on-danger", "#0b1020"},
			{"--chart-1", "#155e75"},
			{"--chart-2", "#0891b2"},
			{"--chart-3", "#22d3ee"},
			{"--chart-4", "#67e8f9"},
		}
	default:
		return []ThemeVar{
			{"--color-surface-raised", "#f1f5f9"},
			{"--color-ring", "#6366f1"},
			{"--color-on-danger", "#ffffff"},
			{"--chart-1", "#c7d2fe"},
			{"--chart-2", "#a5b4fc"},
			{"--chart-3", "#6366f1"},
			{"--chart-4", "#4f46e5"},
		}
	}
}

// themeSlateIndigo is the light default: indigo accent on slate neutrals.
func themeSlateIndigo() ui.Tokens {
	return ui.Tokens{
		FontFamily:        `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:        "16px",
		FontSizeBase:      "18px",
		FontSizeLg:        "20px",
		FontSizeXl:        "24px",
		FontSize2xl:       "30px",
		ColorPrimary:      "#4f46e5",
		OnPrimary:         "#ffffff",
		ColorSecondary:    "#475569",
		OnSecondary:       "#ffffff",
		ColorBackground:   "#f8fafc",
		ColorSurface:      "#ffffff",
		ColorText:         "#0f172a",
		ColorTextMuted:    "#64748b",
		ColorBorder:       "#e2e8f0",
		ColorSuccess:      "#16a34a",
		ColorWarning:      "#b45309",
		ColorDanger:       "#dc2626",
		ColorInfo:         "#2563eb",
		RadiusBase:        "0.625rem",
		FontWeightHeading: "800",
	}
}

// themeDusk is the dark preset: cyan accent on deep navy-slate surfaces.
func themeDusk() ui.Tokens {
	return ui.Tokens{
		FontFamily:        `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:        "16px",
		FontSizeBase:      "18px",
		FontSizeLg:        "20px",
		FontSizeXl:        "24px",
		FontSize2xl:       "30px",
		ColorPrimary:      "#22d3ee",
		OnPrimary:         "#06283d",
		ColorSecondary:    "#818cf8",
		OnSecondary:       "#0b1020",
		ColorBackground:   "#0b1020",
		ColorSurface:      "#0f172a",
		ColorText:         "#f1f5f9",
		ColorTextMuted:    "#94a3b8",
		ColorBorder:       "#1e293b",
		ColorSuccess:      "#34d399",
		ColorWarning:      "#fbbf24",
		ColorDanger:       "#f87171",
		ColorInfo:         "#38bdf8",
		RadiusBase:        "0.625rem",
		FontWeightHeading: "800",
	}
}
