#!/bin/sh
set -eu

repo_root=$(git rev-parse --show-toplevel)
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/gitsy-pre-push.XXXXXX")
trap 'rm -rf "$temp_dir"' 0
trap 'exit 1' HUP INT TERM

while read -r local_ref local_oid remote_ref remote_oid; do
  case "$local_oid" in
    *[!0]*) ;;
    *) continue ;;
  esac
  if ! commit=$(git rev-parse --verify "$local_oid^{commit}" 2>/dev/null); then
    continue
  fi

  checkout="$temp_dir/$commit"
  if [ -d "$checkout" ]; then
    continue
  fi
  mkdir "$checkout"
  git archive --format=tar "$commit" > "$temp_dir/source.tar"
  tar -xf "$temp_dir/source.tar" -C "$checkout"
  printf 'Checking %s (%s)\n' "$local_ref" "$commit"
  (
    for name in $(git rev-parse --local-env-vars); do
      unset "$name"
    done
    GITSY_CHECK_ROOT="$checkout" mise -C "$repo_root" run check </dev/null
  )
done
