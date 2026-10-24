# Release verification — 22 October 2026

This report records the local checks performed on reactor2k3's extension. It is
not a certification or a guarantee against all vulnerabilities. GitHub CI and
registry publication have been prepared locally but have not run on a fork yet.

## Automated checks passed

| Check | Result |
| --- | --- |
| Windows `go test -count=1 ./...` | Passed |
| Windows `go vet ./...` | Passed |
| Linux container `go test -race -count=1 ./...` | Passed; no detected data races |
| Linux container `go vet ./...` | Passed |
| Windows and Linux `govulncheck ./...` | No reachable vulnerabilities reported |
| Provider catalog | 59 providers, 109 fields; examples, help and labels present |
| Browser-script syntax checks | Passed |
| Docker Compose configuration validation | Passed |
| Production Docker image build | Passed, Linux AMD64 |
| Container healthcheck | Healthy |

The vulnerability scanner also identified six advisories in required modules,
with no affected imported packages or reachable calls reported. This is a
point-in-time result and must be repeated as dependencies and advisories change.

Production HTTP integration tests cover first-time setup with a one-time key,
single-account enforcement, protected embedded assets and management API,
Secure/HttpOnly/SameSite cookies under HTTPS, missing/foreign Origin rejection,
secret redaction, duplicate rejection, persistence and remembered-device recovery.
The same composition was tested at `/` and below `/ddns`.

Additional authentication tests cover lock expiry and persistence, rate limiting,
malformed storage, concurrent setup, session expiry, trusted-device expiry and
revocation, signed WebAuthn assertions and replay protection. Updater tests use
local fakes for serialization, dual stack, unavailable IPv6, unchanged records,
IP-only discovery and sanitization of provider errors.

## Production container checks passed

Twelve checks used an isolated, disposable volume and synthetic credentials:

1. Anonymous management access rejected.
2. Embedded first-time setup served.
3. Account setup accepted only with its generated setup key.
4. Ordinary session invalid after container restart.
5. Password login with a remembered device accepted.
6. Dual-stack record saved paused; its key omitted from API responses.
7. Container recreation retained volume, account, device grant and paused record.
8. Deletion required the current record revision.
9. Logout removed authenticated access.
10. Original MIT license present inside the image.
11. Application ran as UID/GID 1000:1000.
12. Device name saved and retained after container recreation.

## Visible browser checks passed

- Create and edit a paused record using synthetic credentials; reload and restart.
- Saved credentials retained when their edit field was left blank.
- Provider filtering, floating labels, field examples and section help.
- IPv4/IPv6 state labels and right-aligned clipboard buttons; copied address pasted
  back correctly; missing addresses had disabled copy controls.
- Custom delete modal: cancel, Escape, confirmed removal, German and English,
  light and dark, including 320-pixel mobile width.
- Compact cards and layout at desktop, 320 and 390 pixels; no horizontal overflow
  or detected text-contrast failures in the checked views.
- Five wrong password attempts locked a separate test instance; its lock survived
  restart and rejected the correct password until expiry (expiry tested with a clock).
- Revoking the current trusted device returned to login; sign-in created a new grant.
- Device names could be edited in a translated modal and remained after reload.
  Additional Windows and Linux race tests verified authorization, input limits,
  control-character rejection, reauthentication, clearing and restart persistence.
- The user registered a real passkey using the Bitwarden browser extension and
  confirmed successful sign-out and passkey sign-in in their normal browser.

## Scope and remaining release work

No real provider updates were sent during these final checks. The earlier IPv64
IPv4/IPv6 integration test succeeded; its local record remains paused. Other
providers have not all been exercised with real customer accounts.

AMD64 container runtime was verified locally. The ARM64 image job is prepared,
but its build and operation on ARM hardware were not verified here. Native PWA
installation into an operating system was not manually verified. Public HTTPS
reverse-proxy deployment must be tested with its actual final hostname.

The real administrator, passkey, sessions, trusted devices, test domain and update
key are excluded from the repository. A check of 432 repository files and the
existing Git history found no copy of the real test domain or its provider key.
Automated tests intentionally retain clearly synthetic example domains and keys.

Release checks are repeated by the GitHub Actions workflow before publishing
a versioned image. Consult the workflow run for the exact revision being used.

## ENV and backup follow-up — 23 October 2026

Automatic records again use the configured generic public-IP fetcher. Tests
verify its selection, caching and failure behavior separately from IPv4/IPv6.
Backups now include authentication, passkeys and named-device grants, use the
actual configuration path, and atomically publish owner-only ZIP archives.
Tests cover custom paths, missing optional files before setup, archive contents,
permissions, failed-write cleanup and rejection of partial JSON inputs.

The affected packages passed Windows tests and static analysis and Linux race
tests. The optional Docker Hub workflow passed actionlint. Docker Hub remains
disabled unless explicitly enabled, and no image was published. The repository
and history scan now covers 435 files and still finds no real test-domain/key copy.

## Browser backup management — 23 October 2026

Account & security now separates access, trusted devices and backups using a
desktop side navigation and mobile tabs, and includes the main footer contents.
Manual ZIP download and upload use authenticated Go endpoints with a fresh-login
requirement. Restore validates contents, bounds compressed and expanded input,
saves a safety archive, pauses imported entries and reloads the Go services.

The scheduled backup was restored into a new Linux volume and container using
only disposable credentials and an example.com record. Byte comparisons verified
configuration, account and history files. Password login, an actual signed
synthetic WebAuthn assertion, a named remembered-device grant and another restart
all passed. No real DNS updates were requested.

Windows unit tests and Linux race tests cover the affected packages, including
unauthenticated/cross-origin/stale-session requests, unsafe/duplicate ZIP paths,
compressed oversized payloads, safety archive contents and paused legacy imports.
An initial browser test exposed a missing temporary directory in the scratch image;
configuration validation now runs in memory, with a regression test.

The complete Linux test suite subsequently passed with the race detector, followed
by `go vet ./...`. A browser download was uploaded again after changing the test
entry; the service reloaded itself, required sign-in and restored the original
description. A damaged ZIP was rejected without changes. All three account
categories, the restore dialog and the project footer were checked at 320 pixels
in light/dark mode and German/English, without detected text-contrast failures or
horizontal overflow. Keyboard navigation between categories also passed.

A second isolated ZIP restore additionally checked that both historical IPv4 and
IPv6 addresses reappeared through the records API. The real IPv64 test record
remained paused throughout these checks.

The final backup validation also rejects malformed or duplicate trusted-device
grants and empty credential material. A regression test confirms that an absent
device cookie cannot authenticate even if persisted data contains an empty-token
hash. The affected authentication packages passed Windows tests and Linux race
tests/static analysis. A valid browser restore with a passkey and named device
was repeated successfully after this hardening.

Active main navigation, record filters and provider-menu options now use the same
three-pixel accent as the account categories. The browser check exercised all
main views and the mobile navigation drawer.
