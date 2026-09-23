package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/flexdinesh/gitsy/internal/ui"
	"github.com/muesli/termenv"
)

func previewModel() Model {
	model := newTestModel(testRepos("api", "web", "design-system", "worker", "docs", "infra", "cli", "sandbox"))
	raw := []string{
		"## feat/auth...origin/feat/auth [ahead 2]\n M auth.go\n?? auth_test.go",
		"## main...origin/main [behind 3]",
		"## main...origin/main",
		"## fix/retry...origin/fix/retry\nUU queue.go",
		"## main...origin/main",
		"## main...origin/main",
		"## main...origin/main",
		"## experiment",
	}
	for index := range model.results {
		model.results[index].Loading = false
		model.results[index].Status = status.Parse(raw[index])
	}
	model.results[4].Stale = true
	model.results[5].Failed = true
	model.results[6].Sync = &ui.SyncOutcome{Kind: "synced", Pulled: 2}
	model.results[7].Loading = true
	return model
}

func TestDenseOverviewAndFileToggle(t *testing.T) {
	model := previewModel()
	model.width, model.height = 80, 16
	model.refresh()
	if len(model.rows) != len(model.results) {
		t.Fatal("overview must use exactly one row per repository")
	}
	model.selectRepo(3)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if !model.expanded || model.selected != 3 || !strings.Contains(model.View(), "queue.go") {
		t.Fatal("Tab must reveal files and preserve selection")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.expanded || model.selected != 3 || len(model.rows) != len(model.results) {
		t.Fatal("Tab must return to dense overview without changing selection")
	}
}

func TestDenseOverviewWheelAndEndNavigation(t *testing.T) {
	model := previewModel()
	model.width, model.height = 80, 10
	model.refresh()
	updated, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	model = updated.(Model)
	if model.selected != 3 || model.offset != 3 {
		t.Fatalf("wheel must advance three repository rows: selected=%d offset=%d", model.selected, model.offset)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if model.selected != 7 || !strings.Contains(model.View(), "sandbox") {
		t.Fatal("End must reveal and select the last repository")
	}
}

func TestNarrowHeaderKeepsExceptionCounts(t *testing.T) {
	model := previewModel()
	header := model.renderHeader(40)
	for _, label := range []string{"8 repos", "1 failed", "2 changed", "1 behind", "1 stale"} {
		if !strings.Contains(header, label) {
			t.Fatalf("narrow header lost %q: %s", label, header)
		}
	}
	if lipgloss.Width(header) > 40 || lipgloss.Height(header) != 3 {
		t.Fatal("summary must fit existing header budget")
	}
}

func TestViewsFitTerminal(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {20, 6}, {31, 10}, {32, 7}, {40, 10}, {60, 16}, {80, 24}, {104, 24}, {120, 24}, {160, 32}} {
		for _, state := range []string{"overview", "files", "empty"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				model := previewModel()
				model.results[0].Repo.DisplayName = "日本語-worktree-long-name"
				model.results[0].Repo.Path = "/workspace/日本語/very-long-repository-path"
				model.expanded = state == "files"
				if state == "empty" {
					model.results = nil
				}
				model.width, model.height = size[0], size[1]
				model.refresh()
				view := model.View()
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
					t.Fatalf("view overflows %dx%d: got %dx%d\n%s", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view), view)
				}
			})
		}
	}
}

// Opt-in captures use the actual View output and never contact Git or remotes.
func TestRenderPreviews(t *testing.T) {
	directory := os.Getenv("GITSY_PREVIEW_DIR")
	if directory == "" {
		t.Skip("set GITSY_PREVIEW_DIR to capture terminal renders")
	}
	profile, dark := lipgloss.ColorProfile(), lipgloss.HasDarkBackground()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(profile)
		lipgloss.SetHasDarkBackground(dark)
	})
	lipgloss.SetColorProfile(termenv.TrueColor)
	captures := map[string]string{}
	for _, theme := range []string{"dark", "light"} {
		lipgloss.SetHasDarkBackground(theme == "dark")
		for _, mode := range []string{"wide", "narrow", "files", "empty"} {
			model := previewModel()
			model.width, model.height = 120, 24
			if mode == "narrow" {
				model.width, model.height = 40, 16
			}
			model.expanded = mode == "files"
			if mode == "empty" {
				model.results = nil
			}
			model.refresh()
			captures[theme+"-"+mode] = model.View()
		}
	}
	data, err := json.Marshal(captures)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "renders.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
