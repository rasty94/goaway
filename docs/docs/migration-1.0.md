# Migrating to 1.0.0

This guide covers upgrading an existing 0.9x installation (0.9.0 – 0.96.0) to 1.0.0.

## Before you upgrade

1. Stop GoAway.
2. Back up both directories:
    - `config/` (holds `settings.yaml`)
    - `data/` (holds `database.db`, plus `database.db-wal` / `-shm` if present)
3. Read [Security](#security) if your `settings.yaml` was ever copied from this repository.

A backup is the only supported rollback path, so do not skip step 2.

## What happens on first start

Both upgrades run automatically. No manual migration steps are needed.

### Configuration (`settings.yaml`)

The settings file carries a `schemaVersion`. 1.0.0 uses schema version **2** and upgrades older files in place, then writes them back to disk:

| From | Change |
| --- | --- |
| no `schemaVersion` (0.9.0) | Set to `1`, then upgraded as below. |
| `schemaVersion: 1` | `dns.upstream.preferred` and `dns.upstream.fallback` are converted into `dns.upstream.servers` entries (protocol `udp`, enabled), then cleared. |
| any | Missing defaults are filled in: `highAvailability.mode: primary`, `highAvailability.replicaSyncInterval: 15m`, `remoteBackup.schedule: manual`, and `dns.dnssec.mode` (`off` unless already `permissive` or `strict`). |

GoAway refuses to start if:

- `schemaVersion` is newer than 2, which means the config was written by a later release;
- `dns.dnssec.mode` holds a value other than `off`, `permissive` or `strict`.

Every feature added since 0.9x is opt-in and off by default. That covers HA, OIDC/LDAP, 2FA/WebAuthn, DoQ/DoH3, the Redis cache and query log export. See the [CHANGELOG](https://github.com/rasty94/goaway/blob/main/CHANGELOG.md) for the full list.

### Database

GoAway creates any missing tables and columns on startup. Existing data is kept. New tables include the replication outbox, TOTP and WebAuthn credentials, and DHCPv6 leases.

## Distribution changes

- **Container images** are published to `ghcr.io/rasty94/goaway` for `linux/amd64` and `linux/arm64`. Image tags have **no `v` prefix**: use `ghcr.io/rasty94/goaway:1.0.0` or `:latest`.
- **Binaries and checksums** are attached to each [GitHub release](https://github.com/rasty94/goaway/releases).
- **Helm chart**: `image.tag` now defaults to the chart `appVersion`. If you pinned `image.tag: "v0.96.0"`, change it to `1.0.0` or remove the override.
- **Building from source** requires Go 1.27.4, Node 26 and pnpm 10.23.0.

## Security

`settings.yaml` is no longer tracked in git. Earlier revisions of the repository contained a real `jwtSecret`. If an instance starts with that leaked value, 1.0.0 replaces it with a new random secret, saves it and logs a warning. Existing sessions are invalidated and users must log in again.

To rotate the secret yourself at any time, clear `api.jwtSecret` (set it to `""`) and restart; GoAway generates a new one on startup.

## Stateless profile (new, optional)

Set `misc.stateless: true` (or pass `--stateless` / `GOAWAY_STATELESS=true`) to run with an in-memory database, a read-only API and no dashboard. This mode is meant for multi-replica deployments managed by GitOps; see `charts/goaway/values-stateless.yaml`. Nothing persists across restarts in this mode.

## Rollback

1. Stop 1.0.0.
2. Restore the `config/` and `data/` backups taken before the upgrade.
3. Start the previous version.

Do not start an older binary against the upgraded files:

- 0.9.0 does not know about `dns.upstream.servers`, so it would start with no upstreams configured;
- versions before 0.93.0 also ignore `schemaVersion`.

Releases from 0.93.0 onwards support schema version 2, but they may not understand tables or settings added later.
