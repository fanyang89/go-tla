#!/usr/bin/env bash
# Explicit provisioning only; analyze and tests never download executables.
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/tlc.env"
if [[ $# -ne 1 ]]; then
  echo "usage: bash scripts/download-tlc.sh /path/to/tla2tools.jar" >&2
  exit 2
fi
mkdir -p -- "$(dirname -- "$1")"
dest_dir=$(cd -- "$(dirname -- "$1")" && pwd)
dest="$dest_dir/$(basename -- "$1")"
if [[ -e "$dest" ]]; then
  if ! printf '%s  %s\n' "$TLC_SHA256" "$dest" | sha256sum --check --status; then
    echo "Existing TLC checksum mismatch; refusing to overwrite: $dest" >&2
    exit 1
  fi
  printf 'Verified existing TLC %s: %s\n' "$TLC_VERSION" "$dest"
  exit 0
fi
# Stage beside the destination; never leave a partial/unverified JAR at its path.
tmp=$(mktemp "${dest}.download.XXXXXX")
trap 'rm -f -- "$tmp"' EXIT
curl --fail --location --retry 3 --connect-timeout 20 --max-time 180 \
  --proto '=https' --proto-redir '=https' \
  "https://github.com/tlaplus/tlaplus/releases/download/v${TLC_VERSION}/tla2tools.jar" \
  --output "$tmp"
if ! printf '%s  %s\n' "$TLC_SHA256" "$tmp" | sha256sum --check --status; then
  echo "Downloaded TLC checksum mismatch; expected $TLC_SHA256" >&2
  exit 1
fi
mv -- "$tmp" "$dest"
printf 'Verified TLC %s: %s\n' "$TLC_VERSION" "$dest"
