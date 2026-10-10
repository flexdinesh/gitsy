package inspect

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestReposContextBoundsConcurrencyAndPreservesOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repos := batchRepos(20)
	started := make(chan struct{}, len(repos))
	completed := make(chan struct{}, len(repos))
	releaseFirst := make(chan struct{})
	releaseOthers := make(chan struct{})
	var mutex sync.Mutex
	active, maximum := 0, 0
	calls := make(map[string]int)
	done := make(chan []Result, 1)
	go func() {
		done <- reposContext(ctx, repos, func(ctx context.Context, repo discover.Repo) Result {
			mutex.Lock()
			active++
			maximum = max(maximum, active)
			calls[repo.Path]++
			mutex.Unlock()
			started <- struct{}{}
			release := releaseOthers
			if repo.Path == repos[0].Path {
				release = releaseFirst
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			mutex.Lock()
			active--
			mutex.Unlock()
			completed <- struct{}{}
			return Result{Repo: repo, Status: status.Parse("## " + repo.DisplayName + "\n")}
		})
	}()
	for range 8 {
		waitBatchSignal(t, ctx, started)
	}
	select {
	case <-started:
		t.Fatal("started more than eight inspections before any finished")
	default:
	}
	close(releaseOthers)
	for range len(repos) - 1 {
		waitBatchSignal(t, ctx, completed)
	}
	close(releaseFirst)
	results := waitBatchResults(t, ctx, done)
	if maximum != 8 {
		t.Fatalf("maximum simultaneous inspections = %d, want 8", maximum)
	}
	for index, result := range results {
		if result.Repo != repos[index] || result.Status.Branch == nil || result.Status.Branch.Name != repos[index].DisplayName || result.Failed {
			t.Fatalf("result %d mismatched input: %+v", index, result)
		}
		if calls[repos[index].Path] != 1 {
			t.Fatalf("inspection count for %s = %d, want 1", repos[index].Path, calls[repos[index].Path])
		}
	}
}

func TestReposContextCancellationSkipsPendingInspections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repos := batchRepos(20)
	started := make(chan struct{}, len(repos))
	done := make(chan []Result, 1)
	go func() {
		done <- reposContext(ctx, repos, func(ctx context.Context, repo discover.Repo) Result {
			started <- struct{}{}
			<-ctx.Done()
			return Result{Repo: repo, Failed: true}
		})
	}()
	for range 8 {
		waitBatchSignal(t, ctx, started)
	}
	cancel()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	results := waitBatchResults(t, waitCtx, done)
	if len(started) != 0 {
		t.Fatal("inspected queued repositories after cancellation")
	}
	for index, result := range results {
		if result.Repo != repos[index] || !result.Failed {
			t.Fatalf("cancelled result %d lost identity or claims success: %+v", index, result)
		}
	}
}

func TestReposContextEmptyNilAndCancelledContexts(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name       string
		ctx        context.Context
		repos      []discover.Repo
		wantCalls  int
		wantFailed bool
	}{
		{name: "empty", ctx: context.Background()},
		{name: "nil context", repos: batchRepos(1), wantCalls: 1},
		{name: "cancelled", ctx: cancelled, repos: batchRepos(2), wantFailed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			results := reposContext(test.ctx, test.repos, func(ctx context.Context, repo discover.Repo) Result {
				if ctx == nil {
					t.Error("inspector received nil context")
				}
				calls++
				return Result{Repo: repo}
			})
			if calls != test.wantCalls || len(results) != len(test.repos) {
				t.Fatalf("got %d calls and %d results, want %d calls and %d results", calls, len(results), test.wantCalls, len(test.repos))
			}
			for index, result := range results {
				if result.Repo != test.repos[index] || result.Failed != test.wantFailed {
					t.Fatalf("unexpected result %d: %+v", index, result)
				}
			}
		})
	}
}

func batchRepos(count int) []discover.Repo {
	repos := make([]discover.Repo, count)
	for index := range repos {
		repos[index] = discover.Repo{Path: fmt.Sprintf("/repo/%d", index), DisplayName: fmt.Sprintf("repo-%d", index)}
	}
	return repos
}

func waitBatchSignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal("timed out waiting for inspection")
	}
}

func waitBatchResults(t *testing.T, ctx context.Context, done <-chan []Result) []Result {
	t.Helper()
	select {
	case results := <-done:
		return results
	case <-ctx.Done():
		t.Fatal("timed out waiting for batch results")
		return nil
	}
}
