#!/usr/bin/env node
import {readFileSync} from "node:fs";
import {render} from "ink";
import React from "react";
import {parseArgs, USAGE} from "./args.ts";
import {discoverRepos} from "./discover.ts";
import {App} from "./ui/App.ts";

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
  const repos = discoverRepos({cwd: options.dir, maxDepth: options.maxDepth, verbose: options.verbose, warn});
  const message = createEmptyMessage(repos.length, 0, options.all);

  if (options.fullscreen) {
    await renderFullscreen(repos, options.all, options.noFetch, repos.length, message, warn);
    return;
  }

  const instance = render(
    h(App, {
      repos,
      fullscreen: false,
      noFetch: options.noFetch,
      message,
      showAll: options.all,
      totalDiscovered: repos.length,
      warn: options.verbose ? warn : undefined,
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

async function renderFullscreen(repos: ReturnType<typeof discoverRepos>, showAll: boolean, noFetch: boolean, totalDiscovered: number, message: string | undefined, warn?: (message: string) => void): Promise<void> {
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
        noFetch,
        message,
        showAll,
        totalDiscovered,
        warn,
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
