#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

check_application_security_opt() {
  file=$1
  count=$(
    awk '
      $0 == "  sub2api:" {
        in_application = 1
        next
      }
      in_application && $0 ~ /^  [A-Za-z0-9_-]+:$/ {
        in_application = 0
      }
      in_application && $0 == "    security_opt:" {
        in_security_opt = 1
        next
      }
      in_application && in_security_opt && $0 == "      - no-new-privileges:true" {
        count++
      }
      END { print count + 0 }
    ' "$file"
  )

  if [ "$count" -ne 1 ]; then
    printf '%s must enable no-new-privileges exactly once for the sub2api service\n' "$file" >&2
    exit 1
  fi
}

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  check_application_security_opt "$compose_file"
done

grep -Fqx '      - SUB2API_UPDATER_GID=${SUB2API_UPDATER_GID:?SUB2API_UPDATER_GID is required}' \
  deploy/updater/docker-compose.updater.yml || {
  printf 'updater compose override must pass its supplemental GID to the entrypoint\n' >&2
  exit 1
}
sh -n deploy/docker-entrypoint.sh

entrypoint_test_root=$(mktemp -d "${TMPDIR:-/tmp}/sub2api-entrypoint-test.XXXXXX")
trap 'rm -rf "$entrypoint_test_root"' EXIT HUP INT TERM
mkdir "$entrypoint_test_root/bin"
cat >"$entrypoint_test_root/bin/mock-command" <<'EOF'
#!/bin/sh
if [ "${0##*/}" = id ]; then
  case "${1:-}" in
    -u) printf '0\n' ;;
    -nG) printf 'sub2api\n' ;;
  esac
fi
EOF
chmod +x "$entrypoint_test_root/bin/mock-command"
for command in id mkdir chown awk addgroup su-exec; do
  ln -s mock-command "$entrypoint_test_root/bin/$command"
done

for gid in 0 00 000000 invalid; do
  if PATH="$entrypoint_test_root/bin:$PATH" SUB2API_UPDATER_GID=$gid \
    sh deploy/docker-entrypoint.sh true >/dev/null 2>&1; then
    printf 'entrypoint must reject updater GID %s\n' "$gid" >&2
    exit 1
  fi
done

for gid in 2000 0002; do
  if ! PATH="$entrypoint_test_root/bin:$PATH" SUB2API_UPDATER_GID=$gid \
    sh deploy/docker-entrypoint.sh true >/dev/null 2>&1; then
    printf 'entrypoint must accept positive updater GID %s\n' "$gid" >&2
    exit 1
  fi
done

printf 'docker compose security test passed\n'
