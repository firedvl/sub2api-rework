#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BACKEND_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(CDPATH= cd -- "$BACKEND_DIR/.." && pwd)"
METADATA_FILE="$BACKEND_DIR/internal/releaseinfo/metadata.json"

# Prefer the exact release tag when building from a tagged checkout so
# source builds from a release tag use that exact identity.
if command -v git >/dev/null 2>&1; then
  TAG="$(
    git -C "$REPO_DIR" describe --tags --exact-match --match 'v[0-9]*' 2>/dev/null || \
    git -C "$REPO_DIR" describe --tags --exact-match --match '[0-9]*' 2>/dev/null || \
    true
  )"
  if [ -n "$TAG" ]; then
    printf '%s\n' "${TAG#v}"
    exit 0
  fi
fi

VERSION="$(sed -n 's/^[[:space:]]*"rework_version":[[:space:]]*"\([^"]*\)",[[:space:]]*$/\1/p' "$METADATA_FILE")"
if [ -z "$VERSION" ]; then
  printf '%s\n' "unable to read rework_version from release metadata" >&2
  exit 1
fi
printf '%s\n' "$VERSION"
