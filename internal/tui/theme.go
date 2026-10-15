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

// Palette pins truecolor, ANSI256 and ANSI16 values for light and dark
// backgrounds instead of relying on nearest-color degradation, which merges
// tokens and drops contrast. Fields are hex, 256-color index, 16-color index;
// an empty 16-color value keeps the terminal default.
var (
	borderSubtle = adaptive([3]string{"#C4CDD6", "250", "8"}, [3]string{"#354555", "238", "8"})
	textHi       = adaptive([3]string{"#243446", "236", ""}, [3]string{"#D8E2ED", "254", ""})
	textMed      = adaptive([3]string{"#465A6E", "240", ""}, [3]string{"#ADBDCD", "250", "7"})
	textLo       = adaptive([3]string{"#566B7F", "242", "8"}, [3]string{"#8CA2B8", "247", "8"})
	brand        = adaptive([3]string{"#176C61", "23", "6"}, [3]string{"#63C5B5", "73", "14"})
	success      = adaptive([3]string{"#306B49", "22", "2"}, [3]string{"#9AC79D", "108", "10"})
	warning      = adaptive([3]string{"#895B10", "94", "3"}, [3]string{"#E5B567", "179", "11"})
	danger       = adaptive([3]string{"#AC3439", "124", "1"}, [3]string{"#EE9298", "210", "9"})
	info         = adaptive([3]string{"#325F91", "25", "4"}, [3]string{"#8BB4DF", "110", "12"})
	violet       = adaptive([3]string{"#785297", "96", "5"}, [3]string{"#BDA6DB", "140", "13"})
	// ANSI16 selection relies on the marker and bold text, never a background.
	selection = adaptive([3]string{"#E2EFEB", "255", ""}, [3]string{"#203B3B", "236", ""})
)

func adaptive(light, dark [3]string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: light[0], ANSI256: light[1], ANSI: light[2]},
		Dark:  lipgloss.CompleteColor{TrueColor: dark[0], ANSI256: dark[1], ANSI: dark[2]},
	}
}

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
	case "muted":
		return style.Foreground(textMed)
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
