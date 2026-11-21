#!/usr/bin/env bash
# Run TLC on the SaveFile model. On the TLC path, exit with TLC's status:
#   0              every invariant holds
#   12             an invariant failed. The output names it
#   11             deadlock
#   150, 151, 255  TLC could not check the spec
# Exit 2 means mise is not on PATH. That status is from this script, not TLC.
set -euo pipefail

skill_dir="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

java_tool="java@temurin-25"
tlc_tool="http:tla2tools[url=https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar,checksum=sha256:936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88]@1.7.4"

if ! command -v mise >/dev/null 2>&1; then
  echo "mise is not on PATH. Install it, then run this script again." >&2
  exit 2
fi

mise install "$java_tool" "$tlc_tool" >/dev/null
jar_dir="$(mise where "$tlc_tool")"
java="$(mise which java --tool "$java_tool")"

cd "$skill_dir"
set +e
"$java" -XX:+UseParallelGC -jar "$jar_dir/tla2tools.jar" \
  -workers 1 \
  -metadir "$work" \
  -config "$skill_dir/SaveFile.cfg" \
  "$skill_dir/SaveFile.tla"
status=$?
set -e
exit "$status"
