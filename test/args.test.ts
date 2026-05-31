import test from "node:test";
import assert from "node:assert/strict";
import {parseArgs} from "../src/args.ts";

test("parseArgs returns defaults", () => {
  const result = parseArgs([]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.all, false);
  assert.equal(result.options.maxDepth, 3);
  assert.equal(result.options.fullscreen, false);
  assert.equal(result.options.verbose, false);
  assert.equal(result.options.noFetch, false);
  assert.equal(result.options.help, false);
  assert.equal(result.options.version, false);
  assert.equal(result.options.sync, false);
  assert.equal(typeof result.options.dir, "string");
});

test("parseArgs supports flags", () => {
  const result = parseArgs(["--all", "--fullscreen", "--verbose", "--no-fetch", "--sync", "--help", "--version"]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.all, true);
  assert.equal(result.options.fullscreen, true);
  assert.equal(result.options.verbose, true);
  assert.equal(result.options.noFetch, true);
  assert.equal(result.options.sync, true);
  assert.equal(result.options.help, true);
  assert.equal(result.options.version, true);
});

test("parseArgs supports --max-depth value", () => {
  const result = parseArgs(["--max-depth", "5"]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.maxDepth, 5);
});

test("parseArgs supports --max-depth=value", () => {
  const result = parseArgs(["--max-depth=7"]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.maxDepth, 7);
});

test("parseArgs rejects invalid max depth", () => {
  assert.equal(parseArgs(["--max-depth", "0"]).ok, false);
  assert.equal(parseArgs(["--max-depth", "abc"]).ok, false);
  assert.equal(parseArgs(["--max-depth"]).ok, false);
});

test("parseArgs supports --dir value", () => {
  const result = parseArgs(["--dir", "/some/path"]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.dir, "/some/path");
});

test("parseArgs supports --dir=value", () => {
  const result = parseArgs(["--dir=/another/path"]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.dir, "/another/path");
});

test("parseArgs rejects missing --dir value", () => {
  assert.equal(parseArgs(["--dir"]).ok, false);
  assert.equal(parseArgs(["--dir", "--all"]).ok, false);
  assert.equal(parseArgs(["--dir="]).ok, false);
});

test("parseArgs rejects unknown args", () => {
  assert.equal(parseArgs(["--raw"]).ok, false);
});
