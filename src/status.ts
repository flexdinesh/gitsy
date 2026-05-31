export type BranchStatus = {
  raw: string;
  name: string;
  upstream?: string | undefined;
  ahead: number;
  behind: number;
  gone: boolean;
  metadata?: string | undefined;
};

export type StatusCategory = "modified" | "staged" | "untracked" | "deleted" | "renamed" | "conflict" | "other";

export type StatusItem = {
  raw: string;
  code: string;
  path: string;
  category: StatusCategory;
};

export type ParsedStatus = {
  raw: string;
  branch?: BranchStatus | undefined;
  items: StatusItem[];
  changed: boolean;
};

const CONFLICT_CODES = new Set(["DD", "AU", "UD", "UA", "DU", "AA", "UU"]);

export function hasChangesOrDivergence(status: string): boolean {
  for (const line of status.split(/\r?\n/)) {
    if (line.startsWith("## ")) {
      if (line.includes("[") && line.includes("]")) {
        return true;
      }
      continue;
    }

    if (line !== "") {
      return true;
    }
  }

  return false;
}

export function canFastForward(status: ParsedStatus): boolean {
  const branch = status.branch;
  if (branch === undefined || branch.name === "") {
    return false;
  }

  return (
    branch.upstream !== undefined &&
    !branch.gone &&
    branch.behind > 0 &&
    branch.ahead === 0 &&
    status.items.length === 0
  );
}

export function parseStatus(status: string): ParsedStatus {
  const lines = status.split(/\r?\n/).filter((line) => line !== "");
  const branchLine = lines.find((line) => line.startsWith("## "));
  const branch = branchLine === undefined ? undefined : parseBranchLine(branchLine);
  const items = lines.filter((line) => !line.startsWith("## ")).map(parseStatusLine);

  return {
    raw: status,
    branch,
    items,
    changed: hasChangesOrDivergence(status),
  };
}

export function parseBranchLine(line: string): BranchStatus {
  const raw = line;
  const content = line.startsWith("## ") ? line.slice(3) : line;
  const metadataMatch = content.match(/\s\[(.+)]$/);
  const metadata = metadataMatch?.[1];
  const branchPart = metadataMatch === null ? content : content.slice(0, metadataMatch.index).trimEnd();
  const upstreamSeparator = branchPart.indexOf("...");
  const name = upstreamSeparator === -1 ? branchPart : branchPart.slice(0, upstreamSeparator);
  const upstream = upstreamSeparator === -1 ? undefined : branchPart.slice(upstreamSeparator + 3);

  return {
    raw,
    name,
    upstream: upstream === "" ? undefined : upstream,
    ahead: parseMetadataCount(metadata, "ahead"),
    behind: parseMetadataCount(metadata, "behind"),
    gone: metadata?.includes("gone") ?? false,
    metadata,
  };
}

export function parseStatusLine(line: string): StatusItem {
  const code = line.slice(0, 2);
  const filePath = line.length > 3 ? line.slice(3) : "";

  return {
    raw: line,
    code,
    path: filePath,
    category: categorizeStatus(code, filePath),
  };
}

function categorizeStatus(code: string, filePath: string): StatusCategory {
  if (CONFLICT_CODES.has(code) || code.includes("U")) {
    return "conflict";
  }

  if (code === "??") {
    return "untracked";
  }

  if (code.includes("R") || filePath.includes(" -> ")) {
    return "renamed";
  }

  if (code.includes("D")) {
    return "deleted";
  }

  const indexStatus = code[0] ?? " ";
  const worktreeStatus = code[1] ?? " ";

  if (indexStatus !== " " && indexStatus !== "?") {
    return "staged";
  }

  if (worktreeStatus !== " ") {
    return "modified";
  }

  return "other";
}

function parseMetadataCount(metadata: string | undefined, key: "ahead" | "behind"): number {
  if (metadata === undefined) {
    return 0;
  }

  const match = metadata.match(new RegExp(`${key} (\\d+)`));
  if (match?.[1] === undefined) {
    return 0;
  }

  return Number.parseInt(match[1], 10);
}
