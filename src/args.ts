export type CliOptions = {
  all: boolean;
  maxDepth: number;
  fullscreen: boolean;
  verbose: boolean;
  help: boolean;
  version: boolean;
};

export type ParseResult =
  | { ok: true; options: CliOptions }
  | { ok: false; error: string };

export const DEFAULT_MAX_DEPTH = 3;

export const USAGE = `Usage: gitsy [options]

Show git status across child repositories and linked worktrees.

Options:
  --all              Show all discovered repositories, including clean repos
  --max-depth <n>    Scan repository directories up to n nested levels (default: 3)
  --fullscreen       Open a scrollable fullscreen terminal UI
  --verbose          Print warnings for skipped repos and failed git commands
  --help             Show this help message
  --version          Show package version
`;

export function parseArgs(argv: readonly string[]): ParseResult {
  const options: CliOptions = {
    all: false,
    maxDepth: DEFAULT_MAX_DEPTH,
    fullscreen: false,
    verbose: false,
    help: false,
    version: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];

    if (arg === "--") {
      continue;
    }

    if (arg === "--all") {
      options.all = true;
      continue;
    }

    if (arg === "--fullscreen") {
      options.fullscreen = true;
      continue;
    }

    if (arg === "--verbose") {
      options.verbose = true;
      continue;
    }

    if (arg === "--help") {
      options.help = true;
      continue;
    }

    if (arg === "--version") {
      options.version = true;
      continue;
    }

    if (arg === "--max-depth") {
      const value = argv[index + 1];
      if (value === undefined || value.startsWith("--")) {
        return { ok: false, error: "Missing value for --max-depth" };
      }
      const parsed = parseMaxDepth(value);
      if (parsed === undefined) {
        return { ok: false, error: `Invalid --max-depth value: ${value}` };
      }
      options.maxDepth = parsed;
      index += 1;
      continue;
    }

    if (arg?.startsWith("--max-depth=")) {
      const value = arg.slice("--max-depth=".length);
      const parsed = parseMaxDepth(value);
      if (parsed === undefined) {
        return { ok: false, error: `Invalid --max-depth value: ${value}` };
      }
      options.maxDepth = parsed;
      continue;
    }

    return { ok: false, error: `Unknown argument: ${arg}` };
  }

  return { ok: true, options };
}

function parseMaxDepth(value: string): number | undefined {
  if (!/^\d+$/.test(value)) {
    return undefined;
  }

  const parsed = Number.parseInt(value, 10);
  if (!Number.isSafeInteger(parsed) || parsed < 1) {
    return undefined;
  }

  return parsed;
}
