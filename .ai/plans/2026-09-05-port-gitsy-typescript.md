# Plan: Port `gitsy.sh` to a native TypeScript + Ink CLI

## Summary

Create a new pnpm-based TypeScript CLI project in this repo that ports `../cli/gitsy.sh` into a richer `gitsy` command using Node 24 native TypeScript execution and Ink for terminal UI rendering.

The CLI will discover git repos and worktrees under the current directory, show pretty parsed git status, support fullscreen scrolling, and default to showing only repos with changes/divergence unless `--all` is passed.

---

## Key implementation changes

### 1. Project setup

Add a new native TypeScript Node project:

```text
package.json
pnpm-lock.yaml
tsconfig.json
.node-version
src/index.ts
src/args.ts
src/discover.ts
src/git.ts
src/status.ts
src/ui/App.ts
src/ui/table.ts
src/ui/theme.ts
test/*.test.ts
```

Use:

- `pnpm`
- Node `24.14.1` / latest local Node
- native TypeScript execution via `node src/index.ts`
- no JS build output required

`package.json` metadata:

```json
{
  "name": "gitsy",
  "version": "0.1.0",
  "description": "Show git status across child repositories and worktrees"
}
```

Add CLI bin:

```json
{
  "bin": {
    "gitsy": "./src/index.ts"
  }
}
```

Likely dependencies:

```text
ink
react
string-width
```

Likely dev dependencies:

```text
typescript
@types/node
@types/react
```

---

### 2. CLI options

Support:

```bash
gitsy
gitsy --all
gitsy --max-depth 5
gitsy --fullscreen
gitsy --verbose
gitsy --help
gitsy --version
```

Behavior:

- `--all`: show all discovered repos/worktrees.
- no `--all`: show only repos/worktrees with changes or branch divergence.
- `--max-depth <n>`: scan repo directories up to `n` nested path segments below cwd.
  - Default: `3`
  - Includes:
    - `./repo/.git`
    - `./team/repo/.git`
    - `./team/project/repo/.git`
- `--fullscreen`: use an alternate-screen interactive UI with scroll controls.
- `--verbose`: print warnings for invalid repos, inaccessible paths, failed git commands, etc.
- `--help`: print usage.
- `--version`: read version from `package.json`.

Invalid args should exit with code `1` and print usage.

---

### 3. Repo and worktree discovery

Implement filesystem discovery in TypeScript.

Rules:

- Start at `process.cwd()`.
- Look for both:
  - `.git` directories
  - `.git` files, for worktrees/submodules
- Do not include the current repo’s root `.git`.
- Do not follow symlinks.
- Sort discovered repos by display path.
- Verify candidates with:

```bash
git -C <repo> rev-parse --show-toplevel
```

- Compare realpaths to ensure the candidate path is the repo top-level.
- Silently skip invalid candidates unless `--verbose` is enabled.

Hardcode the original ignored dirs plus additional common generated/cache dirs.

Original list to preserve:

```text
node_modules
dist
build
public
.gradle
.idea
.vscode
target
coverage
.next
.nuxt
.cache
.terraform
.turbo
.parcel-cache
vendor
out
tmp
temp
__pycache__
.venv
venv
.mypy_cache
.pytest_cache
.tox
```

Additional list:

```text
.yarn
.pnpm-store
.svelte-kit
.angular
.serverless
.wrangler
.netlify
.vercel
.expo
.docusaurus
.storybook-static
.astro
.remix
.output
.cache-loader
.rustup
.cargo
Pods
DerivedData
bin
obj
logs
log
```

---

### 4. Linked worktree discovery

For each discovered repo, also run:

```bash
git -C <repo> worktree list --porcelain
```

Parse `worktree <path>` entries and include those paths as additional repo entries.

Deduplicate by realpath.

Display names:

- If path is inside cwd: use cwd-relative path.
- If path is outside cwd: use absolute path.

This intentionally allows linked worktrees outside the scanned directory to appear.

---

### 5. Git status collection

For every discovered repo/worktree, run:

```bash
git -C <repo> status --short --branch --renames --ahead-behind
```

Use the original Bash behavior for filtering:

- Branch line with `[ahead]`, `[behind]`, `[gone]`, etc. counts as divergence/change.
- Any non-empty non-branch status line counts as a change.
- Clean branch-only output does not count as changed.
- With `--all`, include clean repos too.

---

### 6. Pretty parsed status

Parse raw git status into a richer model for Ink rendering.

Examples:

```text
## main...origin/main [ahead 1]
 M src/index.ts
?? README.md
 D old.ts
R  old.ts -> new.ts
UU conflicted.ts
```

Render as a prettier summary:

```text
main ↑1
● modified src/index.ts
+ untracked README.md
✖ deleted old.ts
➜ renamed old.ts → new.ts
‼ conflict conflicted.ts
```

Use Unicode icons by default:

- modified: `●`
- staged: `◆`
- untracked: `+`
- deleted: `✖`
- renamed: `➜`
- conflict: `‼`
- ahead: `↑`
- behind: `↓`
- upstream gone: `⚠`

Clean repos shown with `--all` should render dim/green as clean.

---

### 7. Ink UI

Use Ink for all rendering.

#### Default mode

```bash
gitsy
```

- Render a pretty static Ink table.
- Exit after rendering.
- Use normal terminal scrollback.

#### Fullscreen mode

```bash
gitsy --fullscreen
```

- Enter terminal alternate screen.
- Render a scrollable viewport.
- Keep header/footer visible.
- Support keyboard controls:
  - `↑` / `k`: scroll up
  - `↓` / `j`: scroll down
  - `PageUp` / `u`: page up
  - `PageDown` / `d`: page down
  - `Home` / `g`: top
  - `End` / `G`: bottom
  - `q` / `esc` / `ctrl+c`: exit
- If stdout/stdin is not a TTY, print an error and exit `1`.

Because native TypeScript should be used directly, avoid JSX/TSX. Implement UI with `React.createElement(...)` or small helper functions instead of JSX.

---

### 8. Fullscreen terminal handling

Ink does not need to own all terminal behavior. Implement alternate-screen behavior explicitly:

- Before rendering fullscreen UI:
  - enter alternate screen
  - optionally hide cursor
- On exit/error:
  - show cursor
  - leave alternate screen
  - restore terminal state

Use cleanup handlers for normal exit and interruption.

---

## Tests / verification

Add Node built-in test runner tests.

### Unit tests

Test argument parsing:

- no args
- `--all`
- `--max-depth 5`
- invalid `--max-depth`
- `--fullscreen`
- `--verbose`
- `--help`
- `--version`
- invalid args

Test status parsing:

- clean branch
- modified file
- staged file
- untracked file
- deleted file
- renamed file
- conflict
- ahead
- behind
- diverged
- upstream gone

Test discovery logic:

- finds `.git` dirs
- finds `.git` files
- respects `--max-depth`
- ignores build/cache dirs
- skips cwd root `.git`
- deduplicates linked worktrees by realpath

### Smoke/integration tests

In temp dirs:

- create child git repos with `git init`
- verify `gitsy --all` shows clean repos
- verify `gitsy` hides clean repos
- modify files and verify changed repos show
- create a linked worktree if supported in the test environment and verify inclusion

### Manual verification

Run:

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm start
pnpm start -- --all
pnpm start -- --max-depth 5 --all
pnpm start -- --fullscreen
pnpm start -- --verbose
pnpm start -- --help
pnpm start -- --version
```

---

## Decisions made

- Use `pnpm`.
- Use latest local Node / Node 24 native TypeScript.
- Use Ink for all output.
- Add `--fullscreen` for optional interactive fullscreen UI.
- Use pretty parsed status instead of raw `git status` output.
- Use Unicode icons/colors by default.
- Add `--max-depth`, default `3`.
- `--max-depth` means repository directory nesting depth, not `.git` marker depth.
- Add `--all`, `--help`, `--version`, and `--verbose`.
- `--version` reads from `package.json`.
- Hardcode ignored dirs for now.
- Support worktree `.git` files.
- Also discover linked worktrees through `git worktree list --porcelain`.
- Display linked worktrees inside cwd as relative paths and outside cwd as absolute paths.
- Skip invalid/inaccessible repo candidates silently unless `--verbose` is enabled.
- In `--fullscreen` without a TTY, error and exit `1`.

---

## Tradeoffs and risks

- **Ink + native TS**: to avoid a build step, UI should avoid JSX/TSX and use `React.createElement`.
- **Linked worktrees outside cwd**: useful, but may show paths outside the user’s current scan tree. This is intentional based on the chosen worktree behavior.
- **Pretty parsing**: nicer than raw output but must be carefully tested to avoid misrepresenting git status.
- **Fullscreen cleanup**: alternate-screen handling must be robust so the terminal is restored on exit/errors.
- **Unicode icons**: prettier, but may render poorly in some terminals. No `--no-icons` is planned for v1.

---

## Remaining open questions

None.

---

## Execution guidance

If implementation deviates from this approved plan, update the saved plan to reflect the latest approved behavior and surface the deviation before continuing.
