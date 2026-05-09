import {existsSync, readdirSync, realpathSync, statSync} from "node:fs";
import path from "node:path";
import {getTopLevel, getWorktreeList, parseWorktreePaths} from "./git.ts";

export type RepoSource = "scan" | "worktree";

export type DiscoveredRepo = {
  path: string;
  realPath: string;
  displayName: string;
  source: RepoSource;
};

export type DiscoverOptions = {
  cwd: string;
  maxDepth: number;
  verbose?: boolean;
  warn?: (message: string) => void;
};

export const IGNORED_DIR_NAMES = new Set([
  "node_modules",
  "dist",
  "build",
  "public",
  ".gradle",
  ".idea",
  ".vscode",
  "target",
  "coverage",
  ".next",
  ".nuxt",
  ".cache",
  ".terraform",
  ".turbo",
  ".parcel-cache",
  "vendor",
  "out",
  "tmp",
  "temp",
  "__pycache__",
  ".venv",
  "venv",
  ".mypy_cache",
  ".pytest_cache",
  ".tox",
  ".yarn",
  ".pnpm-store",
  ".svelte-kit",
  ".angular",
  ".serverless",
  ".wrangler",
  ".netlify",
  ".vercel",
  ".expo",
  ".docusaurus",
  ".storybook-static",
  ".astro",
  ".remix",
  ".output",
  ".cache-loader",
  ".rustup",
  ".cargo",
  "Pods",
  "DerivedData",
  "bin",
  "obj",
  "logs",
  "log",
]);

export function findGitCandidates(options: {
  cwd: string;
  maxDepth: number;
  ignoredDirNames?: ReadonlySet<string>;
}): string[] {
  const cwd = path.resolve(options.cwd);
  const ignoredDirNames = options.ignoredDirNames ?? IGNORED_DIR_NAMES;
  const candidates = new Set<string>();

  function walk(directory: string, depth: number): void {
    let entries;
    try {
      entries = readdirSync(directory, {withFileTypes: true});
    } catch {
      return;
    }

    for (const entry of entries) {
      const entryPath = path.join(directory, entry.name);

      if (entry.name === ".git") {
        if (depth > 0 && depth <= options.maxDepth && (entry.isDirectory() || entry.isFile())) {
          candidates.add(directory);
        }
        continue;
      }

      if (!entry.isDirectory() || entry.isSymbolicLink()) {
        continue;
      }

      if (ignoredDirNames.has(entry.name)) {
        continue;
      }

      if (depth < options.maxDepth) {
        walk(entryPath, depth + 1);
      }
    }
  }

  walk(cwd, 0);

  return [...candidates].sort((left, right) => displayNameForPath(cwd, left).localeCompare(displayNameForPath(cwd, right)));
}

export function discoverRepos(options: DiscoverOptions): DiscoveredRepo[] {
  const cwd = path.resolve(options.cwd);
  const reposByRealPath = new Map<string, DiscoveredRepo>();
  const warn = createWarner(options);

  for (const candidate of findGitCandidates({cwd, maxDepth: options.maxDepth})) {
    const verified = verifyRepo(candidate, cwd, "scan", warn);
    if (verified !== undefined) {
      reposByRealPath.set(verified.realPath, verified);
    }
  }

  const scannedRepos = [...reposByRealPath.values()];
  for (const repo of scannedRepos) {
    const result = getWorktreeList(repo.path);
    if (!result.ok) {
      warn(`Failed to list worktrees for ${repo.displayName}: ${result.stderr.trim() || `git exited ${result.status ?? "unknown"}`}`);
      continue;
    }

    for (const worktreePath of parseWorktreePaths(result.stdout)) {
      const verified = verifyRepo(worktreePath, cwd, "worktree", warn);
      if (verified !== undefined && !reposByRealPath.has(verified.realPath)) {
        reposByRealPath.set(verified.realPath, verified);
      }
    }
  }

  return [...reposByRealPath.values()].sort((left, right) => left.displayName.localeCompare(right.displayName));
}

function verifyRepo(repoPath: string, cwd: string, source: RepoSource, warn: (message: string) => void): DiscoveredRepo | undefined {
  if (!existsSync(repoPath)) {
    warn(`Skipping missing repo path: ${repoPath}`);
    return undefined;
  }

  let repoRealPath: string;
  try {
    const stats = statSync(repoPath);
    if (!stats.isDirectory()) {
      warn(`Skipping non-directory repo path: ${repoPath}`);
      return undefined;
    }
    repoRealPath = realpathSync(repoPath);
  } catch (error) {
    warn(`Skipping inaccessible repo path ${repoPath}: ${formatError(error)}`);
    return undefined;
  }

  const topLevel = getTopLevel(repoPath);
  if (!topLevel.ok) {
    warn(`Skipping invalid git repo ${displayNameForPath(cwd, repoPath)}: ${topLevel.stderr.trim() || `git exited ${topLevel.status ?? "unknown"}`}`);
    return undefined;
  }

  const topLevelPath = topLevel.stdout.trim();
  let topLevelRealPath: string;
  try {
    topLevelRealPath = realpathSync(topLevelPath);
  } catch (error) {
    warn(`Skipping repo ${displayNameForPath(cwd, repoPath)} with inaccessible top-level ${topLevelPath}: ${formatError(error)}`);
    return undefined;
  }

  if (repoRealPath !== topLevelRealPath) {
    warn(`Skipping nested git directory ${displayNameForPath(cwd, repoPath)}; top-level is ${topLevelPath}`);
    return undefined;
  }

  return {
    path: repoPath,
    realPath: repoRealPath,
    displayName: displayNameForPath(cwd, repoPath),
    source,
  };
}

export function displayNameForPath(cwd: string, repoPath: string): string {
  const relativePath = path.relative(path.resolve(cwd), path.resolve(repoPath));

  if (relativePath === "") {
    return ".";
  }

  if (!relativePath.startsWith("..") && !path.isAbsolute(relativePath)) {
    return relativePath;
  }

  return path.resolve(repoPath);
}

function createWarner(options: DiscoverOptions): (message: string) => void {
  return (message: string) => {
    if (options.verbose !== true) {
      return;
    }
    if (options.warn !== undefined) {
      options.warn(message);
      return;
    }
    process.stderr.write(`gitsy: warning: ${message}\n`);
  };
}

function formatError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
