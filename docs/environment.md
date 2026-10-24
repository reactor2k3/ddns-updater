# Environment settings

These settings apply to the production binary and Docker image.
Recreate the production container after changing its environment.

Defaults below refer to the Dockerfile.

| Variable | Default | Purpose |
| --- | --- | --- |
| `CONFIG` | Empty | Native JSON configuration; overrides the file and makes web management read-only. |
| `PERIOD` | `5m` | Periodic public-IP discovery and DNS checks. |
| `UPDATE_COOLDOWN_PERIOD` | `5m` | Minimum interval between provider updates per record, including manual checks. |
| `PUBLICIP_FETCHERS` | `all` | Enable HTTP, DNS or both for public-IP discovery. |
| `PUBLICIP_HTTP_PROVIDERS` | `all` | Generic IPv4-or-IPv6 lookup services used by Automatic records. |
| `PUBLICIPV4_HTTP_PROVIDERS` | `all` | IPv4-only lookup services. |
| `PUBLICIPV6_HTTP_PROVIDERS` | `all` | IPv6-only lookup services. |
| `PUBLICIP_DNS_PROVIDERS` | `all` | DNS services for discovering the public IP. |
| `PUBLICIP_DNS_TIMEOUT` | `3s` | Public-IP DNS query timeout. |
| `HTTP_TIMEOUT` | `10s` | HTTP client timeout; individual operations can impose an additional deadline. |
| `RESOLVER_ADDRESS` | System DNS | Resolver for configured domain records, separate from public-IP discovery. |
| `RESOLVER_TIMEOUT` | `5s` | Domain lookup timeout. |
| `SERVER_ENABLED` | `yes` | Enable the authenticated web interface and API. |
| `LISTENING_ADDRESS` | `:8000` | Internal webserver address. |
| `ROOT_URL` | `/` | URL path prefix, for example `/ddns`. |
| `AUTH_ORIGIN` | `http://localhost:8000` | Exact browser origin without a path; HTTPS or HTTP on a private LAN IP. |
| `AUTH_ALLOW_INSECURE_HTTP` | `true` | Permit password login over HTTP on private IP origins from private/loopback clients; set `false` for HTTPS-only outside localhost. Passkeys require trusted HTTPS (localhost exception). |
| `AUTH_SETUP_KEY` | Generated | Optional initial setup key of at least 24 characters; no longer accepted after account creation. |
| `DATADIR` | `/updater/data` | Authentication, device grants and IP-history directory. |
| `CONFIG_FILEPATH` | `/updater/data/config.json` | Native configuration and web-entry metadata; backups use this actual path. |
| `BACKUP_PERIOD` | `0` | Automatic ZIP backup interval; zero disables backups. |
| `BACKUP_DIRECTORY` | `/updater/data` | Backup output directory. |
| `HEALTH_SERVER_ADDRESS` | `127.0.0.1:9999` | Internal healthcheck listener. |
| `HEALTH_HEALTHCHECKSIO_BASE_URL` | `https://hc-ping.com` | Optional healthchecks.io-compatible endpoint. |
| `HEALTH_HEALTHCHECKSIO_UUID` | Empty | Optional check identifier. |
| `LOG_LEVEL` | `info` | Log verbosity; saved provider credentials remain omitted. |
| `LOG_CALLER` | `hidden` | Caller information in logs. |
| `SHOUTRRR_ADDRESSES` | Empty | Notification destinations. |
| `SHOUTRRR_DEFAULT_TITLE` | `DDNS Updater` | Notification title. |
| `TZ` | Runtime default | Server timezone; UI dates use the browser locale/timezone. |
| `UMASK` | System default | File-creation mask on supported operating systems. Sensitive state explicitly requests owner-only permissions. |

The `AUTH_*` options are environment variables, not native flags. Native
settings continue to use the original settings reader and its supported flags
and legacy aliases. Dockerfile defaults for `CONFIG_FILEPATH` and
`BACKUP_DIRECTORY` are explicit: changing `DATADIR` alone does not rewrite them.

## Address selection

The listed lookup services discover your IP; DNS providers such as IPv64 are
configured separately in your records. IPv4 + IPv6 uses both family-specific
fetchers. Automatic uses the generic fetcher and returns one address family.
Depending on `PUBLICIP_FETCHERS`, discovery uses HTTP, DNS or both. The three
HTTP-provider lists control only their respective HTTP lookups; DNS lookups use
`PUBLICIP_DNS_PROVIDERS`.
Failures of the selected generic services are reported instead of silently
substituting the IPv4-only or IPv6-only list. Recent results are cached for up to
30 seconds. IP-only refreshes never update DNS records.

## Backup contents and restore

The browser provides **Account & security → Backups**. Download creates a ZIP
on the Go server and lets the browser save it locally. Restore accepts a ZIP from
the same updater (maximum 8 MiB compressed, 16 MiB expanded), requires a sign-in
within the last five minutes and checks origin, authentication and archive contents.
Unsafe paths, duplicate entries, malformed JSON and invalid account, provider or
history data are rejected before any current data is replaced.

The server stops its services, saves a safety ZIP in `BACKUP_DIRECTORY`, restores
the known files and reloads itself. No container restart or console access is
needed. The UI reports the last restore outcome after sign-in. All restored DNS
records start **paused**; review them before enabling updates. Sign in with the
account from the backup. Existing passkeys still require their original hostname.
Container environment variables are not part of the ZIP. Restoring while `CONFIG`
supplies the configuration is unsupported; download remains available.

Use a Linux Docker named volume as in the supplied Compose configuration.
The local restore tests use that layout. A mapped Windows-drive bind mount failed
atomic account persistence in the local Docker Desktop environment.

Backups include the actual configuration file, `updates.json`, `auth.json`
(account hash, passkeys, device names and trusted-device grants), and a pending
`.setup-key` when present. Missing initial history/authentication files are
optional; the configuration itself is required.

ZIP entries use basenames; a custom configuration filename is preserved.
Colliding basenames are rejected. Archives are written with owner-only
permissions to a temporary file and renamed only after successful completion.
Failed archives are removed. Restored file permissions are marked owner-only.
On Windows, access also depends on the containing directory's ACLs.

Archives are not encrypted and include provider credentials. Store them privately.
JSON inputs are validated before an archive is published, but independently
changing files do not form a cross-file transaction. Stop the service for a consistent full-volume
backup or restore. Restore configuration to `CONFIG_FILEPATH` and authentication
and history to `DATADIR`. See [install.md](install.md).
