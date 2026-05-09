# gitsy

A small CLI that shows Git status across child repositories and linked worktrees.

It scans the current directory for repos, hides clean repos by default, and renders a pretty Ink terminal table.

## How to install

### 1. Checkout repo

```bash
git clone git@github.com:flexdinesh/gitsy.git
cd gitsy
pnpm install
```

### 2. Setup pnpm global bin

If pnpm has not configured a global bin directory yet, run:

```bash
pnpm setup
```

Then restart your shell, or source your shell config:

```bash
source ~/.zshrc
```

This fixes errors like:

```text
ERR_PNPM_NO_GLOBAL_BIN_DIR Unable to find the global bin directory
```

### 3. Link gitsy globally

From the `gitsy` repo:

```bash
pnpm link --global
```

### 4. CLI will be available

Run from any directory you want to scan:

```bash
gitsy
gitsy --all
gitsy --max-depth 5
gitsy --fullscreen
```

## Usage

```bash
gitsy [options]
```

Options:

```text
--all              Show clean repos too
--max-depth <n>    Scan repo dirs up to n levels deep; default 3
--fullscreen       Open a scrollable fullscreen UI
--verbose          Show warnings for skipped repos/git errors
--help             Show help
--version          Show version
```

## Run with Node

Without linking, run from any directory you want to scan:

```bash
node ~/workspace/gitsy
```

Or run the entry file directly:

```bash
node  ~/workspace/gitsy/src/index.ts --all
```
