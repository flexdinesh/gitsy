import stringWidth from "string-width";
import type {DiscoveredRepo} from "../discover.ts";
import type {ParsedStatus, StatusCategory, StatusItem} from "../status.ts";
import {CATEGORY_STYLES, type InkColor} from "./theme.ts";

export type RepoStatus = {
  repo: DiscoveredRepo;
  status: ParsedStatus;
};

export type RepoFetchState =
  | {kind: "fetching"}
  | {kind: "done"; status: ParsedStatus}
  | {kind: "failed"; status: ParsedStatus};

export type VisualRow =
  | {kind: "separator"}
  | {kind: "data"; repo: string; text: string; color: InkColor; bold?: boolean | undefined; dim?: boolean | undefined};

export type TableLayout = {
  width: number;
  repoWidth: number;
  statusWidth: number;
};

export function createTableLayout(terminalWidth: number | undefined, repos: readonly DiscoveredRepo[]): TableLayout {
  const width = Math.max(60, Math.min(terminalWidth ?? 80, 140));
  const longestRepoName = Math.max(4, ...repos.map((repo) => stringWidth(repo.displayName)));
  const repoWidth = clamp(longestRepoName, 18, Math.floor(width * 0.35));
  const statusWidth = Math.max(24, width - repoWidth - 7);

  return {
    width: repoWidth + statusWidth + 7,
    repoWidth,
    statusWidth,
  };
}

export function buildVisualRows(repos: readonly RepoStatus[]): VisualRow[] {
  const rows: VisualRow[] = [];

  repos.forEach((repoStatus, index) => {
    if (index > 0) {
      rows.push({kind: "separator"});
    }

    const branchSummary = formatBranchSummary(repoStatus.status);
    rows.push({
      kind: "data",
      repo: repoStatus.repo.displayName,
      text: branchSummary.text,
      color: branchSummary.color,
      bold: true,
      dim: branchSummary.dim,
    });

    if (repoStatus.status.items.length === 0) {
      if (!repoStatus.status.changed) {
        rows.push({kind: "data", repo: "", text: "✓ clean", color: "green", dim: true});
      }
      return;
    }

    for (const item of repoStatus.status.items) {
      rows.push(formatItemRow(item));
    }
  });

  return rows;
}

export function buildVisualRowsFromFetchState(
  repos: readonly DiscoveredRepo[],
  states: ReadonlyMap<string, RepoFetchState>,
  showAll: boolean,
): VisualRow[] {
  const rows: VisualRow[] = [];
  let hasVisibleRow = false;

  for (const repo of repos) {
    const state = states.get(repo.realPath);
    const repoRows = buildRowsForRepo(repo, state, showAll);

    if (repoRows.length === 0) {
      continue;
    }

    if (hasVisibleRow) {
      rows.push({kind: "separator"});
    }

    rows.push(...repoRows);
    hasVisibleRow = true;
  }

  return rows;
}

function buildRowsForRepo(
  repo: DiscoveredRepo,
  state: RepoFetchState | undefined,
  showAll: boolean,
): VisualRow[] {
  if (state === undefined || state.kind === "fetching") {
    return [
      {
        kind: "data",
        repo: repo.displayName,
        text: "⏳ fetching…",
        color: "yellow",
        dim: true,
      },
    ];
  }

  const status = state.status;
  const isStale = state.kind === "failed";

  if (!status.changed && !showAll) {
    return [];
  }

  const rows: VisualRow[] = [];
  const branchSummary = formatBranchSummary(status);

  if (isStale) {
    branchSummary.text += " ⚠ stale";
  }

  rows.push({
    kind: "data",
    repo: repo.displayName,
    text: branchSummary.text,
    color: branchSummary.color,
    bold: true,
    dim: branchSummary.dim,
  });

  if (status.items.length === 0) {
    if (!status.changed) {
      rows.push({kind: "data", repo: "", text: "✓ clean", color: "green", dim: true});
    }
    return rows;
  }

  for (const item of status.items) {
    rows.push(formatItemRow(item));
  }

  return rows;
}

export function topBorder(layout: TableLayout): string {
  return `╭${"─".repeat(layout.repoWidth + 2)}┬${"─".repeat(layout.statusWidth + 2)}╮`;
}

export function headerDivider(layout: TableLayout): string {
  return `├${"─".repeat(layout.repoWidth + 2)}┼${"─".repeat(layout.statusWidth + 2)}┤`;
}

export function bottomBorder(layout: TableLayout): string {
  return `╰${"─".repeat(layout.repoWidth + 2)}┴${"─".repeat(layout.statusWidth + 2)}╯`;
}

export function separator(layout: TableLayout): string {
  return `├${"─".repeat(layout.repoWidth + 2)}┼${"─".repeat(layout.statusWidth + 2)}┤`;
}

export function titleLine(layout: TableLayout, text: string): string {
  const innerWidth = layout.repoWidth + layout.statusWidth + 3;
  return `│ ${padEndVisible(truncateVisible(text, innerWidth), innerWidth)} │`;
}

export function formatCell(value: string, width: number): string {
  return padEndVisible(truncateVisible(value, width), width);
}

export function truncateVisible(value: string, maxWidth: number): string {
  if (stringWidth(value) <= maxWidth) {
    return value;
  }

  if (maxWidth <= 1) {
    return "…";
  }

  let output = "";
  for (const char of value) {
    if (stringWidth(`${output}${char}…`) > maxWidth) {
      break;
    }
    output += char;
  }

  return `${output}…`;
}

export function padEndVisible(value: string, width: number): string {
  return `${value}${" ".repeat(Math.max(0, width - stringWidth(value)))}`;
}

export function formatBranchSummary(status: ParsedStatus): {text: string; color: InkColor; dim?: boolean} {
  const branch = status.branch;
  if (branch === undefined) {
    return status.changed ? {text: "changes", color: "yellow"} : {text: "✓ clean", color: "green", dim: true};
  }

  const parts = [branch.name || "detached"];
  if (branch.ahead > 0) {
    parts.push(`↑${branch.ahead}`);
  }
  if (branch.behind > 0) {
    parts.push(`↓${branch.behind}`);
  }
  if (branch.gone) {
    parts.push("⚠ upstream gone");
  }
  if (branch.metadata !== undefined && branch.ahead === 0 && branch.behind === 0 && !branch.gone) {
    parts.push(`[${branch.metadata}]`);
  }

  if (!status.changed) {
    parts.push("✓ clean");
    return {text: parts.join(" "), color: "green", dim: true};
  }

  return {text: parts.join(" "), color: branch.gone ? "yellow" : "blue"};
}

function formatItemRow(item: StatusItem): VisualRow {
  const style = CATEGORY_STYLES[item.category];
  return {
    kind: "data",
    repo: "",
    text: `${style.icon} ${style.label} ${formatItemPath(item)}`,
    color: style.color,
    bold: style.bold,
  };
}

function formatItemPath(item: StatusItem): string {
  if (item.category === "renamed") {
    return item.path.replace(" -> ", " → ");
  }

  return item.path || item.raw;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}
