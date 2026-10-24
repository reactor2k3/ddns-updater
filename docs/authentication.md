# Single-administrator login

The web server requires authentication. There is exactly one administrator account;
there is no public registration or user-management endpoint.

## First start

Set `AUTH_ORIGIN` to the exact public origin, for example
`https://ddns.example.com`. An origin contains a scheme, hostname and optional
port, but no path. `ROOT_URL` continues to select a path prefix separately.
The hostname must be preserved by the reverse proxy. Terminate HTTPS at the
proxy and redirect plain HTTP to HTTPS. Passkeys require HTTPS with a trusted
certificate and a suitable domain name, except on `localhost` during development.
A certificate-warning override may still leave passkeys unavailable.

HTTP password login is enabled by default for private LAN IP origins, such as
`http://192.168.1.100:8095`. Set `LISTENING_ADDRESS=:8095` for LAN access.
Only private or loopback client IPs may use this mode; forwarded headers do not
bypass the restriction. Public-IP origins and hostname-based HTTP origins are
rejected. Set `AUTH_ALLOW_INSECURE_HTTP=false` to require HTTPS outside localhost.

HTTP carries passwords, sessions and provider credentials without encryption.
The login/account screens show a warning, passkey endpoints are disabled, and
stored passkeys are preserved for future HTTPS use. Setup keys, authentication,
account lockouts, rate limits and exact Host/Origin checks remain enforced.

On first start, the server prints an initial setup key. Open the web interface
and enter that key, a username and a password of 12–128 characters. The key is
required even when the production container is reached through localhost.
It can alternatively be supplied as `AUTH_SETUP_KEY` (at least 24 characters).
It is no longer accepted after the administrator account has been created.

The account and trusted-device grants are stored in `DATADIR/auth.json` with
owner-only file permissions. Mount `DATADIR` as a persistent volume. The file
contains the Argon2id password hash, public passkey records, hashed device
tokens and the temporary account lock. It must be included in protected backups.
Do not edit it while the service is running or share the same authentication
directory between multiple running instances.

## Sign-in and locks

- Passwords use Argon2id with a random 16-byte salt, 64 MiB memory, three passes
  and a 32-byte result. Passwords and bearer tokens are never stored as plaintext.
- Five failed password/passkey verifications within 15 minutes lock new sign-ins
  for 15 minutes. The lock is saved and survives a restart. Existing authorised
  sessions remain usable.
- Authentication mutations are limited to 20 requests per IP per minute. The
  remote socket address is used; arbitrary forwarding headers cannot choose the
  rate-limit key. A reverse proxy can consequently share a limit across clients.
- Cookies are HttpOnly and SameSite=Strict, with Secure enabled for HTTPS origins.
  Session identifiers are regenerated at authentication, hashed in server memory,
  and expire after eight hours. A restart clears ordinary sessions.
- State-changing requests must carry the configured Origin. GET requests do not
  perform updates or account changes. Authentication responses are never cached.
- Host checks, same-origin request checks, security headers and a restrictive
  script CSP protect the login routes. They supplement authentication; they are
  not a guarantee against every attack or a substitute for maintaining the host,
  proxy, dependencies and protected backups.

## Passkeys

After setup or a fresh sign-in, open **Account & security** and add a passkey.
Registration requires a sign-in from the last five minutes; being remembered on
a device alone cannot add credentials. The browser handles fingerprint, face,
device-PIN or security-key interaction. The application never receives the private
key or biometric data.

WebAuthn challenges are random, single-use, browser-session bound for registration,
and expire after five minutes. Verification checks the configured relying-party
ID, origin, challenge, user verification, signature and authenticator counters.
Cloning warnings are rejected. The password remains an alternative sign-in method.

## Staying signed in and trusted devices

**Stay signed in – 30 days** is optional and unchecked by default. It stores a
separate, random HttpOnly device token. Only its SHA-256 hash is stored on disk.
This option permits the browser to obtain an ordinary session after closing the
browser, session expiry or a server restart, for a fixed maximum of 30 days.

Under **Trusted devices**, the administrator can revoke individual devices or
all devices and sessions. Revoking a device invalidates sessions associated with
it, rather than merely removing a row from the list. Signing out also removes
this browser's trusted-device grant. There is a limit of ten trusted devices.

Use the pencil icon to give a device a name such as Desktop, Work laptop or iPhone.
The browser and operating system remain visible underneath. Names are optional,
limited to 64 characters and stored with the device grant. Emptying the field
restores the unnamed state. A name survives server restarts and reauthentication
while that device grant still exists. Signing out removes the grant and its name.
Browsers do not supply a reliable computer hostname, so names are entered manually.

## Embedded interface

The production Go binary embeds the web and authentication assets. First-time
setup requires the generated setup key from the log. Keep its data directory
private and set the exact public `AUTH_ORIGIN` before starting the application.

The service worker caches only the manifest and app icons. It does not cache
authenticated pages, scripts or API responses, so offline fallback cannot bypass
the login screen.
