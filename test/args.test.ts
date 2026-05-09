import test from "node:test";
import assert from "node:assert/strict";
import {parseArgs} from "../src/args.ts";

test("parseArgs returns defaults", () => {
  const result = parseArgs([]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.deepEqual(result.options, {
    all: false,
    maxDepth: 3,
    fullscreen: false,
    verbose: false,
    help: false,
    version: false,
  });
});

test("parseArgs supports flags", () => {
  const result = parseArgs(["--all", "--fullscreen", "--verbose", "--help", "--version"]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.options.all, true);
  assert.equal(result.options.fullscreen, true);
  assert.equal(result.options.verbose, true);
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

test("parseArgs rejects unknown args", () => {
  assert.equal(parseArgs(["--raw"]).ok, false);
});
