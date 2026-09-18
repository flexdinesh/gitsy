package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Design tokens. Single source for spacing, sizing, color, type, shape.
// Keep values boring and stable; tests and users rely on exact layout.

const (
	// Spacing scale.
	spaceSM = 2

	// Layout chrome.
	padX      = 1
	columnGap = 2

	// Sizing.
	maxRepoWidthCap = 28
	minWidth        = 32
	minTableHeight  = 4
	maxInspecting   = 8
	mouseWheelRows  = 3
)

// Palette uses adaptive colors so the UI works on dark and light terminals.
var (
	borderSubtle = lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
	textHi       = lipgloss.AdaptiveColor{Light: "#18181B", Dark: "#FAFAFA"}
	textLo       = lipgloss.AdaptiveColor{Light: "#71717A", Dark: "#71717A"}
	brand        = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"}
	success      = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	warning      = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	danger       = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	info         = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
	violet       = lipgloss.AdaptiveColor{Light: "#7E22CE", Dark: "#C084FC"}
)

const iconSelected = "›"

// headerBarStyle is the top chrome bar. No border or fill: it sits above
// the table with standard chrome inset so its text aligns with the
// table content (border 1 + padding 1 on each side).
func headerBarStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width)).
		Padding(0, spaceSM)
}

func headerTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(textHi).
		Bold(true)
}

func headerMetaStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(textLo)
}

func footerStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(textLo).
		Width(max(1, width)).
		Padding(0, spaceSM)
}

// tableStyle is the full-width panel frame.
func tableStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderSubtle).
		Width(max(1, width-2)).
		Padding(0, padX)
}

// columnHeaderStyle renders the table column titles.
func columnHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(textLo).
		Bold(true)
}

// selectedNumStyle bolds the # and REPO segments of the selected row.
func selectedNumStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(textHi).
		Bold(true)
}

// selectedMarkerStyle pops the › in brand so selection reads instantly.
func selectedMarkerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(brand).
		Bold(true)
}

func toneStyle(tone string, bold bool, dim bool) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(bold).Faint(dim)
	switch tone {
	case "red":
		return style.Foreground(danger)
	case "green":
		return style.Foreground(success)
	case "yellow":
		return style.Foreground(warning)
	case "blue":
		return style.Foreground(info)
	case "magenta":
		return style.Foreground(violet)
	case "cyan":
		return style.Foreground(brand)
	case "gray":
		return style.Foreground(textLo)
	case "white":
		return style.Foreground(textHi)
	default:
		return style.Foreground(textHi)
	}
}

// dividerStyle renders the repo separator. Plain border color, no faint:
// faint renders inconsistently bright across terminals.
func dividerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(borderSubtle)
}
