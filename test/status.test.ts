import test from "node:test";
import assert from "node:assert/strict";
import {hasChangesOrDivergence, parseStatus} from "../src/status.ts";

test("clean branch-only status is unchanged", () => {
  const parsed = parseStatus("## main...origin/main\n");
  assert.equal(parsed.changed, false);
  assert.equal(hasChangesOrDivergence(parsed.raw), false);
  assert.equal(parsed.branch?.name, "main");
  assert.equal(parsed.branch?.upstream, "origin/main");
});

test("ahead and behind branch status counts as changed", () => {
  const parsed = parseStatus("## main...origin/main [ahead 1, behind 2]\n");
  assert.equal(parsed.changed, true);
  assert.equal(parsed.branch?.ahead, 1);
  assert.equal(parsed.branch?.behind, 2);
});

test("gone upstream counts as changed", () => {
  const parsed = parseStatus("## feature...origin/feature [gone]\n");
  assert.equal(parsed.changed, true);
  assert.equal(parsed.branch?.gone, true);
});

test("parses modified file", () => {
  const parsed = parseStatus("## main\n M src/index.ts\n");
  assert.equal(parsed.changed, true);
  assert.equal(parsed.items[0]?.category, "modified");
});

test("parses staged file", () => {
  const parsed = parseStatus("## main\nA  README.md\n");
  assert.equal(parsed.items[0]?.category, "staged");
});

test("parses untracked file", () => {
  const parsed = parseStatus("## main\n?? README.md\n");
  assert.equal(parsed.items[0]?.category, "untracked");
});

test("parses deleted file", () => {
  const parsed = parseStatus("## main\n D old.ts\n");
  assert.equal(parsed.items[0]?.category, "deleted");
});

test("parses renamed file", () => {
  const parsed = parseStatus("## main\nR  old.ts -> new.ts\n");
  assert.equal(parsed.items[0]?.category, "renamed");
});

test("parses conflict", () => {
  const parsed = parseStatus("## main\nUU conflicted.ts\n");
  assert.equal(parsed.items[0]?.category, "conflict");
});
