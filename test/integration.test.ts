import test from "node:test";
import assert from "node:assert/strict";
import {spawnSync} from "node:child_process";
import {mkdtempSync, mkdirSync, rmSync, writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import path from "node:path";
import {discoverRepos} from "../src/discover.ts";
import {fastForward, getShortStatus} from "../src/git.ts";
import {canFastForward, parseStatus} from "../src/status.ts";

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

test("CLI --no-fetch renders repo status without fetching", {skip: !gitAvailable()}, () => {
  withTempDir((directory) => {
    const repo = path.join(directory, "repo");
    mkdirSync(repo);
    runGit(repo, ["init"]);
    writeFileSync(path.join(repo, "README.md"), "hello\n");

    const cliPath = path.resolve(import.meta.dirname, "..", "src", "index.ts");
    const result = spawnSync("node", [cliPath, "--no-fetch"], {
      cwd: directory,
      encoding: "utf8",
      env: {...process.env, FORCE_COLOR: "0", CI: "true"},
    });

    assert.equal(result.status, 0, result.stderr || result.stdout);
    assert.ok(result.stdout.includes("repo"), `Expected output to include repo name: ${result.stdout}`);
  });
});

test("--sync safely fast-forwards a clone that is strictly behind", {skip: !gitAvailable()}, () => {
  withTempDir((directory) => {
    const origin = path.join(directory, "origin");
    const clone = path.join(directory, "clone");
    mkdirSync(origin);
    runGit(origin, ["init"]);
    runGit(origin, ["config", "user.email", "gitsy@example.com"]);
    runGit(origin, ["config", "user.name", "Gitsy Test"]);
    writeFileSync(path.join(origin, "README.md"), "one\n");
    runGit(origin, ["add", "README.md"]);
    runGit(origin, ["commit", "-m", "first"]);

    runGit(directory, ["clone", origin, clone]);
    runGit(clone, ["config", "user.email", "gitsy@example.com"]);
    runGit(clone, ["config", "user.name", "Gitsy Test"]);

    writeFileSync(path.join(origin, "README.md"), "one\ntwo\n");
    runGit(origin, ["add", "README.md"]);
    runGit(origin, ["commit", "-m", "second"]);

    runGit(clone, ["fetch"]);
    const before = parseStatus(getShortStatus(clone).stdout);
    assert.equal(before.branch?.behind, 1);
    assert.equal(before.branch?.ahead, 0);
    assert.equal(canFastForward(before), true);

    const ff = fastForward(clone);
    assert.equal(ff.ok, true, ff.stderr);

    const after = parseStatus(getShortStatus(clone).stdout);
    assert.equal(after.branch?.behind, 0);
    assert.equal(canFastForward(after), false);
  });
});

test("--sync refuses to fast-forward a diverged clone", {skip: !gitAvailable()}, () => {
  withTempDir((directory) => {
    const origin = path.join(directory, "origin");
    const clone = path.join(directory, "clone");
    mkdirSync(origin);
    runGit(origin, ["init"]);
    runGit(origin, ["config", "user.email", "gitsy@example.com"]);
    runGit(origin, ["config", "user.name", "Gitsy Test"]);
    writeFileSync(path.join(origin, "README.md"), "one\n");
    runGit(origin, ["add", "README.md"]);
    runGit(origin, ["commit", "-m", "first"]);

    runGit(directory, ["clone", origin, clone]);
    runGit(clone, ["config", "user.email", "gitsy@example.com"]);
    runGit(clone, ["config", "user.name", "Gitsy Test"]);

    writeFileSync(path.join(origin, "README.md"), "one\norigin\n");
    runGit(origin, ["add", "README.md"]);
    runGit(origin, ["commit", "-m", "origin change"]);

    writeFileSync(path.join(clone, "LOCAL.md"), "local\n");
    runGit(clone, ["add", "LOCAL.md"]);
    runGit(clone, ["commit", "-m", "local change"]);

    runGit(clone, ["fetch"]);
    const status = parseStatus(getShortStatus(clone).stdout);
    assert.equal(status.branch?.ahead, 1);
    assert.equal(status.branch?.behind, 1);
    assert.equal(canFastForward(status), false);

    const ff = fastForward(clone);
    assert.equal(ff.ok, false);
  });
});
