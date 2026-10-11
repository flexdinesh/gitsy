package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchKeyGroupsWorktreesAndAliasesButNotClones(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	mainPath, linked := filepath.Join(root, "main"), filepath.Join(root, "linked\n")
	clone, alias := filepath.Join(root, "clone"), filepath.Join(root, "alias")
	mustFetchGit(t, root, "init", "-q", "-b", "main", mainPath)
	mustFetchGit(t, mainPath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "initial")
	mustFetchGit(t, mainPath, "worktree", "add", "-q", "-b", "feature", linked)
	mustFetchGit(t, root, "clone", "-q", mainPath, clone)
	if err := os.Symlink(mainPath, alias); err != nil {
		t.Fatal(err)
	}
	key := FetchKeyContext(context.Background(), mainPath)
	if key == "" {
		t.Fatal("ordinary repository must allow shared fetching")
	}
	for _, path := range []string{linked, alias} {
		if got := FetchKeyContext(context.Background(), path); got != key {
			t.Fatalf("worktree/alias %q must share repository identity", path)
		}
	}
	if got := FetchKeyContext(context.Background(), clone); got == "" || got == key {
		t.Fatal("separate clone must have its own fetch identity")
	}
	mustFetchGit(t, mainPath, "config", "remote.origin.url", filepath.Join(root, "different-remote"))
	if got := FetchKeyContext(context.Background(), mainPath); got == "" || got == key {
		t.Fatal("changed effective configuration must not reuse a fetch")
	}
}

func TestFetchKeyFallsBackForCheckoutSpecificBehavior(t *testing.T) {
	for _, scenario := range []string{"worktree config", "conditional include", "untracked gitmodules", "deleted gitmodules", "submodule config", "modules directory", "relative remote", "URL rewrite", "SSH command", "credential helper", "remote helper", "SSH environment", "relative TLS config", "relative TLS environment", "relative fsck skiplist", "relative bundle URI", "custom hooks path", "reference hook", "maintenance hook", "invalid repo", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			repo := t.TempDir()
			mustFetchGit(t, repo, "init", "-q", "-b", "main")
			ctx := context.Background()
			switch scenario {
			case "worktree config":
				mustFetchGit(t, repo, "config", "extensions.worktreeConfig", "true")
			case "conditional include":
				mustFetchGit(t, repo, "config", "includeIf.onbranch:other.path", filepath.Join(repo, "unused-config"))
			case "untracked gitmodules", "deleted gitmodules":
				modules := filepath.Join(repo, ".gitmodules")
				if err := os.WriteFile(modules, []byte(""), 0o600); err != nil {
					t.Fatal(err)
				}
				if scenario == "deleted gitmodules" {
					mustFetchGit(t, repo, "add", ".gitmodules")
					if err := os.Remove(modules); err != nil {
						t.Fatal(err)
					}
				}
			case "submodule config":
				mustFetchGit(t, repo, "config", "submodule.example.url", "../example")
			case "modules directory":
				if err := os.Mkdir(filepath.Join(repo, ".git", "modules"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "relative remote":
				mustFetchGit(t, repo, "config", "remote.origin.url", "../origin")
			case "URL rewrite":
				mustFetchGit(t, repo, "config", "url.../origin.insteadOf", "https://example.com/origin")
			case "SSH command":
				mustFetchGit(t, repo, "config", "core.sshCommand", "ssh -i ./identity")
			case "credential helper":
				mustFetchGit(t, repo, "config", "credential.helper", "!./helper")
			case "remote helper":
				mustFetchGit(t, repo, "config", "remote.origin.url", "ext::./helper")
			case "SSH environment":
				t.Setenv("GIT_SSH_COMMAND", "ssh -i ./identity")
			case "relative TLS config":
				mustFetchGit(t, repo, "config", "http.https://example.com/.sslCAInfo", "./ca.pem")
			case "relative TLS environment":
				t.Setenv("GIT_SSL_CAINFO", "./ca.pem")
			case "relative fsck skiplist":
				mustFetchGit(t, repo, "config", "fetch.fsck.skipList", "./skiplist")
			case "relative bundle URI":
				mustFetchGit(t, repo, "config", "fetch.bundleURI", "./bundles")
			case "custom hooks path":
				mustFetchGit(t, repo, "config", "core.hooksPath", ".githooks")
			case "reference hook", "maintenance hook":
				name := "reference-transaction"
				if scenario == "maintenance hook" {
					name = "pre-auto-gc"
				}
				if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", name), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "invalid repo":
				repo = t.TempDir()
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if key := FetchKeyContext(ctx, repo); key != "" {
				t.Fatal("uncertain fetch behavior must use per-checkout fetching")
			}
		})
	}
}

func TestCheckoutIndependentURLs(t *testing.T) {
	for _, test := range []struct {
		url  string
		want bool
	}{
		{"/absolute/repo", true},
		{"file:///absolute/repo", true},
		{"https://example.com/repo", true},
		{"ssh://git@example.com/repo", true},
		{"git://example.com/repo", true},
		{"git@example.com:repo", true},
		{"[::1]:repo", false},
		{"../origin", false},
		{"./repo:with-colon", false},
		{"repo", false},
		{"ext::./helper", false},
		{"custom://repo", false},
		{"file://relative", false},
	} {
		t.Run(test.url, func(t *testing.T) {
			if got := checkoutIndependentURL(test.url); got != test.want {
				t.Fatalf("URL %q sharing safe=%t, want %t", test.url, got, test.want)
			}
		})
	}
}

func mustFetchGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	if result := Run(cwd, args...); !result.OK {
		t.Fatalf("git %v: %s", args, result.Stderr)
	}
}
