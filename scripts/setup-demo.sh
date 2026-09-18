#!/usr/bin/env bash
# Regenerates a demo workspace of child git repos covering gitsy's UI states:
# clean, dirty, staged, ahead, behind, plus one linked worktree.
# The output dir is git-ignored on purpose: nested .git dirs are neither
# committable (git records them as gitlinks) nor portable (absolute paths).
# Commit this script, not its output.
#
# Usage: scripts/setup-demo.sh [target-dir]
#   default target: /tmp/gitsy-demo
set -euo pipefail

TARGET="${1:-/tmp/gitsy-demo}"

export GIT_AUTHOR_NAME="Gitsy Demo" GIT_AUTHOR_EMAIL="demo@example.com"
export GIT_COMMITTER_NAME="Gitsy Demo" GIT_COMMITTER_EMAIL="demo@example.com"

rm -rf "$TARGET"
mkdir -p "$TARGET/remotes"

mk() { # $1 = name
  git init -q -b main "$TARGET/$1"
  echo "# $1" > "$TARGET/$1/README.md"
  git -C "$TARGET/$1" add README.md
  git -C "$TARGET/$1" commit -qm "init $1"
}

mk repo-clean
mk repo-dirty
mk repo-staged

# Linked worktree off repo-clean, on its own branch.
git -C "$TARGET/repo-clean" worktree add -q -b feature ../repo-clean-wt

# Dirty: modified tracked file + untracked file.
echo "wip" >> "$TARGET/repo-dirty/README.md"
echo "scratch" > "$TARGET/repo-dirty/notes.txt"

# Staged: new file in the index.
echo "feature" > "$TARGET/repo-staged/feature.go"
git -C "$TARGET/repo-staged" add feature.go

# Ahead: local commit past origin (file:// bare remote, works offline).
git init -q --bare "$TARGET/remotes/repo-ahead.git"
mk repo-ahead
git -C "$TARGET/repo-ahead" remote add origin "$TARGET/remotes/repo-ahead.git"
git -C "$TARGET/repo-ahead" push -q -u origin main
echo "local work" >> "$TARGET/repo-ahead/README.md"
git -C "$TARGET/repo-ahead" commit -qam "local commit ahead"

# Behind: origin moved on; gitsy's default fetch reveals it.
git init -q --bare "$TARGET/remotes/repo-behind.git"
mk repo-behind
git -C "$TARGET/repo-behind" remote add origin "$TARGET/remotes/repo-behind.git"
git -C "$TARGET/repo-behind" push -q -u origin main
git clone -q "$TARGET/remotes/repo-behind.git" "$TARGET/.tmp-push"
echo "teammate work" >> "$TARGET/.tmp-push/README.md"
git -C "$TARGET/.tmp-push" commit -qam "teammate commit"
git -C "$TARGET/.tmp-push" push -q origin main
rm -rf "$TARGET/.tmp-push"

# Bulk: 20 more repos with rotating states so the list overflows a
# standard terminal and exercises scrolling. Dirty ones carry several
# files to add multi-row detail blocks.
for i in $(seq -w 1 20); do
  name="demo-$i"
  if [ "$i" = "07" ]; then
    name="demo-07-a-very-long-service-name-that-truncates"
  fi
  mk "$name"
  case $((10#$i % 4)) in
    1) # dirty: modified files + untracked
      for f in $(seq 1 $((10#$i % 3 + 1))); do
        echo "change $f" >> "$TARGET/$name/file-$f.go"
        git -C "$TARGET/$name" add "file-$f.go" 2>/dev/null || true
        git -C "$TARGET/$name" commit -qm "add file-$f.go" 2>/dev/null || true
        echo "wip $f" >> "$TARGET/$name/file-$f.go"
      done
      echo "scratch" > "$TARGET/$name/notes.txt"
      ;;
    2) # staged
      echo "feature" > "$TARGET/$name/feature.go"
      git -C "$TARGET/$name" add feature.go
      ;;
    3) # untracked only
      echo "scratch" > "$TARGET/$name/notes.txt"
      ;;
  esac
done

echo "demo ready at $TARGET"
echo "run: go run ./cmd/gitsy --dir $TARGET"
