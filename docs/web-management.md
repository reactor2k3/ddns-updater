# Managing DNS records in the web interface

The Go server serves the embedded, responsive interface and an authenticated
management API. No PHP, Node service, or database server is required in production.
The editable frontend lives in `internal/server/web` and is embedded directly.
Run `node scripts/sync-ui.cjs` from the repository root after editing provider
fields to regenerate server validation metadata.

## Records and persistence

Create, edit, pause, resume and delete records in the interface. Each provider uses
the upstream implementation and validation. IPv4 + IPv6 creates two independent
runtime records under one card. The automatic option selects one family; it does
not mean dual stack. Unsupported provider/IP combinations are rejected.

The configured `CONFIG_FILEPATH` (normally `DATADIR/config.json`) is written
atomically with file mode 0600 on Linux. Mount the whole data directory, rather
than a single config file, so atomic replacement works. Ordinary native `settings`
are imported on first use. `ui_entries` retains stable IDs, revisions, descriptions,
paused records and credentials; `settings` contains the active native settings.
Once `ui_entries` exists, use the interface to edit it. Manually editing only
`settings` would leave the managed configuration inconsistent.

If the environment variable `CONFIG` is set, the interface is read-only. Remove it
to manage settings in the interface; otherwise the deployment environment would
overwrite web changes on restart. Preserve and protect the data directory and
backups: provider credentials must be stored there for unattended updates.

## Updates and status

`PERIOD` controls checks (default 5 minutes). An update check discovers the required
public addresses and compares them with DNS. Provider updates are sent only when
necessary, subject to `UPDATE_COOLDOWN_PERIOD` (default 5 minutes) and provider
abuse bans. The manual refresh button requests this same check; it does not bypass
cooldowns. UI polling only reads local state and never triggers provider updates.

Public IPv4 and IPv6 discovery runs at startup and every `PERIOD`, independently
of active records. The panel immediately above the footer shows its own actual
timestamp and unavailable families explicitly. Its refresh button only discovers
addresses; it never updates DNS. Manual discovery is limited to once per 30 seconds.
DNS checks reuse recently discovered addresses. Automatic family selection uses
the generic fetcher configured by `PUBLICIP_HTTP_PROVIDERS`; select dual stack to
update both families using their separately configured fetchers.
Each card has right-aligned copy buttons for its saved addresses. Missing or unused
addresses cannot be copied. Clipboard access requires HTTPS or localhost.

Pausing removes all corresponding runtime records, including both families of a
dual-stack entry. A change waits for an in-flight update before being committed.
Deleting a local entry does not delete the DNS record at the provider.

Cards distinguish pending, updating, ready, failed and paused states. They show
actual addresses from update history, rather than sample addresses. Activity
combines configuration changes and the existing persistent IP-update history.
Logs and API error messages omit provider secrets; saved secret values are never
returned by the management API. Leave a secret field empty when editing to keep it.
Changing ordinary optional fields to blank restores the provider default.

## API

All routes require the normal authenticated session and same-origin mutation
checks. Every mutation is size-limited and validated on the server.

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/entries` | Redacted entries, activity, runtime status |
| POST | `/api/entries` | Create a record |
| PUT | `/api/entries/{id}` | Edit a record, supplying its current revision |
| DELETE | `/api/entries/{id}` | Delete a record, supplying its current revision |
| GET | `/api/status` | Same current snapshot |
| POST | `/api/update` | Queue a check; 202 means accepted, not successful |
| POST | `/api/public-ips/refresh` | Queue IP discovery only; 202 accepted, 429 rate limited |

Paths are relative to `ROOT_URL`. A stale revision is rejected with 409 instead of
silently overwriting another edit. Provider keys are accepted only in request
bodies, never management URLs. No provider credentials are stored in localStorage.

## Local development and tests

From the repository root, run `go build -o ddns-updater ./cmd/ddns-updater`.
Set `AUTH_ORIGIN` and `DATADIR` for your local installation, then run the binary.
This is the real update engine: enabling real credentials can change DNS.

Run `go test ./...` and `go vet ./...`.
Use `go test -race ./...` where a supported C toolchain is available.
Tests use temporary data, fake public IPs and local transports; they do not update
the IPv64 service or other real providers.

For deployment, follow [authentication.md](authentication.md), set the exact HTTPS
`AUTH_ORIGIN`, preserve the single administrator's authentication data and keep
the data directory private. The UI's small asset-only service-worker cache does
not cache authenticated pages, API responses or credentials.
