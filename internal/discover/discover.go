package discover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flexdinesh/gitsy/internal/git"
)

type RepoSource string

const (
	SourceScan     RepoSource = "scan"
	SourceWorktree RepoSource = "worktree"
)

type Repo struct {
	Path        string
	RealPath    string
	DisplayName string
	Source      RepoSource
}

type Options struct {
	Cwd      string
	MaxDepth int
	Verbose  bool
	Warn     func(message string)
}

var IgnoredDirNames = map[string]struct{}{
	"node_modules":      {},
	"dist":              {},
	"build":             {},
	"public":            {},
	".gradle":           {},
	".idea":             {},
	".vscode":           {},
	"target":            {},
	"coverage":          {},
	".next":             {},
	".nuxt":             {},
	".cache":            {},
	".terraform":        {},
	".turbo":            {},
	".parcel-cache":     {},
	"vendor":            {},
	"out":               {},
	"tmp":               {},
	"temp":              {},
	"__pycache__":       {},
	".venv":             {},
	"venv":              {},
	".mypy_cache":       {},
	".pytest_cache":     {},
	".tox":              {},
	".yarn":             {},
	".pnpm-store":       {},
	".svelte-kit":       {},
	".angular":          {},
	".serverless":       {},
	".wrangler":         {},
	".netlify":          {},
	".vercel":           {},
	".expo":             {},
	".docusaurus":       {},
	".storybook-static": {},
	".astro":            {},
	".remix":            {},
	".output":           {},
	".cache-loader":     {},
	".rustup":           {},
	".cargo":            {},
	"Pods":              {},
	"DerivedData":       {},
	"bin":               {},
	"obj":               {},
	"logs":              {},
	"log":               {},
}

func FindGitCandidates(cwd string, maxDepth int, ignoredDirNames map[string]struct{}) []string {
	root, err := filepath.Abs(cwd)
	if err != nil {
		root = filepath.Clean(cwd)
	}
	if ignoredDirNames == nil {
		ignoredDirNames = IgnoredDirNames
	}

	candidates := map[string]struct{}{}
	var walk func(directory string, depth int)
	walk = func(directory string, depth int) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return
		}

		for _, entry := range entries {
			entryPath := filepath.Join(directory, entry.Name())

			if entry.Name() == ".git" {
				if depth > 0 && depth <= maxDepth && isGitMarker(entryPath) {
					candidates[directory] = struct{}{}
				}
				continue
			}

			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if _, ignored := ignoredDirNames[entry.Name()]; ignored {
				continue
			}
			if depth < maxDepth {
				walk(entryPath, depth+1)
			}
		}
	}

	walk(root, 0)

	result := make([]string, 0, len(candidates))
	for candidate := range candidates {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		return DisplayNameForPath(root, result[i]) < DisplayNameForPath(root, result[j])
	})
	return result
}

func Discover(options Options) []Repo {
	return DiscoverContext(context.Background(), options)
}

func DiscoverContext(ctx context.Context, options Options) []Repo {
	cwd, err := filepath.Abs(options.Cwd)
	if err != nil {
		cwd = filepath.Clean(options.Cwd)
	}

	warn := createWarner(options)
	reposByRealPath := map[string]Repo{}

	for _, candidate := range FindGitCandidates(cwd, options.MaxDepth, nil) {
		verified, ok := verifyRepo(ctx, candidate, cwd, SourceScan, warn)
		if ok {
			reposByRealPath[verified.RealPath] = verified
		}
	}

	scannedRepos := make([]Repo, 0, len(reposByRealPath))
	for _, repo := range reposByRealPath {
		scannedRepos = append(scannedRepos, repo)
	}

	for _, repo := range scannedRepos {
		result := git.WorktreeListContext(ctx, repo.Path)
		if !result.OK {
			warn(fmt.Sprintf("Failed to list worktrees for %s: %s", repo.DisplayName, gitError(result)))
			continue
		}

		for _, worktreePath := range git.ParseWorktreePaths(result.Stdout) {
			verified, ok := verifyRepo(ctx, worktreePath, cwd, SourceWorktree, warn)
			if ok {
				if _, exists := reposByRealPath[verified.RealPath]; !exists {
					reposByRealPath[verified.RealPath] = verified
				}
			}
		}
	}

	repos := make([]Repo, 0, len(reposByRealPath))
	for _, repo := range reposByRealPath {
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].DisplayName < repos[j].DisplayName
	})
	return repos
}

func DisplayNameForPath(cwd string, repoPath string) string {
	root, err := filepath.Abs(cwd)
	if err != nil {
		root = filepath.Clean(cwd)
	}
	absoluteRepo, err := filepath.Abs(repoPath)
	if err != nil {
		absoluteRepo = filepath.Clean(repoPath)
	}

	relative, err := filepath.Rel(root, absoluteRepo)
	if err == nil {
		if relative == "." {
			return "."
		}
		if !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != ".." && !filepath.IsAbs(relative) {
			return relative
		}
	}
	return absoluteRepo
}

func verifyRepo(ctx context.Context, repoPath string, cwd string, source RepoSource, warn func(message string)) (Repo, bool) {
	stats, err := os.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			warn(fmt.Sprintf("Skipping missing repo path: %s", repoPath))
		} else {
			warn(fmt.Sprintf("Skipping inaccessible repo path %s: %s", repoPath, err.Error()))
		}
		return Repo{}, false
	}
	if !stats.IsDir() {
		warn(fmt.Sprintf("Skipping non-directory repo path: %s", repoPath))
		return Repo{}, false
	}

	repoRealPath, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		warn(fmt.Sprintf("Skipping inaccessible repo path %s: %s", repoPath, err.Error()))
		return Repo{}, false
	}

	topLevel := git.TopLevelContext(ctx, repoPath)
	if !topLevel.OK {
		warn(fmt.Sprintf("Skipping invalid git repo %s: %s", DisplayNameForPath(cwd, repoPath), gitError(topLevel)))
		return Repo{}, false
	}

	topLevelPath := strings.TrimSpace(topLevel.Stdout)
	topLevelRealPath, err := filepath.EvalSymlinks(topLevelPath)
	if err != nil {
		warn(fmt.Sprintf("Skipping repo %s with inaccessible top-level %s: %s", DisplayNameForPath(cwd, repoPath), topLevelPath, err.Error()))
		return Repo{}, false
	}

	if filepath.Clean(repoRealPath) != filepath.Clean(topLevelRealPath) {
		warn(fmt.Sprintf("Skipping nested git directory %s; top-level is %s", DisplayNameForPath(cwd, repoPath), topLevelPath))
		return Repo{}, false
	}

	return Repo{
		Path:        repoPath,
		RealPath:    repoRealPath,
		DisplayName: DisplayNameForPath(cwd, repoPath),
		Source:      source,
	}, true
}

func isGitMarker(path string) bool {
	stats, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stats.IsDir() || stats.Mode().IsRegular()
}

func createWarner(options Options) func(message string) {
	return func(message string) {
		if !options.Verbose {
			return
		}
		if options.Warn != nil {
			options.Warn(message)
		}
	}
}

func gitError(result git.Result) string {
	stderr := strings.TrimSpace(result.Stderr)
	if stderr != "" {
		return stderr
	}
	return fmt.Sprintf("git exited %d", result.Status)
}
