import { spawn, spawnSync } from "node:child_process";

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
  return runGit(repoPath, ["status", "--short", "--branch", "--ahead-behind"]);
}

export function getWorktreeList(repoPath: string): GitResult {
  return runGit(repoPath, ["worktree", "list", "--porcelain"]);
}

export function fastForward(repoPath: string): GitResult {
  return runGit(repoPath, ["merge", "--ff-only"]);
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

export function fetchAll(repoPath: string, timeoutMs = 30_000): Promise<GitResult> {
  return new Promise((resolve) => {
    const child = spawn("git", ["-C", repoPath, "fetch", "--all"], {
      stdio: ["ignore", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";

    child.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString("utf8");
    });

    child.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString("utf8");
    });

    const timer = setTimeout(() => {
      child.kill("SIGKILL");
    }, timeoutMs);

    child.on("close", (status) => {
      clearTimeout(timer);
      resolve({
        ok: status === 0,
        stdout,
        stderr,
        status,
      });
    });

    child.on("error", (error) => {
      clearTimeout(timer);
      resolve({
        ok: false,
        stdout,
        stderr: error.message,
        status: null,
      });
    });
  });
}
