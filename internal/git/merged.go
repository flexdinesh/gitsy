package git

import (
	"context"
	"strings"
	"time"
)

const mergedTimeout = 15 * time.Second

var defaultBaseNames = []string{"origin/main", "origin/master", "main", "master"}

// BaseRefsContext resolves the default branch to existing refs, remote first,
// then its local counterpart. An override beats gitsy.base config, which beats
// remote HEADs and common default names.
func BaseRefsContext(ctx context.Context, repoPath, override string) []string {
	listed := RunContext(ctx, repoPath, "for-each-ref", "--format=%(refname)%00%(symref)", "refs/heads", "refs/remotes")
	if !listed.OK {
		return nil
	}
	refs := map[string]bool{}
	heads := []string{}
	for _, line := range strings.Split(listed.Stdout, "\n") {
		name, symref, _ := strings.Cut(line, "\x00")
		refs[name] = true
		if strings.HasPrefix(name, "refs/remotes/") && strings.HasSuffix(name, "/HEAD") && symref != "" {
			if name == "refs/remotes/origin/HEAD" {
				heads = append([]string{shortRef(symref)}, heads...)
			} else {
				heads = append(heads, shortRef(symref))
			}
		}
	}

	name := strings.TrimSpace(override)
	if name == "" {
		name = strings.TrimSpace(RunContext(ctx, repoPath, "config", "--get", "gitsy.base").Stdout)
	}
	names := []string{name}
	if name == "" {
		names = append(heads, defaultBaseNames...)
	}
	for _, name := range names {
		remote, local := "", ""
		switch {
		case refs["refs/remotes/"+name]:
			remote = "refs/remotes/" + name
			if _, branch, ok := strings.Cut(name, "/"); ok {
				local = "refs/heads/" + branch
			}
		case refs["refs/heads/"+name]:
			remote, local = "refs/remotes/origin/"+name, "refs/heads/"+name
		default:
			continue
		}
		bases := []string{}
		for _, ref := range []string{remote, local} {
			if ref != "" && refs[ref] {
				bases = append(bases, ref)
			}
		}
		return bases
	}
	return nil
}

// MergedContext returns the short name of the first base that already contains
// HEAD's work. It tries ancestry, per-commit patch ids for rebases, a squashed
// patch id, then whether merging HEAD into the base would change nothing.
func MergedContext(ctx context.Context, repoPath string, bases []string) (string, Result) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, mergedTimeout)
	defer cancel()

	into, result := mergedBase(ctx, repoPath, bases)
	if !result.OK && ctx.Err() == context.DeadlineExceeded {
		result.Stderr = "merged check timed out"
	}
	return into, result
}

func mergedBase(ctx context.Context, repoPath string, bases []string) (string, Result) {
	head := RunContext(ctx, repoPath, "symbolic-ref", "-q", "HEAD")
	if !head.OK && head.Status != 1 {
		return "", head
	}
	branch := strings.TrimSpace(head.Stdout)
	for _, base := range bases {
		if branch != "" && strings.TrimPrefix(branch, "refs/heads/") == baseBranchName(base) {
			return "", Result{OK: true}
		}
	}
	seen := map[string]bool{}
	for _, base := range bases {
		commit := RunContext(ctx, repoPath, "rev-parse", "--verify", "--quiet", base+"^{commit}")
		if !commit.OK {
			return "", commit
		}
		if seen[commit.Stdout] {
			continue
		}
		seen[commit.Stdout] = true
		merged, result := mergedInto(ctx, repoPath, branch, base)
		if !result.OK {
			return "", result
		}
		if merged {
			return shortRef(base), result
		}
	}
	return "", Result{OK: true}
}

func mergedInto(ctx context.Context, repoPath, branch, base string) (bool, Result) {
	ancestor := RunContext(ctx, repoPath, "merge-base", "--is-ancestor", "HEAD", base)
	if ancestor.OK {
		// A branch never committed to is new work, not merged work.
		return branch == "" || !freshBranch(ctx, repoPath, branch), ancestor
	}
	if ancestor.Status != 1 {
		return false, ancestor
	}
	cherry := RunContext(ctx, repoPath, "cherry", base, "HEAD")
	if !cherry.OK {
		return false, cherry
	}
	if allUpstream(cherry.Stdout) {
		return true, cherry
	}
	squashed, result := squashMerged(ctx, repoPath, base)
	if !result.OK || squashed {
		return squashed, result
	}
	return contentMerged(ctx, repoPath, base)
}

func freshBranch(ctx context.Context, repoPath, branch string) bool {
	reflog := RunContext(ctx, repoPath, "reflog", "show", "--format=%H", branch, "--")
	return reflog.OK && len(strings.Fields(reflog.Stdout)) == 1
}

func allUpstream(cherry string) bool {
	lines := strings.Fields(cherry)
	if len(lines) == 0 {
		return false
	}
	for index := 0; index < len(lines); index += 2 {
		if lines[index] != "-" {
			return false
		}
	}
	return true
}

// squashMerged compares HEAD's combined patch with each base commit since they diverged.
func squashMerged(ctx context.Context, repoPath, base string) (bool, Result) {
	ok := Result{OK: true}
	mergeBase := RunContext(ctx, repoPath, "merge-base", base, "HEAD")
	if !mergeBase.OK {
		if mergeBase.Status == 1 {
			return false, ok
		}
		return false, mergeBase
	}
	fork := strings.TrimSpace(mergeBase.Stdout)
	patch := RunContext(ctx, repoPath, "diff-tree", "-p", "--no-color", fork, "HEAD")
	if !patch.OK || strings.TrimSpace(patch.Stdout) == "" {
		return false, patch
	}
	want := RunInputContext(ctx, repoPath, patch.Stdout, "patch-id", "--stable")
	if !want.OK {
		return false, want
	}
	wantID, _, _ := strings.Cut(strings.TrimSpace(want.Stdout), " ")
	if wantID == "" {
		return false, ok
	}
	commits := RunContext(ctx, repoPath, "rev-list", "--no-merges", fork+".."+base)
	if !commits.OK || strings.TrimSpace(commits.Stdout) == "" {
		return false, commits
	}
	patches := RunInputContext(ctx, repoPath, commits.Stdout, "diff-tree", "--stdin", "-p", "--no-color")
	if !patches.OK {
		return false, patches
	}
	ids := RunInputContext(ctx, repoPath, patches.Stdout, "patch-id", "--stable")
	if !ids.OK {
		return false, ids
	}
	for _, line := range strings.Split(ids.Stdout, "\n") {
		if id, _, _ := strings.Cut(line, " "); id == wantID {
			return true, ids
		}
	}
	return false, ids
}

// contentMerged reports whether merging HEAD into base leaves base's tree unchanged.
// Conflicts and Git versions without merge-tree --write-tree count as not merged.
func contentMerged(ctx context.Context, repoPath, base string) (bool, Result) {
	merged := RunContext(ctx, repoPath, "merge-tree", "--write-tree", "--no-messages", base, "HEAD")
	if !merged.OK {
		if ctx.Err() != nil {
			return false, merged
		}
		return false, Result{OK: true}
	}
	tree := RunContext(ctx, repoPath, "rev-parse", "--verify", base+"^{tree}")
	if !tree.OK {
		return false, tree
	}
	mergedTree, _, _ := strings.Cut(merged.Stdout, "\n")
	return mergedTree == strings.TrimSpace(tree.Stdout), tree
}

func shortRef(ref string) string {
	return strings.TrimPrefix(strings.TrimPrefix(ref, "refs/remotes/"), "refs/heads/")
}

func baseBranchName(ref string) string {
	if remote, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
		_, branch, _ := strings.Cut(remote, "/")
		return branch
	}
	return strings.TrimPrefix(ref, "refs/heads/")
}
