package inspect

import (
	"context"
	"sync"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
)

type fetchCall struct {
	done   chan struct{}
	result git.Result
}

// Session shares fetches only for one inspection run.
type Session struct {
	// DetectMerged checks linked worktrees against Base, or the detected default branch.
	DetectMerged bool
	Base         string

	mutex    sync.Mutex
	fetches  map[string]*fetchCall
	commands gitCommands
	fetchKey func(context.Context, string) string
	baseRefs func(context.Context, string, string) []string
}

func NewSession() *Session {
	return &Session{
		fetches: make(map[string]*fetchCall),
		commands: gitCommands{
			fetch:   git.FetchAllContext,
			status:  git.ShortStatusContext,
			merge:   git.FastForwardToContext,
			resolve: git.ResolveCommitContext,
			merged:  git.MergedContext,
			remotes: git.RemoteURLsContext,
		},
		fetchKey: git.FetchKeyContext,
		baseRefs: git.BaseRefsContext,
	}
}

func (session *Session) RepoContext(ctx context.Context, repo discover.Repo, noFetch, syncRepo bool, warn func(string)) Result {
	commands := session.commands
	commands.fetch = session.fetch
	commands.baseRefs = nil
	if session.DetectMerged {
		commands.baseRefs = func(ctx context.Context, repoPath string) []string {
			return session.baseRefs(ctx, repoPath, session.Base)
		}
	}
	return repoContext(ctx, repo, noFetch, syncRepo, warn, commands)
}

// ReposContext inspects repos concurrently, preserving their order in the results.
func (session *Session) ReposContext(ctx context.Context, repos []discover.Repo, noFetch, syncRepos bool, warn func(string)) []Result {
	return reposContext(ctx, repos, func(ctx context.Context, repo discover.Repo) Result {
		return session.RepoContext(ctx, repo, noFetch, syncRepos, warn)
	})
}

func (session *Session) fetch(ctx context.Context, repoPath string, timeout time.Duration) git.Result {
	if ctx.Err() != nil {
		return git.Result{Stderr: ctx.Err().Error(), Status: -1}
	}
	key := session.fetchKey(ctx, repoPath)
	if key == "" {
		return session.commands.fetch(ctx, repoPath, timeout)
	}
	session.mutex.Lock()
	if call, exists := session.fetches[key]; exists {
		session.mutex.Unlock()
		select {
		case <-ctx.Done():
			return git.Result{Stderr: ctx.Err().Error(), Status: -1}
		case <-call.done:
			return call.result
		}
	}
	call := &fetchCall{done: make(chan struct{})}
	session.fetches[key] = call
	session.mutex.Unlock()

	call.result = session.commands.fetch(ctx, repoPath, timeout)
	close(call.done)
	return call.result
}
