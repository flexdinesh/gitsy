import test from "node:test";
import assert from "node:assert/strict";
import type {DiscoveredRepo} from "../src/discover.ts";
import {buildVisualRowsFromFetchState} from "../src/ui/table.ts";

const repo: DiscoveredRepo = {
  path: "/tmp/repo",
  realPath: "/tmp/repo",
  displayName: "repo",
  source: "scan",
};

test("fetching rows use provided spinner text", () => {
  const rows = buildVisualRowsFromFetchState([repo], new Map(), false, "- fetching…");

  assert.deepEqual(rows, [
    {
      kind: "data",
      repo: "repo",
      text: "- fetching…",
      color: "yellow",
      dim: true,
    },
  ]);
});
