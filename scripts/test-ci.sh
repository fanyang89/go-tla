#!/usr/bin/env bash
# Local and CI release gate. Provision TLC explicitly before invoking this script.
set -euo pipefail
: "${TLC_JAR:?Set TLC_JAR to the pinned tla2tools.jar}"
if [[ ${UPDATE_SNAPSHOTS:-} == 1 ]]; then
  echo 'Snapshot regeneration is forbidden in the verification gate.' >&2
  exit 1
fi
# Resolve relative JAR paths before changing to the repository root.
jar_dir=$(cd -- "$(dirname -- "$TLC_JAR")" && pwd)
TLC_JAR="$jar_dir/$(basename -- "$TLC_JAR")"
export TLC_JAR
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd -- "$root"
source scripts/tlc.env
if ! printf '%s  %s\n' "$TLC_SHA256" "$TLC_JAR" | sha256sum --check --status; then
  echo "TLC checksum mismatch: expected TLC $TLC_VERSION ($TLC_SHA256) at $TLC_JAR" >&2
  exit 1
fi
command -v java >/dev/null
printf 'TLC %s sha256=%s\n' "$TLC_VERSION" "$TLC_SHA256"
go version
java -version
# Do not permit ambient GOFLAGS (e.g. -run) to silently reduce the test suite.
export GOFLAGS=
export GOTLA_REQUIRE_TLC=1
python3 -B scripts/capture_self_check_test.py
go vet ./...
go test ./... -count=1 -v
git diff --exit-code -- tests/testdata
