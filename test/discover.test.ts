import test from "node:test";
import assert from "node:assert/strict";
import {mkdtempSync, mkdirSync, rmSync, writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import path from "node:path";
import {findGitCandidates, displayNameForPath} from "../src/discover.ts";
import {parseWorktreePaths} from "../src/git.ts";

function withTempDir(fn: (directory: string) => void): void {
  const directory = mkdtempSync(path.join(tmpdir(), "gitsy-"));
  try {
    fn(directory);
  } finally {
    rmSync(directory, {recursive: true, force: true});
  }
}

test("findGitCandidates finds .git directories and skips cwd root .git", () => {
  withTempDir((directory) => {
    mkdirSync(path.join(directory, ".git"));
    mkdirSync(path.join(directory, "repo", ".git"), {recursive: true});

    const candidates = findGitCandidates({cwd: directory, maxDepth: 3});

    assert.deepEqual(candidates.map((candidate) => path.relative(directory, candidate)), ["repo"]);
  });
});

test("findGitCandidates finds .git files for worktree-like directories", () => {
  withTempDir((directory) => {
    mkdirSync(path.join(directory, "worktree"));
    writeFileSync(path.join(directory, "worktree", ".git"), "gitdir: /tmp/example/.git/worktrees/worktree\n");

    const candidates = findGitCandidates({cwd: directory, maxDepth: 3});

    assert.deepEqual(candidates.map((candidate) => path.relative(directory, candidate)), ["worktree"]);
  });
});

test("findGitCandidates respects repository directory max depth", () => {
  withTempDir((directory) => {
    mkdirSync(path.join(directory, "a", "b", "repo", ".git"), {recursive: true});
    mkdirSync(path.join(directory, "a", "b", "c", "too-deep", ".git"), {recursive: true});

    const candidates = findGitCandidates({cwd: directory, maxDepth: 3});

    assert.deepEqual(candidates.map((candidate) => path.relative(directory, candidate)), [path.join("a", "b", "repo")]);
  });
});

test("findGitCandidates ignores generated directories", () => {
  withTempDir((directory) => {
    mkdirSync(path.join(directory, "node_modules", "repo", ".git"), {recursive: true});
    mkdirSync(path.join(directory, "dist", "repo", ".git"), {recursive: true});
    mkdirSync(path.join(directory, "real", ".git"), {recursive: true});

    const candidates = findGitCandidates({cwd: directory, maxDepth: 3});

    assert.deepEqual(candidates.map((candidate) => path.relative(directory, candidate)), ["real"]);
  });
});

test("parseWorktreePaths parses porcelain worktree entries", () => {
  assert.deepEqual(
    parseWorktreePaths("worktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/feature\nHEAD def456\n"),
    ["/repo/main", "/repo/feature"],
  );
});

test("displayNameForPath uses relative names inside cwd and absolute paths outside", () => {
  withTempDir((directory) => {
    assert.equal(displayNameForPath(directory, path.join(directory, "repo")), "repo");
    assert.equal(displayNameForPath(directory, path.dirname(directory)), path.resolve(path.dirname(directory)));
  });
});
