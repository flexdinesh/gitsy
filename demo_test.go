package gitsy_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestDemoSetupRejectsUnownedTargets(t *testing.T) {
	for _, kind := range []string{"directory", "file", "symlink", "marker symlink", "git repository"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			preserved := filepath.Join(target, "keep")
			switch kind {
			case "file":
				preserved = target
			case "symlink":
				realTarget := filepath.Join(dir, "real")
				mkdirAll(t, realTarget)
				writeFile(t, filepath.Join(realTarget, ".gitsy-demo"), "gitsy demo workspace\n")
				if err := os.Symlink(realTarget, target); err != nil {
					t.Fatal(err)
				}
				preserved = filepath.Join(realTarget, "keep")
				target += "///"
			default:
				mkdirAll(t, target)
				if kind == "marker symlink" {
					marker := filepath.Join(dir, "marker")
					writeFile(t, marker, "gitsy demo workspace\n")
					if err := os.Symlink(marker, filepath.Join(target, ".gitsy-demo")); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "git repository" {
					mkdirAll(t, filepath.Join(target, ".git"))
					writeFile(t, filepath.Join(target, ".gitsy-demo"), "gitsy demo workspace\n")
				}
			}
			writeFile(t, preserved, "user work\n")
			output, err := runDemo(target)
			if err == nil || !strings.Contains(output, "Refusing to reset unowned demo directory") {
				t.Fatalf("unowned target must be rejected: err=%v output=%s", err, output)
			}
			content, err := os.ReadFile(preserved)
			if err != nil || string(content) != "user work\n" {
				t.Fatalf("unrelated work must survive: content=%q err=%v", content, err)
			}
		})
	}
}

func TestDemoSetupIndependentOfDefaultBranchAndResetsOwnedTargets(t *testing.T) {
	requireGit(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	for _, branch := range []string{"master", "main"} {
		t.Run(branch, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_VALUE_0", branch)
			target := filepath.Join(t.TempDir(), "demo")
			for attempt := 0; attempt < 2; attempt++ {
				if output, err := runDemo(target); err != nil {
					t.Fatalf("demo setup failed: %v\n%s", err, output)
				}
				if _, err := os.Stat(filepath.Join(target, "old-state")); !os.IsNotExist(err) {
					t.Fatalf("owned fixture must reset previous state: %v", err)
				}
				behind := filepath.Join(target, "repo-behind")
				if result := git.FetchAll(behind, 3*time.Second); !result.OK {
					t.Fatal(result.Stderr)
				}
				result := git.ShortStatus(behind)
				parsed := status.Parse(result.Stdout)
				if !result.OK || parsed.Branch == nil || parsed.Branch.Behind != 1 || len(parsed.Items) != 0 {
					t.Fatalf("demo must create a clean repo one commit behind: %+v", parsed)
				}
				writeFile(t, filepath.Join(target, "old-state"), "previous fixture state\n")
			}
		})
	}
}

func runDemo(target string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "./scripts/setup-demo.sh", target).CombinedOutput()
	return string(output), err
}
