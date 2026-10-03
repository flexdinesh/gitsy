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
	Dirs     []string
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

func FindGitCandidates(cwd string, maxDepth int, ignoredDirNames map[string]struct{}) ([]string, error) {
	return findGitCandidates(context.Background(), cwd, maxDepth, ignoredDirNames, func(string) {})
}

func findGitCandidates(ctx context.Context, cwd string, maxDepth int, ignoredDirNames map[string]struct{}, warn func(string)) ([]string, error) {
	root, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve scan directory: %w", err)
	}
	if ignoredDirNames == nil {
		ignoredDirNames = IgnoredDirNames
	}

	candidates := map[string]struct{}{}
	var walk func(directory string, depth int) error
	walk = func(directory string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			if depth == 0 {
				return fmt.Errorf("read scan directory %s: %w", directory, err)
			}
			warn(fmt.Sprintf("Skipping inaccessible directory %s: %s", directory, err))
			return nil
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
				if err := walk(entryPath, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(root, 0); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(candidates))
	for candidate := range candidates {
		result = append(result, candidate)
	}
	sort.Strings(result)
	return result, nil
}

func Discover(options Options) ([]Repo, error) {
	return DiscoverContext(context.Background(), options)
}

func DiscoverContext(ctx context.Context, options Options) ([]Repo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cwd, err := filepath.Abs(options.Cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve scan directory: %w", err)
	}

	roots := make([]string, 0, len(options.Dirs))
	for _, dir := range options.Dirs {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		roots = append(roots, filepath.Clean(dir))
	}
	if len(roots) == 0 {
		roots = append(roots, cwd)
	}
	warn := createWarner(options)
	reposByRealPath := map[string]Repo{}

	for _, root := range roots {
		candidates, err := findGitCandidates(ctx, root, options.MaxDepth, nil, warn)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			verified, ok := verifyRepo(ctx, candidate, SourceScan, warn)
			if ok {
				if _, exists := reposByRealPath[verified.RealPath]; !exists {
					reposByRealPath[verified.RealPath] = verified
				}
			}
		}
	}

	scannedRepos := make([]Repo, 0, len(reposByRealPath))
	for _, repo := range reposByRealPath {
		scannedRepos = append(scannedRepos, repo)
	}

	for _, repo := range scannedRepos {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result := git.WorktreeListContext(ctx, repo.Path)
		if !result.OK {
			warn(fmt.Sprintf("Failed to list worktrees for %s: %s", repo.DisplayName, gitError(result)))
			continue
		}

		for _, worktreePath := range git.ParseWorktreePaths(result.Stdout) {
			verified, ok := verifyRepo(ctx, worktreePath, SourceWorktree, warn)
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].DisplayName == repos[j].DisplayName {
			return repos[i].Path < repos[j].Path
		}
		return repos[i].DisplayName < repos[j].DisplayName
	})
	return repos, nil
}

func DisplayNameForPath(repoPath string) string {
	absoluteRepo, err := filepath.Abs(repoPath)
	if err != nil {
		absoluteRepo = filepath.Clean(repoPath)
	}

	return displayPath(filepath.Base(absoluteRepo))
}

func displayPath(path string) string {
	return strings.NewReplacer("\r", "\\r", "\n", "\\n", "\t", "\\t").Replace(path)
}

func verifyRepo(ctx context.Context, repoPath string, source RepoSource, warn func(message string)) (Repo, bool) {
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
		warn(fmt.Sprintf("Skipping invalid git repo %s: %s", DisplayNameForPath(repoPath), gitError(topLevel)))
		return Repo{}, false
	}

	topLevelPath := strings.TrimSuffix(topLevel.Stdout, "\n")
	topLevelRealPath, err := filepath.EvalSymlinks(topLevelPath)
	if err != nil {
		warn(fmt.Sprintf("Skipping repo %s with inaccessible top-level %s: %s", DisplayNameForPath(repoPath), topLevelPath, err.Error()))
		return Repo{}, false
	}

	if filepath.Clean(repoRealPath) != filepath.Clean(topLevelRealPath) {
		warn(fmt.Sprintf("Skipping nested git directory %s; top-level is %s", DisplayNameForPath(repoPath), topLevelPath))
		return Repo{}, false
	}

	return Repo{
		Path:        repoPath,
		RealPath:    repoRealPath,
		DisplayName: DisplayNameForPath(repoPath),
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
