import {spawnSync} from "node:child_process";

export type GitResult = {
  ok: boolean;
  stdout: string;
  stderr: string;
  status: number | null;
};

export function runGit(cwd: string, args: readonly string[]): GitResult {
  const result = spawnSync("git", ["-C", cwd, ...args], {
    encoding: "utf8",
    maxBuffer: 1024 * 1024 * 20,
  });

  return {
    ok: result.status === 0,
    stdout: result.stdout ?? "",
    stderr: result.stderr ?? result.error?.message ?? "",
    status: result.status,
  };
}

export function getTopLevel(repoPath: string): GitResult {
  return runGit(repoPath, ["rev-parse", "--show-toplevel"]);
}

export function getShortStatus(repoPath: string): GitResult {
  return runGit(repoPath, ["status", "--short", "--branch", "--renames", "--ahead-behind"]);
}

export function getWorktreeList(repoPath: string): GitResult {
  return runGit(repoPath, ["worktree", "list", "--porcelain"]);
}

export function parseWorktreePaths(porcelain: string): string[] {
  const paths: string[] = [];

  for (const line of porcelain.split(/\r?\n/)) {
    if (line.startsWith("worktree ")) {
      paths.push(line.slice("worktree ".length));
    }
  }

  return paths;
}
