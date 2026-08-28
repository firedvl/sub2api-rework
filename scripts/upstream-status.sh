#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
metadata_file="$repo_dir/backend/internal/releaseinfo/metadata.json"
upstream_ref=refs/remotes/upstream/main

baseline_tag=$(sed -n 's/^[[:space:]]*"upstream_baseline":[[:space:]]*"\([^"]*\)",[[:space:]]*$/\1/p' "$metadata_file")
baseline_sha=$(sed -n 's/^[[:space:]]*"upstream_baseline_sha":[[:space:]]*"\([^"]*\)",[[:space:]]*$/\1/p' "$metadata_file")

if [ -z "$baseline_tag" ] || [ -z "$baseline_sha" ]; then
	printf '%s\n' "unable to read the canonical upstream baseline" >&2
	exit 1
fi
if ! git -C "$repo_dir" show-ref --verify --quiet "$upstream_ref"; then
	printf '%s\n' "missing upstream/main; run: git fetch upstream --tags" >&2
	exit 1
fi

resolved_sha=$(git -C "$repo_dir" rev-parse "refs/tags/$baseline_tag^{commit}")
latest_tag=$(git -C "$repo_dir" tag --merged "$upstream_ref" --list 'v[0-9]*' --sort=-v:refname | sed -n '1p')
commits_ahead=$(git -C "$repo_dir" rev-list --count "refs/tags/$baseline_tag..$upstream_ref")

baseline_matches=false
[ "$resolved_sha" = "$baseline_sha" ] && baseline_matches=true
newer_release=false
if [ -n "$latest_tag" ] && [ "$latest_tag" != "$baseline_tag" ] &&
	git -C "$repo_dir" merge-base --is-ancestor "refs/tags/$baseline_tag" "refs/tags/$latest_tag"; then
	newer_release=true
fi

printf '%s\n' \
	"documented_baseline=$baseline_tag" \
	"canonical_baseline_sha=$baseline_sha" \
	"resolved_baseline_sha=$resolved_sha" \
	"baseline_matches=$baseline_matches" \
	"latest_fetched_upstream_tag=$latest_tag" \
	"commits_ahead=$commits_ahead" \
	"newer_upstream_release=$newer_release"
