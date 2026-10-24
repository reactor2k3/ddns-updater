# Install reactor2k3's DDNS Updater

This is an independent extension of qdm12/ddns-updater. The original provider
implementations and MIT license are retained. The default Compose file pulls
the published latest image. Use `compose.build.yaml` to build a source checkout.

## Docker Compose

Download the main Compose file and `.env.example`. Copy `.env.example` to `.env`,
set `WEB_HOST` to your Docker host's private LAN IP and choose `WEB_PORT` (default
8095). From that directory:

```sh
docker compose up -d
docker compose logs ddns-updater
```

Open `http://YOUR_SERVER_IP:8095`. Use the initial setup key from the logs to create
the single administrator account. Then add your own provider and credentials.
The repository does not supply an administrator, credentials or live DNS records.
Keep the default five-minute checks and cooldown unless your provider requires
something different. Select IPv4 + IPv6 explicitly for dual stack.

The default Compose file uses host networking and listens on all interfaces.
Private LAN IPs can use HTTP password login by default. For passkeys, deploy an
HTTPS reverse proxy, preserve its public Host header and set `AUTH_ORIGIN` to the
exact public origin, for example `https://ddns.example.com`. Adjust Compose's
networking for your proxy. For direct LAN HTTP password login, set `AUTH_ORIGIN`
to the private IP URL and `LISTENING_ADDRESS` to `:PORT`. HTTP is unencrypted and
passkeys are disabled; the UI displays a warning. See
[authentication.md](authentication.md) for passkeys and setup details.

The `ddns-data` named volume persists account hashes, passkeys, trusted devices,
provider credentials, paused entries and update history. Protect it and its
backups. Provider credentials must be readable by the updater for unattended
operation; they are not password hashes. Never commit this volume's contents.

## Update, backup and restore

In **Account & security → Backups**, download a ZIP file or restore a saved ZIP.
The backend creates the snapshot, validates uploads and performs the restore.
A fresh sign-in is required. Restoration creates a safety backup, pauses all DNS
entries, revokes sessions and reloads the application. Store downloaded backups
securely: they contain credentials and authentication data.

For a manual filesystem backup, stop the application before copying the entire data volume.
To update the published image, run `docker compose pull` and then
`docker compose up -d`, keeping the existing data volume. For source builds,
fetch your selected version and run `docker compose -f compose.build.yaml up -d --build`.
Use `docker compose down` to stop
without deleting data. **Do not add `--volumes` when retaining your installation.**

Restore a backup into the data volume only while the service is stopped, then
start the same or a compatible newer version. Keep the previous image and backup
until the update is verified. The original native `settings` configuration can be
imported; after web management is used, retain `ui_entries` as well, including
paused records. See [web-management.md](web-management.md).

If both password and all passkeys are lost, no email recovery or backdoor exists.
An administrator with access to the host can restore a protected backup. Account
reinitialization requires deliberately replacing authentication state while the
service is stopped; it revokes existing devices and requires first-time setup.
It must not be exposed as a public password-reset endpoint.

## Development and verification

The complete editable frontend is in `internal/server/web`, embedded in the Go
binary. Authentication assets are in `internal/auth/ui`. After changing provider
fields, run `node scripts/sync-ui.cjs` to regenerate server validation metadata.
Node is only used for development and is not required by the running application.

```sh
go test ./...
go vet ./...
go build -o ddns-updater ./cmd/ddns-updater
```

CI also runs the race detector, a reachable-vulnerability scan, Compose validation,
and a first-start/restart Docker smoke test. Publishing a GitHub release under
the reactor2k3 account builds AMD64 and ARM64 images in GHCR only after these
checks pass. Prereleases do not overwrite `latest`. No image has been published
merely by adding the workflow. Docker Hub publishing is optional and disabled
by default; see [publishing.md](publishing.md).
