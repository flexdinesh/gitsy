package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	OK     bool
	Stdout string
	Stderr string
	Status int
}

func Run(cwd string, args ...string) Result {
	return RunContext(context.Background(), cwd, args...)
}

func RunContext(ctx context.Context, cwd string, args ...string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	fullArgs := append([]string{"-C", cwd}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	return runCommand(ctx, cmd)
}

func runCommand(ctx context.Context, cmd *exec.Cmd) Result {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	status := 0
	if err != nil {
		status = -1
		if ctx.Err() != nil {
			stderr.WriteString(ctx.Err().Error())
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			status = exitErr.ExitCode()
		} else if stderr.Len() == 0 {
			stderr.WriteString(err.Error())
		}
	}

	return Result{
		OK:     err == nil,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Status: status,
	}
}

func TopLevel(repoPath string) Result {
	return TopLevelContext(context.Background(), repoPath)
}

func TopLevelContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "rev-parse", "--show-toplevel")
}

func OriginURLContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "config", "--null", "--get", "remote.origin.url")
}

func ShortStatus(repoPath string) Result {
	return ShortStatusContext(context.Background(), repoPath)
}

func ShortStatusContext(ctx context.Context, repoPath string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result := RunContext(ctx, repoPath, "status", "--porcelain=v1", "--branch", "--ahead-behind", "--untracked-files=normal")
	if !result.OK && ctx.Err() == context.DeadlineExceeded {
		result.Stderr = "git status timed out"
	}
	return result
}

func WorktreeList(repoPath string) Result {
	return WorktreeListContext(context.Background(), repoPath)
}

func WorktreeListContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "worktree", "list", "--porcelain", "-z")
}

func FastForward(repoPath string) Result {
	return FastForwardContext(context.Background(), repoPath)
}

func FastForwardContext(ctx context.Context, repoPath string) Result {
	return FastForwardToContext(ctx, repoPath, "@{upstream}")
}

func FastForwardToContext(ctx context.Context, repoPath, commit string) Result {
	return RunContext(ctx, repoPath, "merge", "--ff-only", "--no-squash", commit)
}

func ResolveCommitContext(ctx context.Context, repoPath, revision string) Result {
	return RunContext(ctx, repoPath, "rev-parse", "--verify", revision+"^{commit}")
}

func FetchAll(repoPath string, timeout time.Duration) Result {
	return FetchAllContext(context.Background(), repoPath, timeout)
}

func FetchAllContext(ctx context.Context, repoPath string, timeout time.Duration) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "fetch", "--all")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	cmd.WaitDelay = time.Second
	result := runCommand(ctx, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		result.Stderr = "git fetch timed out"
	}
	return result
}

func ParseWorktreePaths(porcelain string) []string {
	paths := []string{}
	for _, worktree := range ParseWorktrees(porcelain) {
		paths = append(paths, worktree.Path)
	}
	return paths
}

type Worktree struct {
	Path       string
	MainPath   string
	Branch     string
	Detached   bool
	Locked     bool
	LockReason string
}

func ParseWorktrees(porcelain string) []Worktree {
	worktrees := []Worktree{}
	for _, field := range strings.Split(porcelain, "\x00") {
		if strings.HasPrefix(field, "worktree ") {
			worktrees = append(worktrees, Worktree{Path: strings.TrimPrefix(field, "worktree ")})
			continue
		}
		if len(worktrees) == 0 {
			continue
		}
		worktree := &worktrees[len(worktrees)-1]
		switch {
		case strings.HasPrefix(field, "branch "):
			worktree.Branch = strings.TrimPrefix(field, "branch refs/heads/")
		case field == "detached":
			worktree.Detached = true
		case field == "locked" || strings.HasPrefix(field, "locked "):
			worktree.Locked = true
			worktree.LockReason = strings.TrimPrefix(strings.TrimPrefix(field, "locked"), " ")
		}
	}
	for index := range worktrees {
		worktrees[index].MainPath = worktrees[0].Path
	}
	return worktrees
}

type WorktreeRemovalResult struct {
	Result
	NeedsForce bool
}

// Revalidate membership and protection even when discarding files is confirmed.
func RemoveWorktreeContext(ctx context.Context, mainPath, worktreePath string, force bool) WorktreeRemovalResult {
	fail := func(message string) WorktreeRemovalResult {
		return WorktreeRemovalResult{Result: Result{Stderr: message, Status: -1}}
	}
	target, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		return fail(fmt.Sprintf("resolve worktree: %s", err))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(fmt.Sprintf("resolve current directory: %s", err))
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return fail(fmt.Sprintf("resolve current directory: %s", err))
	}
	if relative, err := filepath.Rel(target, cwd); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fail("Cannot delete the current directory's worktree.")
	}
	listed := WorktreeListContext(ctx, mainPath)
	if !listed.OK {
		return WorktreeRemovalResult{Result: listed}
	}
	for index, worktree := range ParseWorktrees(listed.Stdout) {
		realPath, err := filepath.EvalSymlinks(worktree.Path)
		if err != nil || realPath != target {
			continue
		}
		if index == 0 {
			return fail("Cannot delete the main worktree.")
		}
		if worktree.Locked {
			return fail("Worktree is locked; unlock it before deleting.")
		}
		checked := RunContext(ctx, worktree.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--ignore-submodules=none")
		if !checked.OK {
			return WorktreeRemovalResult{Result: checked}
		}
		if checked.Stdout != "" && !force {
			return WorktreeRemovalResult{
				Result:     Result{Stderr: "Worktree has changed, untracked, or ignored files.", Status: -1},
				NeedsForce: true,
			}
		}
		args := []string{"worktree", "remove"}
		if force {
			args = append(args, "--force")
		}
		return WorktreeRemovalResult{Result: RunContext(ctx, mainPath, append(args, "--", worktree.Path)...)}
	}
	return fail("Worktree is no longer registered with this repository.")
}
