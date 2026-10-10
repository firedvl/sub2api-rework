# Safe Release Updates

## Architecture And Trust Boundaries

The update system has three independent layers:

1. The application release watcher reads public GitHub release metadata and
   manifests. An upstream tag is informational, not installable.
2. The admin API and Settings UI expose status and fixed prepare, install, and
   rollback and recovery actions. Writes require an administrator JWT, enabled
   TOTP, a recent step-up grant, and exact confirmation text for mutations.
3. `sub2api-rework-updater` runs on the host and accepts only `status`,
   `prepare`, `install`, `rollback`, `prepare_recovery`, and `recover` over a Unix socket. It independently
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

   The base unit sets `HOME=/var/lib/sub2api-rework-updater`, inside the private
   state directory that systemd creates with mode `0700`. Keep `ProtectHome=yes`.
   `HOME=/root` is inaccessible under that setting and can prevent the Docker CLI
   from discovering Compose. Install the Compose plugin system-wide; do not depend
   on root's Docker CLI configuration or plugins, expose `/root`, or disable
   `ProtectHome`.

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

### Roll Out 0.1.183-rework.8 From Current Production

Use this flow only after `0.1.183-rework.8` is published. Current production
already runs application `0.1.183-rework.3`, migration `232`, and updater `1.1.3`,
and already has `RuntimeDirectoryPreserve=restart`. This rollout replaces the
base unit to add the private updater home. It does not replace the updater binary
or recreate the application. Do not retry `0.1.183-rework.7` after `.8` exists.

1. Install the `.8` base unit, verify it, and reload systemd:

   ```bash
   sudo install -o root -g root -m 0644 \
     deploy/updater/sub2api-rework-updater.service \
     /etc/systemd/system/sub2api-rework-updater.service
   sudo systemd-analyze verify /etc/systemd/system/sub2api-rework-updater.service
   sudo systemctl daemon-reload
   ```

2. Record the runtime directory identity and the application, PostgreSQL, and
   Redis container IDs as shown below.
3. Restart the updater with `systemctl restart`. Verify the runtime directory
   identity is unchanged and updater `1.1.3` is healthy and idle.
4. From the existing application, verify `updater.sock` is reachable. Verify the
   application has no Docker socket and all three container IDs are unchanged.
5. Run the raw Compose preflight with the production file order:

   ```bash
   docker compose --project-directory /opt/sub2api \
     -f /opt/sub2api/docker-compose.yml \
     -f /opt/sub2api/docker-compose.updater.yml \
     --env-file /opt/sub2api/.env \
     config --no-interpolate --format json
   ```

   This host-side command checks the file order and raw Compose syntax. The
   following updater `prepare` is the authoritative preflight under the hardened
   service with its private home; do not treat the host command as a substitute.

6. Prepare `0.1.183-rework.8` and require updater state `prepared` before install.
7. Install `0.1.183-rework.8`. Verify migration remains `232`, updater status is
   healthy, the application reaches `updater.sock`, the application has no Docker
   socket, and the application, PostgreSQL, and Redis container IDs remain the
   recorded values.

### Replace An Active Updater

Updater `1.1.6` fixes the source-write window in `1.1.5`: install verifies
application quiescence before taking the pre-update snapshot. Releases advancing
beyond schema249 from `.8` require `minimum_updater_version >= 1.1.6`.
The candidate is pulled and verified before shutdown. If shutdown or backup
preparation fails before candidate activation, the updater restarts and checks
the unchanged source without restoring the database. A failed source restart
remains critical. Accepting a new install revokes the previous rollback/recovery
handle; its backup files remain preserved. Interrupted installs fail closed.

This is source preparation only. Production remains `.7`, schema249, updater
`1.1.5`; replacing the production updater requires separate authorization.

Updater `1.1.5` replaces unsafe post-exposure restore in `1.1.4`. It reads
legitimate schema-v2 state from `1.1.4`, keeps `/v1/status` compatible with the
immutable `.6` application's strict client, and adds `/v1/recovery` for rescue
identity and consent. Replace it only while the deployment is healthy and idle.
An interrupted operation starts in `critical` and quiesces the application;
restarting the updater does not resume an install or infer recovery from schema.

Production remains `.6` at schema244 until a separately authorized production
updater installation and qualification. This source change does not authorize
production installation or `.7` release preparation.

Release `0.2.3-rework.1` requires updater `1.1.4` before preparing or installing
the release. Follow this replacement procedure while `.13` remains healthy and
idle. Version `1.1.4` accepts the existing `1.1.3` policy/state and keeps the
status response schema compatible with the running `.13` application. New
backups record checksums required for explicit post-success database recovery.

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
   migration `232` before preparing or installing `0.1.183-rework.8`.

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
2. disables restart and stops all application-service containers, including
   migration one-offs, then verifies their stopped state;
3. pins `SUB2API_IMAGE` to the approved digest;
4. runs `/app/sub2api --migrate` in a one-shot application container;
5. durably records `EXPOSURE_POSSIBLE` before starting the candidate application;
6. checks `/health`, `/`, public frontend settings, PostgreSQL, Redis, and the
   exact migration state;
7. records the bounded audit result.

The checks send no provider credentials and no paid model traffic.

## Rollback Behavior And Limits

There are three distinct paths:

1. A failed install before candidate exposure may restore its pre-update snapshot.
2. Ordinary manual rollback is application-only and allowed only with an
   unchanged migration and no incomplete recovery.
3. Recovery after exposure requires **Prepare recovery**, then a separate
   **Restore pre-update database and roll back** acknowledgement bound to the
   verified rescue snapshot.

The updater persists `PRE_EXPOSURE` with each new pre-update backup. Migration
execution uses the application's migration-only command, which does not serve
traffic. Before any candidate service launch attempt, it saves and syncs
`EXPOSURE_POSSIBLE`, including the state directory. Failed health or an unsuccessful
launch command never clears this record. Missing exposure metadata in old state
means unknown exposure, not proof of safety.

Before exposure, a failed migration may restore the verified pre-update dump,
environment, and immutable source image, then require source-schema and health
checks. After exposure becomes possible, every install failure suppresses
automatic database restore and source startup. It uses a fresh bounded timeout
to quiesce the candidate, preserves the current database and pre-update backup,
and enters durable `critical` state. It disables Docker restart and verifies all
containers carrying the deployment's application-service labels are stopped,
including one-offs. Prepare, install, and ordinary rollback are blocked in that
state. A failure to verify shutdown remains critical and reports a fixed reason.

Manual rollback is allowed only to the updater's recorded previous version and
only while the database migration number still equals the backup's source
migration. It is blocked after schema advancement because substituting an older
application without restoring the database is not generally safe.

Exact `.6` at `2d61454ebfd43f36baee38bf55bf01a4b6ffd2c3` is **not compatible
with schema249**: migrated group reasoning-effort multipliers can disappear
through its pricing read/update behavior. `dot6_forward_compatible_with_schema249=false`.
The accepted `.6` artifacts are immutable. No recovery path may start `.6` until
the database has been restored to its recorded source schema244. Future
schema-advancing releases from `.7` require `minimum_updater_version >= 1.1.5`.
Historical manifests retain their original minimum-version semantics.

### Two-Stage Destructive Recovery

Stage 1 accepts only the recorded source version and `PREPARE RECOVERY <version>`.
Under the operation lock it validates the pre-update backup, quiesces the
application and verifies shutdown, then creates a new custom-format dump of the
current database with current schema, version, environment and Compose metadata.
It syncs the files, verifies their checksums and `pg_restore --list`, and persists
the rescue identity. The application remains stopped with restart disabled.

Stage 2 accepts only the exact `confirmation` returned by `GET /v1/recovery`.
That acknowledgement names the source version, rescue operation ID, rescue
checksum, and current authorization generation. It states that the active
database will revert to the pre-update snapshot. The updater consumes the
generation durably before commands, revalidates both backups and the stopped
container identities/start timestamps, restores the old snapshot, then pins and
starts the source image only at its source schema. It validates source image,
schema, Redis and HTTP before recording completion. The current-data rescue
remains under the private backup tree after success, failure, or restart; the
updater never deletes or overwrites a completed rescue.

`RESTORE DATABASE AND ROLLBACK <version>` is rejected before any commands. Old
`1.1.4` successful state without exposure/rescue metadata must explicitly prepare
a current rescue first. Older dumps without checksums do not qualify for this
destructive path. Requests never carry filesystem paths, images, SQL, or commands.

The gateway stops during Stage 1, so its admin page cannot poll or execute Stage 2.
Use an independent host session to inspect and finish recovery. The UI stops
polling after repeated connection failures and points to this host workflow:

```bash
sudo curl --fail --unix-socket /run/sub2api-rework-updater/updater.sock \
  http://updater/v1/recovery
```

Read the recorded source version and rescue identity. Submit a JSON request to
`http://updater/v1/recover` over the same socket containing `version`,
`actor: "admin:<operator-id>"`, and the **exact returned confirmation**. Stage 2
is never submitted automatically. Socket access is a privileged host boundary;
the actor is an audit label, not independent host authentication.

If recovery stops during database recreation or restore, it preserves the
original rescue and rotates consent for retry. A schema number alone does not
prove that restore completed. If source startup was attempted, a retry first
prepares a new rescue of the current database and retains the earlier rescue.
Completed or stale consent cannot be replayed. A container restarted or replaced
after rescue preparation invalidates that rescue generation for destructive use.

## Staging Qualification

Before any production installation:

1. Clone the production Compose topology with synthetic credentials and data in
   a deployment directory other than `/opt/sub2api-rework/deploy`.
2. Start the application with a base file plus the updater socket override.
3. Model a root-owned, non-root-group `/var/log` ancestor with mode `0775`.
   Confirm a custom audit path below it fails, then start updater `1.1.5` with
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
9. Force failure before candidate launch and require automatic snapshot restore
   and healthy source schema. Separately fail candidate launch and health: require
   current database preservation, verified shutdown, critical state, no automatic
   restore, and no `.6` launch against schema249.
10. Force restore failure and confirm the visible `critical` state and audit.
11. Send concurrent operations and confirm only one acquires the lock.
12. After success write a sentinel, reject the old one-stage confirmation, prepare
    a rescue, restart the updater, and execute the exact new acknowledgement.
    Restore the retained rescue into a separate disposable database and verify
    the sentinel and schema249. Tampered backups and failed dump, disk, checksum,
    metadata, or audit writes must never fall through to destructive restore.
13. Exercise interrupted recreation, restore, source startup and validation.
    Preserve both backups and require fresh consent for retry. Run exact `.6`
    only against a separate disposable compatibility database to retain its
    expected negative multiplier qualification.

## Emergency Recovery

If updater state is `critical`, stop automated attempts. Preserve
`/var/lib/sub2api-rework-updater`, any configured audit path outside that tree,
and the deployment directory before changing anything. Inspect the bounded
updater audit and Docker service state locally; service logs remain in the
systemd journal and are not exposed in the UI.

Do not prepare or install another release while state is `critical`. Use the
two-stage recovery operation above; a new install would replace the metadata needed for
the current incident. Do not edit updater state, migration records, images, or
database contents by hand to simulate recovery.

## Disable The Updater

Disable the host service and remove the Compose override/socket mount:

```bash
sudo systemctl disable --now sub2api-rework-updater.service
```

The watcher remains read-only and the Settings page reports the updater as
unavailable. Do not delete backups or state until the last update is verified.
