import test from "node:test";
import assert from "node:assert/strict";
import {spawnSync} from "node:child_process";
import {mkdtempSync, mkdirSync, rmSync, writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import path from "node:path";
import {discoverRepos} from "../src/discover.ts";
import {getShortStatus} from "../src/git.ts";
import {parseStatus} from "../src/status.ts";

function gitAvailable(): boolean {
  return spawnSync("git", ["--version"], {encoding: "utf8"}).status === 0;
}

function runGit(cwd: string, args: readonly string[]): void {
  const result = spawnSync("git", ["-C", cwd, ...args], {encoding: "utf8"});
  assert.equal(result.status, 0, result.stderr || result.stdout);
}

function withTempDir(fn: (directory: string) => void): void {
  const directory = mkdtempSync(path.join(tmpdir(), "gitsy-"));
  try {
    fn(directory);
  } finally {
    rmSync(directory, {recursive: true, force: true});
  }
}

test("discovers clean child repos and filters status by changed flag", {skip: !gitAvailable()}, () => {
  withTempDir((directory) => {
    const repo = path.join(directory, "repo");
    mkdirSync(repo);
    runGit(repo, ["init"]);

    const repos = discoverRepos({cwd: directory, maxDepth: 3});
    assert.equal(repos.length, 1);
    assert.equal(repos[0]?.displayName, "repo");

    const cleanStatus = parseStatus(getShortStatus(repo).stdout);
    assert.equal(cleanStatus.changed, false);

    writeFileSync(path.join(repo, "README.md"), "hello\n");
    const dirtyStatus = parseStatus(getShortStatus(repo).stdout);
    assert.equal(dirtyStatus.changed, true);
  });
});

test("discovers linked worktrees from a discovered repo", {skip: !gitAvailable()}, () => {
  withTempDir((directory) => {
    const repo = path.join(directory, "repo");
    const worktree = path.join(directory, "linked-worktree");
    mkdirSync(repo);
    runGit(repo, ["init"]);
    runGit(repo, ["config", "user.email", "gitsy@example.com"]);
    runGit(repo, ["config", "user.name", "Gitsy Test"]);
    writeFileSync(path.join(repo, "README.md"), "hello\n");
    runGit(repo, ["add", "README.md"]);
    runGit(repo, ["commit", "-m", "initial"]);
    runGit(repo, ["worktree", "add", "-b", "feature", worktree]);

    const repos = discoverRepos({cwd: directory, maxDepth: 3});
    const names = repos.map((discovered) => discovered.displayName).sort();

    assert.deepEqual(names, ["linked-worktree", "repo"]);
  });
});
