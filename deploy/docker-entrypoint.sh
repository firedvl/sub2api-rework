#!/bin/sh
set -e

# Fix data directory permissions when running as root.
# Docker named volumes / host bind-mounts may be owned by root,
# preventing the non-root sub2api user from writing files.
if [ "$(id -u)" = "0" ]; then
    mkdir -p /app/data
    # Use || true to avoid failure on read-only mounted files (e.g. config.yaml:ro)
    chown -R sub2api:sub2api /app/data 2>/dev/null || true
    if [ -n "${SUB2API_UPDATER_GID:-}" ]; then
        case "$SUB2API_UPDATER_GID" in
            *[!0-9]*) echo "SUB2API_UPDATER_GID must be a positive numeric GID" >&2; exit 1 ;;
            *[1-9]*) ;;
            *) echo "SUB2API_UPDATER_GID must be a positive numeric GID" >&2; exit 1 ;;
        esac
        updater_group=$(awk -F: -v gid="$SUB2API_UPDATER_GID" '$3 == gid { print $1; exit }' /etc/group)
        if [ -z "$updater_group" ]; then
            updater_group=sub2api-updater
            addgroup -g "$SUB2API_UPDATER_GID" "$updater_group"
        fi
        if ! id -nG sub2api | tr ' ' '\n' | grep -Fxq "$updater_group"; then
            addgroup sub2api "$updater_group"
        fi
    fi
    # Re-invoke this script as sub2api so the flag-detection below
    # also runs under the correct user.
    exec su-exec sub2api "$0" "$@"
fi

# Compatibility: if the first arg looks like a flag (e.g. --help),
# prepend the default binary so it behaves the same as the old
# ENTRYPOINT ["/app/sub2api"] style.
if [ "${1#-}" != "$1" ]; then
    set -- /app/sub2api "$@"
fi

exec "$@"
