#!/usr/bin/env bash
set -euo pipefail

expected_sha=${1:-$(git rev-parse HEAD)}
expected_version=${2:-$(jq -er .rework_version backend/internal/releaseinfo/metadata.json)}

[[ "$expected_version" == "$(jq -er .rework_version backend/internal/releaseinfo/metadata.json)" ]]
[[ -s backend/internal/web/dist/index.html ]]

[[ "$(git rev-parse HEAD)" == "$expected_sha" ]]
[[ -z "$(git status --porcelain=v1)" ]]
git diff --exit-code >/dev/null
git diff --cached --exit-code >/dev/null
[[ "$(git rev-parse --verify HEAD^{commit})" == "$expected_sha" ]]

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  goos=${target%/*}; goarch=${target#*/}
  out="$tmp/sub2api-${goos}-${goarch}"
  [[ "$goos" == windows ]] && out+='.exe'
  (cd backend && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -tags=embed -ldflags="-s -w -X main.Commit=$expected_sha -X main.BuildType=release" -o "$out" ./cmd/server)
  info=$(go version -m "$out")
  grep -Fq "vcs.revision=$expected_sha" <<<"$info"
  grep -Fq 'vcs.modified=false' <<<"$info"
  # Custom identity is embedded JSON, not a go version -m build setting.
  if [[ "$target" == "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" ]]; then
    "$out" --version 2>&1 | grep -F "Sub2API $expected_version (commit: $expected_sha,"
  fi
  printf '%s vcs.revision=%s vcs.modified=false\n' "$target" "$expected_sha"
done

[[ -z "$(git status --porcelain=v1)" ]]
git diff --exit-code >/dev/null
git diff --cached --exit-code >/dev/null
