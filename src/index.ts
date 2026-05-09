#!/usr/bin/env node
import {readFileSync} from "node:fs";
import {render} from "ink";
import React from "react";
import {parseArgs, USAGE} from "./args.ts";
import {discoverRepos} from "./discover.ts";
import {getShortStatus} from "./git.ts";
import {parseStatus} from "./status.ts";
import {App} from "./ui/App.ts";
import type {RepoStatus} from "./ui/table.ts";

const h = React.createElement;

async function main(): Promise<void> {
  const parsedArgs = parseArgs(process.argv.slice(2));
  if (!parsedArgs.ok) {
    process.stderr.write(`gitsy: ${parsedArgs.error}\n\n${USAGE}`);
    process.exitCode = 1;
    return;
  }

  const options = parsedArgs.options;

  if (options.help) {
    process.stdout.write(USAGE);
    return;
  }

  if (options.version) {
    process.stdout.write(`gitsy ${readPackageVersion()}\n`);
    return;
  }

  if (options.fullscreen && (!process.stdin.isTTY || !process.stdout.isTTY)) {
    process.stderr.write("gitsy: --fullscreen requires an interactive TTY\n");
    process.exitCode = 1;
    return;
  }

  const warn = (message: string) => process.stderr.write(`gitsy: warning: ${message}\n`);
  const repos = discoverRepos({cwd: process.cwd(), maxDepth: options.maxDepth, verbose: options.verbose, warn});
  const repoStatuses: RepoStatus[] = [];

  for (const repo of repos) {
    const statusResult = getShortStatus(repo.path);
    if (!statusResult.ok) {
      if (options.verbose) {
        warn(`Failed to read status for ${repo.displayName}: ${statusResult.stderr.trim() || `git exited ${statusResult.status ?? "unknown"}`}`);
      }
      continue;
    }

    const status = parseStatus(statusResult.stdout);
    if (options.all || status.changed) {
      repoStatuses.push({repo, status});
    }
  }

  const message = createEmptyMessage(repos.length, repoStatuses.length, options.all);

  if (options.fullscreen) {
    await renderFullscreen(repoStatuses, options.all, repos.length, message);
    return;
  }

  const instance = render(
    h(App, {
      repos: repoStatuses,
      fullscreen: false,
      message,
      showAll: options.all,
      totalDiscovered: repos.length,
    }),
    {exitOnCtrlC: true},
  );
  await instance.waitUntilExit();
}

function createEmptyMessage(totalDiscovered: number, shown: number, showAll: boolean): string | undefined {
  if (totalDiscovered === 0) {
    return "No child git repositories found.";
  }

  if (shown === 0 && !showAll) {
    return "No child git repositories with changes or branch divergence found.";
  }

  return undefined;
}

async function renderFullscreen(repos: RepoStatus[], showAll: boolean, totalDiscovered: number, message: string | undefined): Promise<void> {
  let cleanedUp = false;

  const cleanup = () => {
    if (cleanedUp) {
      return;
    }
    cleanedUp = true;
    process.stdout.write("\u001B[?25h\u001B[?1049l");
  };

  const handleSigint = () => {
    cleanup();
    process.exit(130);
  };

  process.once("SIGINT", handleSigint);
  process.once("exit", cleanup);
  process.stdout.write("\u001B[?1049h\u001B[?25l");

  try {
    const instance = render(
      h(App, {
        repos,
        fullscreen: true,
        message,
        showAll,
        totalDiscovered,
      }),
      {exitOnCtrlC: false},
    );
    await instance.waitUntilExit();
  } finally {
    process.removeListener("SIGINT", handleSigint);
    cleanup();
  }
}

function readPackageVersion(): string {
  const packageJsonUrl = new URL("../package.json", import.meta.url);
  const packageJson = JSON.parse(readFileSync(packageJsonUrl, "utf8")) as {version?: string};
  return packageJson.version ?? "0.0.0";
}

main().catch((error: unknown) => {
  process.stderr.write(`gitsy: ${error instanceof Error ? error.stack ?? error.message : String(error)}\n`);
  process.exitCode = 1;
});
