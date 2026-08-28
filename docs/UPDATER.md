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
topology, where the selected Compose file contains the application, PostgreSQL,
and Redis services. It does not support `docker-compose.standalone.yml` with
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

These steps are examples for a staging host. This pull request does not run them.

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

3. Copy and edit the policy. Keep it root-owned and mode `0600`. Confirm every
   service name, path, initial version, and migration number against the staging
   deployment.

   ```bash
   sudo install -d -o root -g root -m 0755 /etc/sub2api-rework
   sudo install -o root -g root -m 0600 deploy/updater/updater.example.json /etc/sub2api-rework/updater.json
   ```

4. Set `SUB2API_IMAGE` in the deployment `.env`; use the existing image for the
   first start. Compose must reference `${SUB2API_IMAGE}` for the application.
   Keep the deployment directory, Compose file, and environment file owned by
   root or the updater service UID. No managed directory or file may be group or
   world writable; state, audit, lock, backup, and environment files must not be
   accessible to group or world. The updater rejects symlinks at managed file
   paths and rejects unsafe parent directories.

5. Install and inspect the unit before enabling it:

   ```bash
   sudo install -o root -g root -m 0644 deploy/updater/sub2api-rework-updater.service /etc/systemd/system/
   systemd-analyze verify /etc/systemd/system/sub2api-rework-updater.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now sub2api-rework-updater.service
   sudo systemctl status sub2api-rework-updater.service
   ```

6. Apply `deploy/updater/docker-compose.updater.yml` as a second Compose file,
   set `SUB2API_UPDATER_GID` to the host group ID, and recreate only the
   application container. Verify that the application can reach the updater
   socket and still has no Docker socket mount.

## Prepare And Install Flow

`prepare` acquires the exclusive file lock, validates the approved manifest,
runs preflight, pulls the digest-addressed image, verifies its repository digest,
and persists the prepared identity. It does not change the active deployment.

`install` reacquires the lock and refetches the manifest. Before mutation it
requires a matching prepared identity, healthy application, reachable PostgreSQL
and Redis, valid Compose structure, sufficient disk, writable backup storage,
compatible migration state, and the approved image digest. It then:

1. stores the Compose file, environment file, source image/digest, migration
   state, and a PostgreSQL custom-format dump under a timestamped update ID;
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
uses a fresh bounded recovery timeout, stops the application, restores the
PostgreSQL dump whenever migration execution was attempted, restores the
previous image/configuration, restarts the service, and reruns all health
checks. A stop or rollback failure leaves the updater in `critical` state.
Prepare and install remain blocked in that state. A matching recorded rollback
may be retried, but a failed or interrupted recovery attempt remains `critical`
and preserves the recorded backup.

Manual rollback is allowed only to the updater's recorded previous version and
only while the database migration number still equals the backup's source
migration. It is blocked after schema advancement because substituting an older
application without restoring the database is not generally safe.

Database restore is destructive and can discard writes made after the backup.
Use automatic restore only during the bounded failed-install window. Outside
that window, stop traffic and choose recovery based on the incident timeline;
do not claim application rollback alone reverses database changes.

## Staging Qualification

Before any production installation:

1. Clone the production Compose topology with synthetic credentials and data.
2. Start the updater with staging-only paths and a loopback health URL.
3. Verify `prepare` leaves the active `.env`, containers, and database unchanged.
4. Install an approved test release and verify the digest, migration state,
   frontend assets, PostgreSQL, Redis, and gateway health endpoints.
5. Force application health failure and confirm automatic image/database
   recovery and a healthy prior version.
6. Force restore failure and confirm the visible `critical` state and audit.
7. Send concurrent operations and confirm only one acquires the lock.
8. Confirm the application container has the updater socket only, never the
   Docker socket.

## Emergency Recovery

If updater state is `critical`, stop automated attempts. Preserve
`/var/lib/sub2api-rework-updater`, `/var/log/sub2api-rework-updater`, and the
deployment directory before changing anything. Inspect the bounded updater
audit and Docker service state locally; raw host logs are not exposed in the UI.

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
