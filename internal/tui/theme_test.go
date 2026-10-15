package tui

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var textTokens = map[string]lipgloss.CompleteAdaptiveColor{
	"textHi": textHi, "textMed": textMed, "textLo": textLo, "brand": brand, "success": success,
	"warning": warning, "danger": danger, "info": info, "violet": violet,
}

var semanticTokens = []string{"brand", "success", "warning", "danger", "info", "violet"}

func paletteModes() map[string]func(lipgloss.CompleteAdaptiveColor) lipgloss.CompleteColor {
	return map[string]func(lipgloss.CompleteAdaptiveColor) lipgloss.CompleteColor{
		"light": func(color lipgloss.CompleteAdaptiveColor) lipgloss.CompleteColor { return color.Light },
		"dark":  func(color lipgloss.CompleteAdaptiveColor) lipgloss.CompleteColor { return color.Dark },
	}
}

func ansi256Luminance(t *testing.T, index string) float64 {
	t.Helper()
	value, err := strconv.Atoi(index)
	if err != nil {
		t.Fatalf("ANSI256 value %q must be an index", index)
	}
	rgb := termenv.ConvertToRGB(termenv.ANSI256Color(value))
	channel := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(rgb.R) + 0.7152*channel(rgb.G) + 0.0722*channel(rgb.B)
}

func TestANSI256TokensStayDistinctAndReadableOnSelection(t *testing.T) {
	for mode, pick := range paletteModes() {
		t.Run(mode, func(t *testing.T) {
			seen := map[string]string{}
			wash := ansi256Luminance(t, pick(selection).ANSI256)
			for name, token := range textTokens {
				index := pick(token).ANSI256
				if other, exists := seen[index]; exists {
					t.Fatalf("%s and %s share ANSI256 %s", name, other, index)
				}
				seen[index] = name
				text := ansi256Luminance(t, index)
				ratio := (math.Max(text, wash) + 0.05) / (math.Min(text, wash) + 0.05)
				if ratio < 4.5 {
					t.Fatalf("%s %s on selection %s is %.2f:1", name, index, pick(selection).ANSI256, ratio)
				}
			}
		})
	}
}

func TestANSI16UsesStandardNamesWithoutSelectionBackground(t *testing.T) {
	for mode, pick := range paletteModes() {
		t.Run(mode, func(t *testing.T) {
			if pick(selection).ANSI != "" {
				t.Fatal("ANSI16 selection must not paint a background")
			}
			seen := map[string]string{}
			for _, name := range semanticTokens {
				index, err := strconv.Atoi(pick(textTokens[name]).ANSI)
				if err != nil || index%8 < 1 || index%8 > 6 {
					t.Fatalf("%s must use a standard color name, got %q", name, pick(textTokens[name]).ANSI)
				}
				if other, exists := seen[pick(textTokens[name]).ANSI]; exists {
					t.Fatalf("%s and %s share ANSI16 %s", name, other, pick(textTokens[name]).ANSI)
				}
				seen[pick(textTokens[name]).ANSI] = name
			}
			for _, name := range []string{"textHi", "textMed", "textLo"} {
				if value := pick(textTokens[name]).ANSI; value != "" && value != "7" && value != "8" {
					t.Fatalf("%s must use the default foreground or ANSI 7/8, got %q", name, value)
				}
			}
		})
	}
}

func TestSelectionStaysVisibleWithoutStyling(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, test := range []struct {
		profile termenv.Profile
		cap     bool
	}{
		{termenv.Ascii, true},
		{termenv.ANSI, false},
		{termenv.TrueColor, false},
	} {
		lipgloss.SetColorProfile(test.profile)
		model := newTestModel(testRepos("repo-a", "repo-b"))
		model.updateTableWithSize(80, 10)
		rows := model.visibleRows()
		selected, other := model.renderRow(rows[0]), model.renderRow(rows[1])
		if strings.HasSuffix(selected, selectionCap) != test.cap || strings.HasSuffix(other, selectionCap) {
			t.Fatalf("profile %v: selected %q other %q", test.profile, selected, other)
		}
		if lipgloss.Width(selected) != model.lineWidth() {
			t.Fatalf("selection cap must keep row width: %q", selected)
		}
	}
}

func TestBackgroundQueryRewritesOnlyMultiplexerTerm(t *testing.T) {
	for _, test := range []struct {
		term, want string
	}{
		{"tmux-256color", "xterm-256color"},
		{"screen", "xterm-256color"},
		{"screen.xterm-256color", "xterm-256color"},
		{"xterm-kitty", "xterm-kitty"},
		{"dumb", "dumb"},
		{"", ""},
	} {
		environ := queryEnviron{getenv: func(key string) string {
			if key == "TERM" {
				return test.term
			}
			return "11;0"
		}}
		if got := environ.Getenv("TERM"); got != test.want {
			t.Fatalf("TERM %q became %q, want %q", test.term, got, test.want)
		}
		if environ.Getenv("COLORFGBG") != "11;0" {
			t.Fatal("other variables, including COLORFGBG, must pass through")
		}
	}
}
