package gitsy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushChecksPushedCommits(t *testing.T) {
	requireGit(t)
	hook, err := os.ReadFile("scripts/pre-push.sh")
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(t.TempDir(), "repo with spaces")
	mkdirAll(t, filepath.Join(repo, "scripts"))
	writeFile(t, filepath.Join(repo, "scripts", "pre-push.sh"), string(hook))
	writeFile(t, filepath.Join(repo, "scripts", "mise"), `#!/bin/sh
set -eu
test -z "${GIT_DIR+x}"
test "$1" = -C
test "$3" = run
test "$4" = check
cat "$GITSY_CHECK_ROOT/result" >> "$GITSY_HOOK_TEST_LOG"
test "$(cat "$GITSY_CHECK_ROOT/result")" = pass
`)
	for _, path := range []string{"scripts/pre-push.sh", "scripts/mise"} {
		if err := os.Chmod(filepath.Join(repo, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repo, "init", "-q", "-b", "main")
	configureGitUser(t, repo)
	writeFile(t, filepath.Join(repo, "result"), "pass\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "passing")
	pass := gitObject(t, repo, "HEAD")
	runGit(t, repo, "tag", "-am", "passing tag", "v1")
	tag := gitObject(t, repo, "v1")
	writeFile(t, filepath.Join(repo, "result"), "fail\n")
	runGit(t, repo, "commit", "-qam", "failing")
	fail := gitObject(t, repo, "HEAD")
	writeFile(t, filepath.Join(repo, "result"), "pass\n")
	zero := strings.Repeat("0", len(pass))
	ref := func(name, oid string) string {
		return name + " " + oid + " refs/heads/main " + zero + "\n"
	}
	tests := []struct {
		name string
		refs string
		log  string
		fail bool
	}{
		{name: "pushed commit despite different HEAD", refs: ref("refs/heads/older", pass), log: "pass\n"},
		{name: "dirty files cannot mask failure", refs: ref("refs/heads/main", fail), log: "fail\n", fail: true},
		{name: "duplicate commits and annotated tag", refs: ref("refs/heads/older", pass) + ref("refs/heads/other", pass) + ref("refs/tags/v1", tag), log: "pass\n"},
		{name: "all distinct commits", refs: ref("refs/heads/older", pass) + ref("refs/heads/main", fail), log: "pass\nfail\n", fail: true},
		{name: "stop on failure", refs: ref("refs/heads/main", fail) + ref("refs/heads/older", pass), log: "fail\n", fail: true},
		{name: "deletion", refs: ref("(delete)", zero)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "checks")
			temp := t.TempDir()
			writeFile(t, log, "")
			command := exec.Command(filepath.Join(repo, "scripts", "pre-push.sh"), "origin", "unused")
			command.Dir = repo
			command.Env = append(os.Environ(), "GITSY_HOOK_TEST_LOG="+log, "TMPDIR="+temp, "GIT_DIR="+filepath.Join(repo, ".git"), "PATH="+filepath.Join(repo, "scripts")+string(os.PathListSeparator)+os.Getenv("PATH"))
			command.Stdin = strings.NewReader(test.refs)
			output, err := command.CombinedOutput()
			if (err != nil) != test.fail {
				t.Fatalf("unexpected hook result: err=%v output=%s", err, output)
			}
			got, err := os.ReadFile(log)
			if err != nil || string(got) != test.log {
				t.Fatalf("expected checks %q, got %q: %v", test.log, got, err)
			}
			got, err = os.ReadFile(filepath.Join(repo, "result"))
			if err != nil || string(got) != "pass\n" {
				t.Fatalf("hook must preserve working files: %q, %v", got, err)
			}
			entries, err := os.ReadDir(temp)
			if err != nil || len(entries) != 0 {
				t.Fatalf("hook must remove temporary snapshots: %v, %v", entries, err)
			}
		})
	}
}

func gitObject(t *testing.T, repo, ref string) string {
	t.Helper()
	output, err := exec.Command("git", "-C", repo, "rev-parse", ref).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}
