package inspect

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
)

func TestSessionSharesConcurrentAndCompletedFetchResults(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "failure"}[success], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			session := NewSession()
			var fetches, statuses atomic.Int32
			started := make(chan struct{})
			release := make(chan struct{})
			keys := make(chan struct{}, 8)
			session.fetchKey = func(context.Context, string) string {
				keys <- struct{}{}
				return "shared"
			}
			session.commands.fetch = func(ctx context.Context, _ string, _ time.Duration) git.Result {
				if fetches.Add(1) == 1 {
					close(started)
				}
				select {
				case <-release:
				case <-ctx.Done():
				}
				return git.Result{OK: success, Stderr: "fetch failed", Status: 1}
			}
			session.commands.status = func(_ context.Context, path string) git.Result {
				statuses.Add(1)
				return git.Result{OK: true, Stdout: "## " + path + "\n"}
			}
			results := make([]Result, 8)
			var workers sync.WaitGroup
			for index := range results {
				workers.Go(func() {
					results[index] = session.RepoContext(ctx, discover.Repo{Path: string(rune('a' + index))}, false, false, nil)
				})
			}
			waitBatchSignal(t, ctx, started)
			for range results {
				waitBatchSignal(t, ctx, keys)
			}
			if statuses.Load() != 0 {
				t.Fatal("status must wait for the shared fetch")
			}
			close(release)
			workers.Wait()
			for _, result := range results {
				if result.Failed || result.Stale == success || result.Status.Branch == nil || result.Status.Branch.Name != result.Repo.Path {
					t.Fatalf("each checkout needs its own status and shared fetch outcome: %+v", result)
				}
			}
			session.RepoContext(ctx, discover.Repo{Path: "later"}, false, false, nil)
			if fetches.Load() != 1 || statuses.Load() != 9 {
				t.Fatalf("got %d fetches and %d statuses", fetches.Load(), statuses.Load())
			}
		})
	}
}

func TestSessionFetchWaitingCanBeCanceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session := NewSession()
	keys := make(chan struct{}, 3)
	session.fetchKey = func(context.Context, string) string {
		keys <- struct{}{}
		return "shared"
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var fetches atomic.Int32
	session.commands.fetch = func(ctx context.Context, _ string, _ time.Duration) git.Result {
		fetches.Add(1)
		close(started)
		select {
		case <-release:
			return git.Result{OK: true}
		case <-ctx.Done():
			return git.Result{Stderr: ctx.Err().Error(), Status: -1}
		}
	}
	first := make(chan git.Result, 1)
	go func() { first <- session.fetch(ctx, "main", time.Second) }()
	waitBatchSignal(t, ctx, started)
	waitBatchSignal(t, ctx, keys)
	waitCtx, stopWaiting := context.WithCancel(ctx)
	waiter := make(chan git.Result, 1)
	go func() { waiter <- session.fetch(waitCtx, "linked", time.Second) }()
	waitBatchSignal(t, ctx, keys)
	stopWaiting()
	select {
	case result := <-waiter:
		if result.OK || result.Stderr != context.Canceled.Error() {
			t.Fatalf("waiter must observe cancellation: %+v", result)
		}
	case <-ctx.Done():
		t.Fatal("cancellation did not release fetch waiter")
	}
	close(release)
	select {
	case result := <-first:
		if !result.OK {
			t.Fatal("canceling a waiter must not cancel the fetching checkout")
		}
	case <-ctx.Done():
		t.Fatal("fetch did not finish")
	}
	if result := session.fetch(ctx, "later", time.Second); !result.OK || fetches.Load() != 1 {
		t.Fatal("waiter cancellation must not discard the shared result")
	}
}

func TestSessionLocalModeSkipsFetchPreparation(t *testing.T) {
	session := NewSession()
	session.fetchKey = func(context.Context, string) string {
		t.Fatal("local mode must not prepare a fetch")
		return ""
	}
	session.commands.fetch = func(context.Context, string, time.Duration) git.Result {
		t.Fatal("local mode must not fetch")
		return git.Result{}
	}
	session.commands.status = func(context.Context, string) git.Result {
		return git.Result{OK: true, Stdout: "## main\n"}
	}
	if result := session.RepoContext(nil, discover.Repo{Path: "main"}, true, false, nil); result.Failed || result.Stale {
		t.Fatalf("local inspection failed: %+v", result)
	}
}

func TestSessionFallbackFetchesEachCheckout(t *testing.T) {
	session := NewSession()
	session.fetchKey = func(context.Context, string) string { return "" }
	fetched := []string{}
	session.commands.fetch = func(_ context.Context, path string, _ time.Duration) git.Result {
		fetched = append(fetched, path)
		return git.Result{OK: true}
	}
	for _, path := range []string{"main", "linked"} {
		if result := session.fetch(context.Background(), path, time.Second); !result.OK {
			t.Fatal("fallback fetch failed")
		}
	}
	if len(fetched) != 2 || fetched[0] != "main" || fetched[1] != "linked" {
		t.Fatalf("fallback lost per-checkout fetches: %v", fetched)
	}
}
