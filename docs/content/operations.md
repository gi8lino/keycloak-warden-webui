# Operations

## Health and readiness

Warden Web UI has two HTTP listeners. By default, the application listens on `127.0.0.1:8082`, and the management listener on `127.0.0.1:8083`.

| Path       | Listener    | Purpose                          |
| ---------- | ----------- | -------------------------------- |
| `/`        | Application | Authenticated dashboard          |
| `/setup`   | Application | One-time administrator bootstrap |
| `/healthz` | Management  | Process health                   |
| `/readyz`  | Management  | Database readiness               |

Only the **application** listener is subject to `--route-prefix`. Keep the management listener private; it does not serve the administrative UI.

## Background workers

The **Database maintenance** page reports three independent Warden workers:

- **Enforcement:** ban expiry, current-revision pending/retrying/exhausted delivery counts, active bans, next deadline and last attempt.
- **Notifications:** incident delivery lifecycle (or not configured), retained delivered/pending/retrying/exhausted counts, next deadline and last attempt.
- **Maintenance:** process state, schedule and last successful prune/optimize tasks.

This is a read-only snapshot from Warden's authenticated `GET /workers` endpoint. A temporary failure retrieving the worker snapshot does not disable maintenance operations. Counts are not proof of remote firewall enforcement.

## Database maintenance

The **Database maintenance** page shows Warden's housekeeping status and can trigger two operations through the server-side administrator API:

- **Prune:** remove expired event observations, incidents and terminal notification history according to Warden's retention settings.
- **Optimize:** run SQLite `PRAGMA optimize` through Warden; this is not a full `VACUUM`.

These operations do not delete existing ban decisions or destination acknowledgements. Warden owns the maintenance scheduler and persistence logic, not Web UI.

## Persistent state and backups

Web UI stores local accounts, password hashes, identity links, pending sign-ins, theme preferences, and hashed sessions in its **own** SQLite database. Keep the database file and companion WAL/SHM files on writable persistent storage. Back up this database before making operational changes. Keep it separate from Warden's enforcement database.

For Kubernetes, use a single replica per database and `Recreate` update strategy. The included manifests do this for both deployment variants.

## Logs and diagnostics

Enable `--access-log` to see HTTP requests, and `--debug` for additional diagnostics. For errors retrieving bans or maintenance information, verify Warden's application URL, token, API compatibility, and network connectivity. An inaccessible Warden API does not grant browser users direct administrative access.

Use `make test` for Go unit tests, `make test-race` for the race detector, and `make test-auth-e2e` for Playwright's local/proxy/Keycloak browser sign-in suite (requires Node.js, Chromium and Docker). See [Getting started](getting-started.md) for local Compose testing.
