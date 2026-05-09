import React, {useEffect, useMemo, useState} from "react";
import {Box, Text, useApp, useInput, useStdout} from "ink";
import type {RepoStatus, TableLayout, VisualRow} from "./table.ts";
import {
  bottomBorder,
  buildVisualRows,
  createTableLayout,
  formatCell,
  headerDivider,
  separator,
  titleLine,
  topBorder,
} from "./table.ts";

const h = React.createElement;

export type AppProps = {
  repos: readonly RepoStatus[];
  fullscreen: boolean;
  message: string | undefined;
  showAll: boolean;
  totalDiscovered: number;
};

export function App(props: AppProps): React.ReactElement {
  const {exit} = useApp();
  const {stdout} = useStdout();
  const [scrollOffset, setScrollOffset] = useState(0);
  const terminalHeight = stdout.rows ?? 24;
  const layout = useMemo(() => createTableLayout(stdout.columns, props.repos), [stdout.columns, props.repos]);
  const rows = useMemo(() => buildVisualRows(props.repos), [props.repos]);
  const viewportHeight = props.fullscreen ? Math.max(1, terminalHeight - 8) : rows.length;
  const maxScroll = Math.max(0, rows.length - viewportHeight);

  useEffect(() => {
    if (!props.fullscreen) {
      const timer = setTimeout(() => exit(), 0);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [exit, props.fullscreen]);

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
  const title = createTitle(props.totalDiscovered, props.repos.length, props.showAll, props.fullscreen, scrollOffset, maxScroll);

  return h(
    Box,
    {flexDirection: "column"},
    h(Text, {color: "gray"}, topBorder(layout)),
    h(Text, {color: "cyan", bold: true}, titleLine(layout, title)),
    h(Text, {color: "gray"}, headerDivider(layout)),
    h(HeaderRow, {layout}),
    h(Text, {color: "gray"}, headerDivider(layout)),
    props.message !== undefined || props.repos.length === 0
      ? h(EmptyRow, {layout, message: props.message ?? "No repositories to display."})
      : visibleRows.map((row, index) => h(TableRow, {key: `${index}-${row.kind}-${scrollOffset}`, layout, row})),
    h(Text, {color: "gray"}, bottomBorder(layout)),
    props.fullscreen
      ? h(Text, {dimColor: true}, "↑/k ↓/j page u/d home g end G quit q/esc")
      : undefined,
  );
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
