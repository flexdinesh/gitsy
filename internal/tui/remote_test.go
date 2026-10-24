package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/muesli/termenv"
)

func withTrueColor(t *testing.T) {
	t.Helper()
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestRenderSegmentsAccentsOnlyTheSegment(t *testing.T) {
	withTrueColor(t)
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("#112233"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#445566"))
	full := "main ✓ clean · ⚠ remote auth needed"
	compact := "⚠ remote auth needed · main"
	segment := "⚠ remote auth needed"
	for _, test := range []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"fits", full, 40, base.Render("main ✓ clean · ") + accent.Render(segment) + base.Render("     ")},
		{"truncated inside segment", full, 20, base.Render("main ✓ clean · ") + accent.Render("⚠ re…") + base.Render("")},
		{"truncated before segment", full, 10, base.Render("main ✓ cl…")},
		{"leading segment", compact, 30, base.Render("") + accent.Render(segment) + base.Render(" · main   ")},
		{"no segment", "main ✓ clean", 14, base.Render("main ✓ clean  ")},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := renderSegments(test.text, segment, test.width, "", base, accent)
			if test.name == "no segment" {
				got = renderSegments(test.text, "", test.width, "", base, accent)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			if lipgloss.Width(got) != test.width {
				t.Fatalf("width %d, want %d", lipgloss.Width(got), test.width)
			}
		})
	}
}

func TestRemoteStateRowsKeepWidthAndReason(t *testing.T) {
	withTrueColor(t)
	model := previewModel()
	model.results[0].Status = status.Parse("## main...origin/main")
	model.results[0].Stale = true
	model.results[0].Reason = inspect.ReasonAuth
	model.results[1].Status = status.Parse("## main")
	model.results[1].Remotes = inspect.Remotes{Known: true}
	for _, width := range []int{40, 60, 80, 104, 160} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			model.width, model.height = width, 24
			model.refresh()
			for _, selected := range []int{0, 1} {
				model.selected = selected
				for _, row := range model.rows[:4] {
					if got := lipgloss.Width(model.renderRow(row)); got != model.lineWidth() {
						t.Fatalf("row width %d, want %d: %q", got, model.lineWidth(), model.renderRow(row))
					}
				}
			}
			view := model.View()
			if !strings.Contains(view, "⚠ remote auth") {
				t.Fatalf("fetch reason must survive at %d columns:\n%s", width, view)
			}
			if strings.Contains(view, "stale") || strings.Contains(view, "skipped") {
				t.Fatalf("generic stale wording must be gone:\n%s", view)
			}
		})
	}
}

func TestRemoteRailHidesWhileLoading(t *testing.T) {
	model := previewModel()
	model.results[0].Loading = true
	model.results[0].Remotes = inspect.Remotes{Known: true}
	model.selected = 0
	if info := strings.Join(model.infoLines(30), "\n"); strings.Contains(info, "Remote") {
		t.Fatalf("loading rows must not claim remote state:\n%s", info)
	}
}
