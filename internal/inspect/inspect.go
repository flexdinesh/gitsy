package inspect

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/flexdinesh/gitsy/internal/ui"
)

func Repos(repos []discover.Repo, noFetch bool, syncRepos bool, warn func(message string)) []ui.RepoResult {
	results := make([]ui.RepoResult, len(repos))
	var waitGroup sync.WaitGroup

	for index, repo := range repos {
		waitGroup.Add(1)
		go func(index int, repo discover.Repo) {
			defer waitGroup.Done()
			results[index] = Repo(repo, noFetch, syncRepos, warn)
		}(index, repo)
	}

	waitGroup.Wait()
	return results
}

func Repo(repo discover.Repo, noFetch bool, syncRepo bool, warn func(message string)) ui.RepoResult {
	stale := false
	if !noFetch {
		fetchResult := git.FetchAll(repo.Path, 30*time.Second)
		if !fetchResult.OK {
			stale = true
			if warn != nil {
				warn(fmt.Sprintf("Fetch failed for %s: %s", repo.DisplayName, gitError(fetchResult)))
			}
		}
	}

	statusResult := git.ShortStatus(repo.Path)
	if !statusResult.OK {
		if warn != nil {
			warn(fmt.Sprintf("Failed to read status for %s: %s", repo.DisplayName, gitError(statusResult)))
		}
		return ui.RepoResult{
			Repo:   repo,
			Status: status.Parse(""),
			Failed: true,
			Stale:  stale,
		}
	}

	parsedStatus := status.Parse(statusResult.Stdout)
	result := ui.RepoResult{
		Repo:   repo,
		Status: parsedStatus,
		Stale:  stale,
	}

	if syncRepo && status.CanFastForward(parsedStatus) {
		pulled := parsedStatus.Branch.Behind
		ffResult := git.FastForward(repo.Path)
		if !ffResult.OK {
			if warn != nil {
				warn(fmt.Sprintf("Sync failed for %s: %s", repo.DisplayName, gitError(ffResult)))
			}
			result.Sync = &ui.SyncOutcome{Kind: "failed", Reason: strings.TrimSpace(ffResult.Stderr)}
			return result
		}

		postStatus := git.ShortStatus(repo.Path)
		if !postStatus.OK {
			if warn != nil {
				warn(fmt.Sprintf("Failed to read status after sync for %s: %s", repo.DisplayName, gitError(postStatus)))
			}
			result.Failed = true
			result.Sync = &ui.SyncOutcome{Kind: "synced", Pulled: pulled}
			return result
		}

		result.Status = status.Parse(postStatus.Stdout)
		result.Sync = &ui.SyncOutcome{Kind: "synced", Pulled: pulled}
	}

	return result
}

func gitError(result git.Result) string {
	stderr := strings.TrimSpace(result.Stderr)
	if stderr != "" {
		return stderr
	}
	return fmt.Sprintf("git exited %d", result.Status)
}
