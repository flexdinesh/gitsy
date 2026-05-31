import React, {useEffect, useMemo, useState} from "react";
import {Box, Text, useApp, useInput, useStdout} from "ink";
import ora from "ora";
import type {DiscoveredRepo} from "../discover.ts";
import {fetchAll, fastForward, getShortStatus} from "../git.ts";
import {canFastForward, parseStatus} from "../status.ts";
import type {ParsedStatus} from "../status.ts";
import type {RepoFetchState, TableLayout, VisualRow} from "./table.ts";
import {
  bottomBorder,
  buildVisualRowsFromFetchState,
  createTableLayout,
  formatCell,
  headerDivider,
  separator,
  titleLine,
  topBorder,
} from "./table.ts";

const h = React.createElement;

export type AppProps = {
  repos: readonly DiscoveredRepo[];
  fullscreen: boolean;
  noFetch: boolean;
  sync: boolean;
  message: string | undefined;
  showAll: boolean;
  totalDiscovered: number;
  warn?: ((message: string) => void) | undefined;
};

export function App(props: AppProps): React.ReactElement {
  const {exit} = useApp();
  const {stdout} = useStdout();
  const spinner = useMemo(() => ora({text: "fetching…", color: false}), []);
  const [spinnerTick, setSpinnerTick] = useState(0);
  const [scrollOffset, setScrollOffset] = useState(0);
  const [fetchStates, setFetchStates] = useState<Record<string, RepoFetchState>>({});

  // Initialise every repo as fetching
  useEffect(() => {
    const initial: Record<string, RepoFetchState> = {};
    for (const repo of props.repos) {
      initial[repo.realPath] = {kind: "fetching"};
    }
    setFetchStates(initial);
  }, [props.repos]);

  // Kick off async fetch + status for every repo
  useEffect(() => {
    if (props.repos.length === 0) return;

    for (const repo of props.repos) {
      if (props.noFetch) {
        runStatusOnly(repo);
      } else {
        fetchAll(repo.path).then((fetchResult) => {
          if (!fetchResult.ok && props.warn !== undefined) {
            props.warn(
              `Fetch failed for ${repo.displayName}: ${fetchResult.stderr.trim() || `git exited ${fetchResult.status ?? "unknown"}`}`,
            );
          }
          runStatusAfterFetch(repo);
        });
      }
    }

    function runStatusOnly(repo: DiscoveredRepo): void {
      const statusResult = getShortStatus(repo.path);
      updateState(repo, statusResult);
    }

    function runStatusAfterFetch(repo: DiscoveredRepo): void {
      const statusResult = getShortStatus(repo.path);
      updateState(repo, statusResult);
    }

    function updateState(repo: DiscoveredRepo, statusResult: ReturnType<typeof getShortStatus>): void {
      if (!statusResult.ok) {
        if (props.warn !== undefined) {
          props.warn(
            `Failed to read status for ${repo.displayName}: ${statusResult.stderr.trim() || `git exited ${statusResult.status ?? "unknown"}`}`,
          );
        }
        setFetchStates((prev) => ({
          ...prev,
          [repo.realPath]: {kind: "failed", status: parseStatus("")},
        }));
        return;
      }
      const status = parseStatus(statusResult.stdout);

      if (props.sync && canFastForward(status)) {
        syncRepo(repo, status);
        return;
      }

      setFetchStates((prev) => ({
        ...prev,
        [repo.realPath]: {kind: "done", status},
      }));
    }

    function syncRepo(repo: DiscoveredRepo, preSyncStatus: ParsedStatus): void {
      const pulled = preSyncStatus.branch?.behind ?? 0;
      setFetchStates((prev) => ({
        ...prev,
        [repo.realPath]: {kind: "syncing"},
      }));

      const ffResult = fastForward(repo.path);
      if (!ffResult.ok) {
        if (props.warn !== undefined) {
          props.warn(
            `Sync failed for ${repo.displayName}: ${ffResult.stderr.trim() || `git exited ${ffResult.status ?? "unknown"}`}`,
          );
        }
        setFetchStates((prev) => ({
          ...prev,
          [repo.realPath]: {
            kind: "done",
            status: preSyncStatus,
            sync: {kind: "failed", reason: ffResult.stderr.trim()},
          },
        }));
        return;
      }

      const postStatus = parseStatus(getShortStatus(repo.path).stdout);
      setFetchStates((prev) => ({
        ...prev,
        [repo.realPath]: {
          kind: "done",
          status: postStatus,
          sync: {kind: "synced", pulled},
        },
      }));
    }
  }, [props.repos, props.noFetch, props.sync, props.warn]);

  // In static mode, exit once every repo has reached done or failed
  useEffect(() => {
    if (props.fullscreen) return undefined;
    if (props.repos.length === 0) {
      const timer = setTimeout(() => exit(), 0);
      return () => clearTimeout(timer);
    }
    const allComplete = props.repos.every((repo) => {
      const state = fetchStates[repo.realPath];
      return state !== undefined && state.kind !== "fetching" && state.kind !== "syncing";
    });
    if (allComplete) {
      const timer = setTimeout(() => exit(), 50);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [props.fullscreen, props.repos, fetchStates, exit]);

  const terminalHeight = stdout.rows ?? 24;
  const stateMap = useMemo(() => {
    const map = new Map<string, RepoFetchState>();
    for (const [key, value] of Object.entries(fetchStates)) {
      map.set(key, value);
    }
    return map;
  }, [fetchStates]);
  const hasFetching = hasFetchingRepos(props.repos, stateMap);
  const fetchingText = useMemo(() => (hasFetching ? spinner.frame() : "fetching…"), [hasFetching, spinner, spinnerTick]);

  useEffect(() => {
    if (!hasFetching) {
      return undefined;
    }

    const timer = setInterval(() => setSpinnerTick((current) => current + 1), spinner.interval);
    return () => clearInterval(timer);
  }, [hasFetching, spinner]);

  const layout = useMemo(() => createTableLayout(stdout.columns, props.repos), [stdout.columns, props.repos]);
  const rows = useMemo(() => buildVisualRowsFromFetchState(props.repos, stateMap, props.showAll, fetchingText), [props.repos, stateMap, props.showAll, fetchingText]);
  const viewportHeight = props.fullscreen ? Math.max(1, terminalHeight - 8) : rows.length;
  const maxScroll = Math.max(0, rows.length - viewportHeight);

  useEffect(() => {
    setScrollOffset((current) => Math.min(current, maxScroll));
  }, [maxScroll]);

  useInput(
    (input, key) => {
      if (!props.fullscreen) {
        return;
      }

      const keyInfo = key as typeof key & Record<string, boolean | undefined>;
      const pageSize = Math.max(1, viewportHeight - 1);

      if (input === "q" || key.escape || (key.ctrl && input === "c")) {
        exit();
        return;
      }

      if (key.upArrow || input === "k") {
        setScrollOffset((current) => Math.max(0, current - 1));
        return;
      }

      if (key.downArrow || input === "j") {
        setScrollOffset((current) => Math.min(maxScroll, current + 1));
        return;
      }

      if (keyInfo.pageUp || input === "u") {
        setScrollOffset((current) => Math.max(0, current - pageSize));
        return;
      }

      if (keyInfo.pageDown || input === "d") {
        setScrollOffset((current) => Math.min(maxScroll, current + pageSize));
        return;
      }

      if (keyInfo.home || input === "g") {
        setScrollOffset(0);
        return;
      }

      if (keyInfo.end || input === "G") {
        setScrollOffset(maxScroll);
      }
    },
    {isActive: props.fullscreen},
  );

  const visibleRows = props.fullscreen ? rows.slice(scrollOffset, scrollOffset + viewportHeight) : rows;
  const shownCount = countVisibleRepos(props.repos, stateMap, props.showAll);
  const title = createTitle(props.totalDiscovered, shownCount, props.showAll, props.fullscreen, scrollOffset, maxScroll);

  return h(
    Box,
    {flexDirection: "column"},
    h(Text, {color: "gray"}, topBorder(layout)),
    h(Text, {color: "cyan", bold: true}, titleLine(layout, title)),
    h(Text, {color: "gray"}, headerDivider(layout)),
    h(HeaderRow, {layout}),
    h(Text, {color: "gray"}, headerDivider(layout)),
    rows.length === 0
      ? h(EmptyRow, {layout, message: props.message ?? "No repositories to display."})
      : visibleRows.map((row, index) => h(TableRow, {key: `${index}-${row.kind}-${scrollOffset}`, layout, row})),
    h(Text, {color: "gray"}, bottomBorder(layout)),
    props.fullscreen
      ? h(Text, {dimColor: true}, "↑/k ↓/j page u/d home g end G quit q/esc")
      : undefined,
  );
}

function countVisibleRepos(repos: readonly DiscoveredRepo[], states: Map<string, RepoFetchState>, showAll: boolean): number {
  let count = 0;
  for (const repo of repos) {
    const state = states.get(repo.realPath);
    if (state === undefined || state.kind === "fetching" || state.kind === "syncing") {
      count += 1;
      continue;
    }
    if (state.status.changed || showAll || (state.kind === "done" && state.sync?.kind === "synced")) {
      count += 1;
    }
  }
  return count;
}

function hasFetchingRepos(repos: readonly DiscoveredRepo[], states: Map<string, RepoFetchState>): boolean {
  for (const repo of repos) {
    const state = states.get(repo.realPath);
    if (state === undefined || state.kind === "fetching" || state.kind === "syncing") {
      return true;
    }
  }

  return false;
}

function HeaderRow(props: {layout: TableLayout}): React.ReactElement {
  return h(Text, {bold: true}, dataLine(props.layout, "REPO", "STATUS"));
}

function EmptyRow(props: {layout: TableLayout; message: string}): React.ReactElement {
  return h(TableRow, {
    layout: props.layout,
    row: {kind: "data", repo: "", text: props.message, color: "yellow"},
  });
}

function TableRow(props: {layout: TableLayout; row: VisualRow}): React.ReactElement {
  if (props.row.kind === "separator") {
    return h(Text, {color: "gray"}, separator(props.layout));
  }

  return h(
    Text,
    {color: props.row.color, bold: props.row.bold === true, dimColor: props.row.dim === true},
    dataLine(props.layout, props.row.repo, props.row.text),
  );
}

function dataLine(layout: TableLayout, repo: string, status: string): string {
  return `│ ${formatCell(repo, layout.repoWidth)} │ ${formatCell(status, layout.statusWidth)} │`;
}

function createTitle(totalDiscovered: number, shown: number, showAll: boolean, fullscreen: boolean, offset: number, maxScroll: number): string {
  const filter = showAll ? "all repos" : "changed repos";
  const mode = fullscreen ? "fullscreen" : "static";
  const scroll = maxScroll > 0 ? ` • scroll ${offset + 1}/${maxScroll + 1}` : "";
  return `gitsy • ${shown}/${totalDiscovered} ${filter} • ${mode}${scroll}`;
}
