package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Terminal-cell spacing and adaptive palette.

const (
	// Spacing scale.
	spaceSM = 2

	// Layout chrome.
	padX      = 1
	columnGap = 2
	panelGap  = 2

	// Reserve at least 72 columns for the ledger before adding context.
	tableMinOuter = 72
	infoMinOuter  = 30
	infoMaxOuter  = 38

	// Sizing.
	maxRepoWidthCap = 28
	minWidth        = 32
	minTableHeight  = 4
	maxInspecting   = 8
	mouseWheelRows  = 3
)

// Palette uses adaptive colors so the UI works on dark and light terminals.
var (
	borderSubtle = lipgloss.AdaptiveColor{Light: "#C4CDD6", Dark: "#354555"}
	textHi       = lipgloss.AdaptiveColor{Light: "#243446", Dark: "#D8E2ED"}
	textMed      = lipgloss.AdaptiveColor{Light: "#465A6E", Dark: "#ADBDCD"}
	textLo       = lipgloss.AdaptiveColor{Light: "#566B7F", Dark: "#8CA2B8"}
	brand        = lipgloss.AdaptiveColor{Light: "#176C61", Dark: "#63C5B5"}
	success      = lipgloss.AdaptiveColor{Light: "#306B49", Dark: "#9AC79D"}
	warning      = lipgloss.AdaptiveColor{Light: "#895B10", Dark: "#E5B567"}
	danger       = lipgloss.AdaptiveColor{Light: "#AC3439", Dark: "#EE9298"}
	info         = lipgloss.AdaptiveColor{Light: "#325F91", Dark: "#8BB4DF"}
	violet       = lipgloss.AdaptiveColor{Light: "#785297", Dark: "#BDA6DB"}
	selection    = lipgloss.AdaptiveColor{Light: "#E2EFEB", Dark: "#203B3B"}
)

const iconSelected = "›"

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

// Open gutters align the ledger with the header and footer.
func tableStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width)).
		Padding(0, spaceSM)
}

// One rule separates selected-repository context from the ledger.
func infoStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(borderSubtle).
		Width(max(1, width-1)).
		Padding(0, padX)
}

// infoSectionStyle renders the info panel section labels.
func infoSectionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(textMed).
		Bold(true)
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
	style := lipgloss.NewStyle().Bold(bold)
	if dim {
		return style.Foreground(textLo)
	}
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

// Faint renders inconsistently across terminals; use explicit colors.
func dividerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(borderSubtle)
}
