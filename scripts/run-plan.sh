#!/usr/bin/env bash
# Run the numbered prompts in prompt/ one at a time, each in a fresh session,
# stopping at the first failure.
#
#   ./scripts/run-plan.sh
#   FROM=1 TO=14 ./scripts/run-plan.sh
#   ONLY=07-existing-member-gets-labels ./scripts/run-plan.sh
#   COMMIT=0 ./scripts/run-plan.sh

set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
LOGS="$ROOT/.scratch/split"
MODEL=${MODEL:-ollama/deepseek-v4.1-flash:1017-cloud}
FROM=${FROM:-1}
TO=${TO:-99}
COMMIT=${COMMIT:-1}
ONLY=${ONLY:-}

mkdir -p "$LOGS"
cd "$ROOT" || exit 1

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "== tree is dirty; commit it before starting" >&2
  exit 1
fi

if [ -n "$ONLY" ]; then
  IFS=',' read -r -a files <<<"$ONLY"
  for i in "${!files[@]}"; do files[$i]="$ROOT/prompt/${files[$i]}.md"; done
else
  files=("$ROOT"/prompt/*.md)
fi

for path in "${files[@]}"; do
  name=$(basename "$path" .md)
  step=$((10#${name%%-*}))

  if [ -z "$ONLY" ]; then
    [ "$step" -lt "$FROM" ] && continue
    [ "$step" -gt "$TO" ] && continue
  fi

  [ -f "$path" ] || { echo "== $name: no such prompt" >&2; exit 1; }

  echo "== $name"
  opencode run -m "$MODEL" --auto --title "plan $name" "$(cat "$path")" 2>&1 | tee "$LOGS/$name.log"
  status=${PIPESTATUS[0]}
  [ "$status" -ne 0 ] && { echo "== $name: opencode exited $status" >&2; exit 1; }

  if ! go vet ./... >"$LOGS/$name.vet" 2>&1 || ! make test >"$LOGS/$name.test" 2>&1; then
    echo "== $name: vet or test failed, see $LOGS" >&2
    exit 1
  fi

  git diff --stat >"$LOGS/$name.diff"

  if [ "$COMMIT" = "1" ]; then
    git add -A
    if git diff --cached --quiet; then
      echo "== $name: no changes to commit" >&2
    else
      git commit -q -m "split: $name"
    fi
  fi
done

echo "== finished"
