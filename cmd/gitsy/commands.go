package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/flexdinesh/gitsy/internal/args"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/ui"
)

// console holds command streams; summaries go to stdout, failures and prompts to stderr.
type console struct {
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	terminal bool // stdin can answer confirmation prompts
	verbose  bool
}

var errFailed = exitError{code: exitFailed}

func runCommand(ctx context.Context, out console, workspace discover.Workspace, options args.Options, warn func(string)) error {
	switch options.Command {
	case args.CommandStatus:
		return runInspection(ctx, out, workspace, inspect.NewSession(), options.NoFetch, false, func(results []inspect.Result) string {
			return ui.StatusSummary(workspace, results)
		}, warn)
	case args.CommandSync:
		if options.FetchOnly {
			return runInspection(ctx, out, workspace, inspect.NewSession(), false, false, ui.FetchSummary, warn)
		}
		return runInspection(ctx, out, workspace, inspect.NewSession(), false, true, ui.SyncSummary, warn)
	case args.CommandDeleteMerged:
		return runDeleteMerged(ctx, out, workspace, options, warn)
	}
	return fmt.Errorf("unsupported command: %s", options.Command)
}

func runInspection(ctx context.Context, out console, workspace discover.Workspace, session *inspect.Session, noFetch, syncRepos bool, summary func([]inspect.Result) string, warn func(string)) error {
	results := session.ReposContext(ctx, workspace.Repos, noFetch, syncRepos, warn)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(workspace.Repos) == 0 {
		_, err := fmt.Fprintln(out.stdout, ui.EmptyMessage(0))
		return err
	}
	report := summary(results) + "\n"
	if out.verbose {
		report = ui.PlainGroups(workspace, results) + report
	}
	if _, err := io.WriteString(out.stdout, report); err != nil {
		return err
	}
	return printFailures(out.stderr, results)
}

// printFailures reports each failed repository once and returns errFailed if any failed.
func printFailures(output io.Writer, results []inspect.Result) error {
	failed := false
	for _, result := range results {
		if reason, ok := ui.Failure(result); ok {
			failed = true
			if _, err := fmt.Fprintf(output, "gitsy: %s: %s\n", ui.RepoLine(result.Repo), reason); err != nil {
				return err
			}
		}
	}
	if failed {
		return errFailed
	}
	return nil
}

// runDeleteMerged force deletes linked worktrees merged into the base branch,
// rechecking each one immediately before deletion. Branches are kept.
func runDeleteMerged(ctx context.Context, out console, workspace discover.Workspace, options args.Options, warn func(string)) error {
	worktrees := []discover.Repo{}
	for _, repo := range workspace.Repos {
		if repo.Worktree != nil {
			worktrees = append(worktrees, repo)
		}
	}
	session := inspect.NewSession()
	session.DetectMerged = true
	session.Base = options.Base
	results := session.ReposContext(ctx, worktrees, options.NoFetch, false, warn)
	if err := ctx.Err(); err != nil {
		return err
	}
	inspectErr := printFailures(out.stderr, results)
	if inspectErr != nil && !errors.Is(inspectErr, errFailed) {
		return inspectErr
	}

	merged := []ui.RepoResult{}
	for _, result := range results {
		if result.MergedInto != "" {
			merged = append(merged, ui.RepoResult{Repo: result.Repo, Status: result.Status, MergedInto: result.MergedInto})
		}
	}
	if len(merged) == 0 {
		if _, err := fmt.Fprintln(out.stdout, "No merged worktrees."); err != nil {
			return err
		}
		return inspectErr
	}

	if !options.Yes {
		if !out.terminal {
			return exitError{code: exitUsage, err: fmt.Errorf("confirmation required to delete %s; rerun with -y", ui.Count(len(merged), "merged worktree"))}
		}
		confirmed, err := confirmDeletion(ctx, out, merged)
		if err != nil {
			return err
		}
		if !confirmed {
			return exitError{code: exitFailed, err: errors.New("aborted; no worktrees deleted")}
		}
	}

	deleted, skipped, failed := 0, 0, 0
	for _, result := range merged {
		if ctx.Err() != nil {
			break
		}
		repo := result.Repo
		into, _ := git.MergedContext(ctx, repo.Path, git.BaseRefsContext(ctx, repo.Path, options.Base))
		if into == "" {
			skipped++
			if out.verbose {
				fmt.Fprintf(out.stdout, "Skipped %s: no longer merged\n", ui.RepoLine(repo))
			}
			continue
		}
		removal := git.RemoveWorktreeContext(ctx, repo.Worktree.MainPath, repo.Path, true)
		if removal.OK {
			deleted++
			if out.verbose {
				fmt.Fprintf(out.stdout, "Deleted %s\n", ui.RepoLine(repo))
			}
			continue
		}
		failed++
		fmt.Fprintf(out.stderr, "gitsy: %s: delete failed: %s\n", ui.RepoLine(repo), removalError(removal))
	}
	if _, err := fmt.Fprintln(out.stdout, ui.DeletionSummary(deleted, skipped, failed, "")); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if failed > 0 {
		return errFailed
	}
	return inspectErr
}

func confirmDeletion(ctx context.Context, out console, merged []ui.RepoResult) (bool, error) {
	var prompt strings.Builder
	prompt.WriteString("Merged worktrees:\n")
	for _, result := range merged {
		fmt.Fprintf(&prompt, "  %s · merged into %s\n", ui.RepoLine(result.Repo), result.MergedInto)
	}
	prompt.WriteString(ui.DeletionPrompt(merged) + " [y/N] ")
	if _, err := io.WriteString(out.stderr, prompt.String()); err != nil {
		return false, err
	}

	answers := make(chan string, 1)
	go func() {
		answer, _ := bufio.NewReader(out.stdin).ReadString('\n')
		answers <- answer
	}()
	select {
	case <-ctx.Done():
		fmt.Fprintln(out.stderr)
		return false, ctx.Err()
	case answer := <-answers:
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			return true, nil
		}
		return false, nil
	}
}

func removalError(result git.WorktreeRemovalResult) string {
	if message := strings.Join(strings.Fields(result.Stderr), " "); message != "" {
		return message
	}
	return fmt.Sprintf("git exited %d", result.Status)
}
