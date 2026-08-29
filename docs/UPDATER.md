# Safe Release Updates

## Architecture And Trust Boundaries

The update system has three independent layers:

1. The application release watcher reads public GitHub release metadata and
   manifests. An upstream tag is informational, not installable.
2. The admin API and Settings UI expose status and fixed prepare, install, and
   rollback actions. Writes require an administrator JWT, enabled TOTP, a recent
   step-up grant, and exact confirmation text for install or rollback.
3. `sub2api-rework-updater` runs on the host and accepts only `status`,
   `prepare`, `install`, and `rollback` over a Unix socket. It independently
   downloads and validates the approved manifest and image digest.

The application container must never mount `/var/run/docker.sock`. The updater
is the only component permitted to control Docker. A process with Docker control
has host-equivalent privilege; the systemd sandbox limits accidental access but
does not remove that risk.

This updater supports only the repository's three-service Linux Docker Compose
topology. Policy owns the complete ordered Compose file set, including the base
file and every override. The merged model must contain the application,
PostgreSQL, and Redis services and the updater socket access. It does not support
`volumes_from` on the application or application named volumes with custom
drivers or driver options. It does not support
`docker-compose.standalone.yml` with
external PostgreSQL or Redis, and it does not support Apple container
deployments. Those topologies need separate backup, health, and runtime-control
implementations before they can use this updater.

```text
GitHub releases (read only)
        |
        v
application watcher -> admin API/UI
                            |
                            | fixed Unix-socket protocol
                            v
                    host updater -> Docker / backups
```

The updater does not accept commands, shell text, paths, images, Compose
arguments, or registry locations from a request. A compromised application can
request a fixed operation, but it cannot select an artifact outside the embedded
repository policy or bypass manifest, digest, migration, backup, and health
checks.

## Release Lifecycle

The lifecycle deliberately separates detection from installation:

```text
upstream detected
  -> compatibility review
  -> rework CI passes
  -> rework image is built
  -> release manifest is approved
  -> operator prepares
  -> operator installs
```

`.github/workflows/upstream-watch.yml` only creates or updates a tracking issue.
It never merges, builds an approved release, installs, or deploys.

A qualified rework tag must match
`backend/internal/releaseinfo/metadata.json`. The release workflow publishes
`ghcr.io/firedvl/sub2api-rework:<rework-version>`, resolves its immutable digest,
creates `release-manifest.json`, validates that file with the shared Go contract,
and uploads it to the matching GitHub release.

Signing is not required by the current updater. Adding keyless Sigstore/cosign
verification is the next integrity layer once its identity policy and incident
rotation procedure can be maintained. Digest and strict manifest verification
remain mandatory regardless of future signing.

## Host Installation

Run these steps on a staging host before the first production bootstrap.

First-time or manual maintenance that can stop the gateway must not use that
gateway as its control plane. Use an independent terminal or session, or a
reviewed detached host operation. After acceptance, normal updater operations
run host-side and do not depend on keeping the initiating HTTP stream alive.

1. Build the updater from a reviewed checkout and install it root-owned:

   ```bash
   cd backend
   CGO_ENABLED=0 go build -trimpath -o sub2api-rework-updater ./cmd/updater
   sudo install -o root -g root -m 0755 sub2api-rework-updater /usr/local/sbin/sub2api-rework-updater
   ```

2. Create the socket-access group. Add only the application container's numeric
   group ID through the provided Compose override.

   ```bash
   sudo groupadd --system sub2api-updater
   getent group sub2api-updater
   ```

3. Place the base Compose file and updater override in the managed deployment.
   Copy and edit the schema-v2 policy. Keep it root-owned and mode `0600`.
   `compose_files` must list the base file first, then every override in the exact
   order used to start the deployment. The updater rejects an empty set,
   duplicates, paths outside `deployment_directory`, missing files, symlinks,
   and unsafe ownership or permissions. It does not accept the old schema-v1
   `compose_file` field.

   ```bash
   sudo install -d -o root -g root -m 0755 /etc/sub2api-rework
   sudo install -o root -g root -m 0644 deploy/updater/docker-compose.updater.yml /opt/sub2api/docker-compose.updater.yml
   sudo install -o root -g root -m 0600 deploy/updater/updater.example.json /etc/sub2api-rework/updater.json
   ```

   Confirm every service name, path, initial version, and migration number. The
   configured database user must be able to terminate remaining connections and
   drop, create, and own the database so automatic recovery can replace it from
   the backup.

   Keep trusted updater state, audit records, and backups under the private
   `/var/lib/sub2api-rework-updater` state tree. Keep the socket and operation
   lock under `/run/sub2api-rework-updater`. The audit file is updater state and
   is not the service log; service stdout and stderr remain available through
   `journalctl -u sub2api-rework-updater.service`.

4. Set `SUB2API_IMAGE` in the deployment `.env`; use the existing image for the
   first start. The merged Compose model must keep `${SUB2API_IMAGE}` as the
   application image, mount only the updater socket directory, and pass the same
   supplemental updater GID through `group_add` and
   `SUB2API_UPDATER_GID`. Keep the deployment directory, all Compose files, and
   the environment file owned by root or the updater service UID. No managed
   directory or file may be group or world writable; state, audit, lock, backup,
   and environment files must not be accessible to group or world. The updater
   rejects symlinks at managed file paths and rejects unsafe parent directories.
   It also rejects a custom audit path when any ancestor is owned by an untrusted
   user or is group or world writable without the safe root-owned sticky-directory
   condition. Do not change the ownership or permissions of system directories to
   make a custom path pass; choose a private path whose full ancestor chain meets
   the validation rules.

5. Install the base unit. Then render its deployment-specific drop-in from the
   validated root-owned policy. The base unit keeps `ProtectSystem=strict`; the
   drop-in adds one `ReadWritePaths` entry for the configured
   `deployment_directory`. The renderer rejects control characters and broad
   paths such as `/` or `/opt`, paths containing updater installation files,
   quotes spaces and backslashes, and escapes systemd `%` specifiers.

   ```bash
   sudo install -o root -g root -m 0644 deploy/updater/sub2api-rework-updater.service /etc/systemd/system/
   sudo install -d -o root -g root -m 0755 /etc/systemd/system/sub2api-rework-updater.service.d
   sudo /usr/local/sbin/sub2api-rework-updater \
     --config /etc/sub2api-rework/updater.json \
     --print-systemd-drop-in \
     | sudo tee /etc/systemd/system/sub2api-rework-updater.service.d/10-deployment.conf >/dev/null
   sudo chmod 0644 /etc/systemd/system/sub2api-rework-updater.service.d/10-deployment.conf
   systemd-analyze verify /etc/systemd/system/sub2api-rework-updater.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now sub2api-rework-updater.service
   sudo systemctl status sub2api-rework-updater.service
   ```

   Regenerate the drop-in before restarting the service whenever
   `deployment_directory` changes.

6. Set `SUB2API_UPDATER_GID` to the host group ID and start or recreate the
   application with the same ordered file set stored in policy:

   ```bash
   docker compose --project-directory /opt/sub2api \
     -f /opt/sub2api/docker-compose.yml \
     -f /opt/sub2api/docker-compose.updater.yml \
     --env-file /opt/sub2api/.env \
     up -d --no-deps sub2api
   ```

   Verify that the application reaches updater status through the Unix socket,
   has the supplemental socket GID, and has no Docker socket mount. Install the
   corrected updater `1.1.3` and its schema-v2 policy before installing a release
   whose manifest requires `minimum_updater_version: 1.1.3`. Version `1.1.3`
   scopes Compose decoding to fields the updater validates, so valid fields on
   unrelated services cannot fail application preflight. It retains the `1.1.2`
   Redis reply validation instead of trusting `redis-cli`'s zero exit status.
   It accepts an exact `PONG` without incidental client auth, then retries with
   the managed service credential only when Redis requires authentication.

### Replace An Active Updater

When the application already bind-mounts `/run/sub2api-rework-updater`, replace
the updater without stopping it first. The base unit uses
`RuntimeDirectoryPreserve=restart`, which keeps the same runtime directory for
manual and automatic restarts but removes it on an actual stop. Using
`RuntimeDirectoryPreserve=yes` would also keep it after a stop and is not
appropriate here.

On restart, systemd leaves the managed directory in place instead of removing
and recreating it. `ServeUnix` removes the old socket and creates the new socket
inside that preserved directory, so the application's existing bind mount keeps
the same directory identity and sees the new socket.

1. Build the reviewed updater and verify its version before installation:

   ```bash
   cd backend
   CGO_ENABLED=0 go build -trimpath -o sub2api-rework-updater ./cmd/updater
   ./sub2api-rework-updater --version
   ```

2. Confirm the running updater is healthy and idle. Back up the current binary,
   unit, drop-in, private state, and policy to a root-only directory.
3. Install the updated base unit, verify the combined unit, and reload systemd
   while the old updater keeps running:

   ```bash
   sudo install -o root -g root -m 0644 \
     ../deploy/updater/sub2api-rework-updater.service \
     /etc/systemd/system/sub2api-rework-updater.service
   sudo systemd-analyze verify /etc/systemd/system/sub2api-rework-updater.service
   sudo systemctl daemon-reload
   ```

4. Record the runtime directory and container identities before replacement:

   ```bash
   runtime_before=$(stat -Lc '%d:%i' /run/sub2api-rework-updater)
   app_before=$(docker compose --project-directory /opt/sub2api \
     -f /opt/sub2api/docker-compose.yml \
     -f /opt/sub2api/docker-compose.updater.yml \
     --env-file /opt/sub2api/.env ps -q sub2api)
   postgres_before=$(docker compose --project-directory /opt/sub2api \
     -f /opt/sub2api/docker-compose.yml \
     -f /opt/sub2api/docker-compose.updater.yml \
     --env-file /opt/sub2api/.env ps -q postgres)
   redis_before=$(docker compose --project-directory /opt/sub2api \
     -f /opt/sub2api/docker-compose.yml \
     -f /opt/sub2api/docker-compose.updater.yml \
     --env-file /opt/sub2api/.env ps -q redis)
   ```

5. Install the new executable beside the active path, rename it over the old
   executable on the same filesystem, and restart the service:

   ```bash
   sudo install -o root -g root -m 0755 sub2api-rework-updater \
     /usr/local/sbin/sub2api-rework-updater.next
   sudo mv -fT /usr/local/sbin/sub2api-rework-updater.next \
     /usr/local/sbin/sub2api-rework-updater
   sudo systemctl restart sub2api-rework-updater.service
   ```

6. Verify the runtime directory identity is unchanged, updater status is
   reachable from the application, migration remains `232`, the application has
   no Docker socket, and the application, PostgreSQL, and Redis container IDs
   match the recorded values. Do not prepare or install a release until every
   check passes.

Do not use a separate `systemctl stop` and `systemctl start` for this attached
socket-directory case. A stop intentionally removes the runtime directory and
can leave the running application attached to the removed directory.

### Recover A Partial 1.1.1 Bootstrap

For a healthy `0.1.183-rework.3` deployment at migration `232` where updater
`1.1.1` was installed but remains inactive and the application has not yet been
recreated with socket access:

1. Replace the inactive updater binary with reviewed updater `1.1.3`.
2. Change `audit_path` in the root-owned schema-v2 policy to
   `/var/lib/sub2api-rework-updater/audit.jsonl`. Do not rewrite other custom
   administrator paths. Preserve any audit file at the old configured path as
   incident evidence before changing it.
3. Install the updated base unit, regenerate the deployment-specific drop-in,
   and run `systemd-analyze verify` before enabling and starting the updater.
4. Verify updater status through its Unix socket.
5. Recreate only the existing `0.1.183-rework.3` application with the complete
   ordered Compose file set so it receives socket access. Verify health and
   migration `232` before preparing or installing `0.1.183-rework.7`.

This recovery does not repeat the completed `231` to `232` migration.

## Prepare And Install Flow

`prepare` acquires the exclusive file lock, validates the approved manifest,
runs preflight against Docker's merged non-interpolated ownership model and its
interpolated runtime model, pulls the
digest-addressed image, verifies its repository digest, and persists the prepared
identity. It does not change the active deployment. Every Compose command uses
the policy's full ordered file set.

Deployment-structure failures expose only a fixed safe reason after
`deployment structure check failed:`: `compose-file`, `environment-file`,
`raw-compose-command`, `raw-compose-json`, `managed-image`,
`rendered-compose-command`, `rendered-compose-json`, `application-service`,
`database-service`, `redis-service`, `volumes-from`, `docker-socket`,
`named-volume`, `updater-socket-access`, or `internal`. The updater does not
include Compose output, environment values, Docker stderr, or underlying command
errors in status or audit records.

`install` reacquires the lock and refetches the manifest. Before mutation it
requires a matching prepared identity, healthy application, reachable PostgreSQL
and Redis, valid Compose structure, sufficient disk, writable backup storage,
compatible migration state, and the approved image digest. It then:

1. stores every Compose file under a deterministic name, its original absolute
   path, order, and SHA-256 checksum, plus the environment file, source
   image/digest, migration state, and a PostgreSQL custom-format dump under a
   timestamped update ID;
2. stops the application service;
3. pins `SUB2API_IMAGE` to the approved digest;
4. runs `/app/sub2api --migrate` in a one-shot application container;
5. starts the application service;
6. checks `/health`, `/`, public frontend settings, PostgreSQL, Redis, and the
   exact migration state;
7. records the bounded audit result.

The checks send no provider credentials and no paid model traffic.

## Rollback Behavior And Limits

If migration or health validation fails after deployment mutation, the updater
uses a fresh bounded recovery timeout and stops the application. Whenever
migration execution was attempted, it forcibly disconnects remaining clients,
recreates the target PostgreSQL database, and restores the pre-update dump. It
then restores the previous image and environment, restarts the service with the
complete ordered Compose set, and reruns all health checks. The updater does not
modify Compose files, so their private backup copies remain recovery and audit
evidence. Backup validation rejects missing, reordered, renamed, or
checksum-mismatched Compose copies. A stop or rollback failure leaves the updater in `critical` state.
Prepare and install remain blocked in that state. A matching recorded rollback
may be retried, but a failed or interrupted recovery attempt remains `critical`
and preserves the recorded backup.

Manual rollback is allowed only to the updater's recorded previous version and
only while the database migration number still equals the backup's source
migration. It is blocked after schema advancement because substituting an older
application without restoring the database is not generally safe.

Database recreation is destructive and discards writes made after the backup,
including objects created by the failed migration. Use automatic restore only
during the bounded failed-install window. Outside that window, stop traffic and
choose recovery based on the incident timeline; do not claim application
rollback alone reverses database changes.

## Staging Qualification

Before any production installation:

1. Clone the production Compose topology with synthetic credentials and data in
   a deployment directory other than `/opt/sub2api-rework/deploy`.
2. Start the application with a base file plus the updater socket override.
3. Model a root-owned, non-root-group `/var/log` ancestor with mode `0775`.
   Confirm a custom audit path below it fails, then start updater `1.1.3` with
   state, audit, and backups under its private state tree, the same ordered set,
   staging-only paths, and a loopback health URL. Verify its systemd drop-in
   permits the configured deployment directory and no broader tree.
4. Before prepare, record the runtime directory identity and all three container
   IDs. Restart the updater while preserving the runtime directory. Confirm the
   directory and container identities are unchanged, migration remains `232`,
   the application reaches the new socket, and the Docker socket is absent.
5. With no Redis server password, set stale `REDISCLI_AUTH`. Confirm ordinary
   `redis-cli` prints an auth failure but exits zero with `PONG`, while updater
   preflight passes. Then require a password and confirm a zero-exit `NOAUTH`
   reply does not fool the updater: correct credentials must pass; missing or
   wrong credentials and a stopped or unhealthy Redis must fail without exposing
   the password in status or audit.
6. Verify `prepare` leaves the active `.env`, containers, and database unchanged.
7. Install an approved test release. Inspect the recreated application container
   and verify the updater socket bind mount, supplemental socket GID, image
   digest, migration state, frontend assets, PostgreSQL, Redis, and health
   endpoints. Verify the Docker socket is absent.
8. Query updater status from the recreated application, then run another
   prepare/status operation.
9. Force application health failure and confirm automatic image/database
   recovery, removal of schema objects created only by the failed migration, and
   a healthy prior version. Recheck the updater socket mount, supplemental GID,
   Docker socket absence, and updater status after rollback.
10. Force restore failure and confirm the visible `critical` state and audit.
11. Send concurrent operations and confirm only one acquires the lock.

## Emergency Recovery

If updater state is `critical`, stop automated attempts. Preserve
`/var/lib/sub2api-rework-updater`, any configured audit path outside that tree,
and the deployment directory before changing anything. Inspect the bounded
updater audit and Docker service state locally; service logs remain in the
systemd journal and are not exposed in the UI.

Do not prepare or install another release while state is `critical`. Resolve the
recorded rollback or complete a reviewed manual recovery first; a new install
would replace the recovery metadata needed for the current incident.

For application-only recovery with an unchanged schema, set `SUB2API_IMAGE` to
the recorded prior digest and recreate only the application service. If the
schema advanced, stop application traffic and restore the recorded PostgreSQL
dump before starting the prior image. Validate health and migration state before
reopening traffic.

## Disable The Updater

Disable the host service and remove the Compose override/socket mount:

```bash
sudo systemctl disable --now sub2api-rework-updater.service
```

The watcher remains read-only and the Settings page reports the updater as
unavailable. Do not delete backups or state until the last update is verified.
