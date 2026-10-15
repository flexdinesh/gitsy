package inspect

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
)

type SyncOutcome struct {
	Kind   string
	Pulled int
	Reason string
}

type Result struct {
	Repo   discover.Repo
	Status status.Parsed
	Failed bool
	Stale  bool
	Sync   *SyncOutcome
	Reason string // short explanation for failed, stale or skipped state
}

type gitCommands struct {
	fetch   func(context.Context, string, time.Duration) git.Result
	status  func(context.Context, string) git.Result
	merge   func(context.Context, string, string) git.Result
	resolve func(context.Context, string, string) git.Result
}

func Repos(repos []discover.Repo, noFetch bool, syncRepos bool, warn func(message string)) []Result {
	return ReposContext(context.Background(), repos, noFetch, syncRepos, warn)
}

func ReposContext(ctx context.Context, repos []discover.Repo, noFetch bool, syncRepos bool, warn func(message string)) []Result {
	session := NewSession()
	return reposContext(ctx, repos, func(ctx context.Context, repo discover.Repo) Result {
		return session.RepoContext(ctx, repo, noFetch, syncRepos, warn)
	})
}

func reposContext(ctx context.Context, repos []discover.Repo, inspect func(context.Context, discover.Repo) Result) []Result {
	if ctx == nil {
		ctx = context.Background()
	}
	results := make([]Result, len(repos))
	for index, repo := range repos {
		results[index] = Result{Repo: repo, Failed: true}
	}
	jobs := make(chan int)
	var waitGroup sync.WaitGroup

	for range min(8, len(repos)) {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				results[index] = inspect(ctx, repos[index])
			}
		}()
	}

schedule:
	for index := range repos {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			break schedule
		case jobs <- index:
		}
	}
	close(jobs)
	waitGroup.Wait()
	return results
}

func Repo(repo discover.Repo, noFetch bool, syncRepo bool, warn func(message string)) Result {
	return RepoContext(context.Background(), repo, noFetch, syncRepo, warn)
}

func RepoContext(ctx context.Context, repo discover.Repo, noFetch bool, syncRepo bool, warn func(message string)) Result {
	return NewSession().RepoContext(ctx, repo, noFetch, syncRepo, warn)
}

func repoContext(ctx context.Context, repo discover.Repo, noFetch bool, syncRepo bool, warn func(message string), commands gitCommands) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	stale := false
	reason := ""
	if !noFetch || syncRepo {
		fetchResult := commands.fetch(ctx, repo.Path, fetchTimeout)
		if !fetchResult.OK {
			stale = true
			reason = fetchReason(fetchResult)
			if warn != nil {
				warn(fmt.Sprintf("Fetch failed for %s: %s", repo.DisplayName, gitError(fetchResult)))
			}
		}
	}
	var syncOutcome *SyncOutcome
	if syncRepo && stale && ctx.Err() == nil {
		syncOutcome = &SyncOutcome{Kind: "skipped", Reason: "fetch failed"}
	}

	statusResult := commands.status(ctx, repo.Path)
	if !statusResult.OK {
		if warn != nil {
			warn(fmt.Sprintf("Failed to read status for %s: %s", repo.DisplayName, gitError(statusResult)))
		}
		return Result{
			Repo:   repo,
			Status: status.Parse(""),
			Failed: true,
			Stale:  stale,
			Sync:   syncOutcome,
			Reason: statusReason(statusResult),
		}
	}

	parsedStatus := status.Parse(statusResult.Stdout)
	result := Result{
		Repo:   repo,
		Status: parsedStatus,
		Stale:  stale,
		Sync:   syncOutcome,
		Reason: reason,
	}
	if syncRepo && stale {
		return result
	}

	if syncRepo && status.CanFastForward(parsedStatus) {
		pulled := parsedStatus.Branch.Behind
		syncFailed := func(reason string) Result {
			if warn != nil {
				warn(fmt.Sprintf("Sync failed for %s: %s", repo.DisplayName, reason))
			}
			result.Sync = &SyncOutcome{Kind: "failed", Reason: reason}
			result.Reason = "sync failed: " + shortMessage(reason)
			return result
		}
		target := commands.resolve(ctx, repo.Path, "@{upstream}")
		if !target.OK {
			return syncFailed(gitError(target))
		}
		commit := strings.TrimSpace(target.Stdout)
		if commit == "" {
			return syncFailed("upstream commit is unavailable")
		}
		ffResult := commands.merge(ctx, repo.Path, commit)
		if !ffResult.OK {
			return syncFailed(gitError(ffResult))
		}
		head := commands.resolve(ctx, repo.Path, "HEAD")
		if !head.OK {
			return syncFailed(gitError(head))
		}
		if strings.TrimSpace(head.Stdout) != commit {
			return syncFailed("HEAD did not reach the upstream commit")
		}

		postStatus := commands.status(ctx, repo.Path)
		if !postStatus.OK {
			if warn != nil {
				warn(fmt.Sprintf("Failed to read status after sync for %s: %s", repo.DisplayName, gitError(postStatus)))
			}
			result.Failed = true
			result.Sync = &SyncOutcome{Kind: "synced", Pulled: pulled}
			result.Reason = statusReason(postStatus)
			return result
		}

		result.Status = status.Parse(postStatus.Stdout)
		result.Sync = &SyncOutcome{Kind: "synced", Pulled: pulled}
	}

	return result
}

const fetchTimeout = 30 * time.Second

// fetchReason classifies fetch stderr into a short, stable phrase.
func fetchReason(result git.Result) string {
	stderr := strings.ToLower(result.Stderr)
	switch {
	case strings.Contains(stderr, "timed out") && !strings.Contains(stderr, "connection timed out"):
		return "fetch timed out after " + fetchTimeout.String()
	case containsAny(stderr, "authentication failed", "permission denied", "could not read username", "could not read password", "terminal prompts disabled", "invalid username or password", "returned error: 401", "returned error: 403"):
		return "fetch failed: auth"
	case containsAny(stderr, "repository not found", "does not appear to be a git repository", "returned error: 404"):
		return "fetch failed: repo not found"
	case containsAny(stderr, "could not resolve host", "connection refused", "connection timed out", "network is unreachable", "no route to host", "failed to connect", "could not read from remote repository"):
		return "remote unreachable"
	case strings.Contains(stderr, "context canceled"):
		return "fetch canceled"
	}
	return "fetch failed"
}

func statusReason(result git.Result) string {
	if strings.Contains(result.Stderr, "timed out") {
		return "status failed: timed out"
	}
	return "status failed: " + shortMessage(gitError(result))
}

// shortMessage keeps the first line of a Git error without its severity prefix.
func shortMessage(message string) string {
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"fatal:", "error:"} {
			line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
		if line != "" {
			if runes := []rune(line); len(runes) > maxReasonRunes {
				line = string(runes[:maxReasonRunes-1]) + "…"
			}
			return line
		}
	}
	return "unknown error"
}

const maxReasonRunes = 48

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func gitError(result git.Result) string {
	stderr := strings.TrimSpace(result.Stderr)
	if stderr != "" {
		return stderr
	}
	return fmt.Sprintf("git exited %d", result.Status)
}
