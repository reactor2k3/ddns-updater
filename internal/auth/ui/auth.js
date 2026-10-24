const root = document.body.dataset.root || "";
const main = document.querySelector("#auth-main");
const esc = (value) =>
  String(value).replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
const keyIcon =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="8" cy="9" r="5"/><path d="m12 12 9 9m-6-6 3-3m0 6 3-3"/></svg>';
const shieldIcon =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 4 6v6c0 4 4 7 8 9 4-2 8-5 8-9V6Z"/><path d="m8 12 3 3 5-6"/></svg>';
const eyeIcon =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/></svg>';
const eyeOffIcon =
  '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 3 18 18M10.6 5.1 12 5c6.5 0 10 7 10 7a18 18 0 0 1-3 3.8M6.2 6.2A21 21 0 0 0 2 12s3.5 7 10 7a12 12 0 0 0 5.8-1.8M9.9 9.9a3 3 0 0 0 4.2 4.2"/></svg>';
let preference = "auto",
  theme = "light";
try {
  preference = localStorage.getItem("ddns-language") || "auto";
  theme =
    localStorage.getItem("ddns-theme") ||
    (matchMedia("(prefers-color-scheme:dark)").matches ? "dark" : "light");
} catch {}
let language =
  preference === "auto"
    ? navigator.language.startsWith("de")
      ? "de"
      : "en"
    : preference;
let state = null,
  busy = false;
const messages = {
  de: {
    setup: "Ersteinrichtung",
    httpNotice:
      "HTTP im lokalen Netz: Passwort, Sitzung und Zugangsdaten werden unverschlüsselt übertragen. Passkeys sind hier deaktiviert und benötigen HTTPS mit einem vertrauenswürdigen Zertifikat.",
    certificateNotice:
      "Passkeys benötigen HTTPS mit einem vertrauenswürdigen Zertifikat. Eine Zertifikatswarnung kann Passkeys blockieren. Ausnahme: localhost bei lokaler Entwicklung.",
    PASSKEY_REQUIRES_HTTPS:
      "Passkeys benötigen HTTPS mit einem vertrauenswürdigen Zertifikat.",
    login: "Anmeldung",
    account: "Konto & Sicherheit",
    setupTitle: "Dein Zugang",
    loginTitle: "Willkommen zurück",
    accountTitle: "Dein Konto",
    setupIntro:
      "Erstelle das einzige Administratorkonto für deinen DDNS Updater.",
    loginIntro: "Melde dich an, um deine Domains zu verwalten.",
    accountIntro: "Passkeys und angemeldete Geräte an einem Ort.",
    username: "Benutzername",
    password: "Passwort",
    confirm: "Passwort wiederholen",
    setupKey: "Einrichtungsschlüssel",
    setupKeyHelp:
      "Den einmaligen Schlüssel findest du in der Ausgabe des Servers oder in den Container-Logs.",
    usernameHelp:
      "3–64 Zeichen: Buchstaben, Zahlen, Punkt, Bindestrich oder Unterstrich.",
    passwordHelp:
      "Mindestens 12 Zeichen. Ein langer, einzigartiger Satz eignet sich gut.",
    show: "Anzeigen",
    hide: "Verbergen",
    remember: "Angemeldet bleiben – 30 Tage",
    rememberHelp:
      "Nur auf deinem eigenen Gerät. Du kannst den Zugang später unter „Vertraute Geräte“ widerrufen.",
    create: "Konto erstellen",
    signIn: "Anmelden",
    passkeySignIn: "Mit Passkey anmelden",
    or: "oder mit Passwort",
    busy: "Bitte warten …",
    security:
      "Nach 5 Fehlversuchen wird die Anmeldung für 15 Minuten gesperrt. Ohne „Angemeldet bleiben“ endet die Sitzung nach 8 Stunden.",
    footer: "Ein Konto. Dein Netzwerk.",
    dashboard: "Zur Übersicht",
    passkeys: "Passkeys",
    passkeyHelp:
      "Melde dich mit Fingerabdruck, Gesichtserkennung oder der Gerätesperre an. Dein privater Schlüssel bleibt beim Passkey-Anbieter.",
    addPasskey: "Passkey hinzufügen",
    passkeyAdded: "Passkey gespeichert. Du kannst dich künftig damit anmelden.",
    passkeyCount: "Gespeicherte Passkeys",
    reauth: "Zum Hinzufügen eines Passkeys bitte erneut anmelden.",
    reauthButton: "Erneut anmelden",
    devices: "Vertraute Geräte",
    devicesHelp:
      "Diese Geräte dürfen bis zu 30 Tage angemeldet bleiben. Nach dem Abmelden benötigen sie wieder dein Passwort oder einen Passkey.",
    noDevices: "Noch keine vertrauten Geräte.",
    current: "Dieses Gerät",
    seen: "Zuletzt verwendet",
    expires: "Gültig bis",
    revoke: "Abmelden",
    revokeAll: "Alle Geräte abmelden",
    confirmAll: "Alle Geräte und Sitzungen jetzt abmelden?",
    logout: "Abmelden",
    deviceRevoked: "Gerät abgemeldet.",
    confirmMismatch: "Die Passwörter stimmen nicht überein.",
    passkeyUnsupported:
      "Dieser Browser unterstützt hier keine Passkeys. Verwende einen kompatiblen Browser über HTTPS oder localhost.",
    passkeyCancelled:
      "Passkey-Vorgang abgebrochen oder nicht verfügbar. Du kannst es erneut versuchen oder dein Passwort verwenden.",
    offline: "Der Server ist nicht erreichbar. Bitte versuche es erneut.",
    welcome: "Konto erstellt",
    welcomeTitle: "Dein Zugang steht",
    welcomeIntro:
      "Füge jetzt einen Passkey hinzu, um dich ohne Passwort anzumelden.",
    skip: "Später einrichten",
    deviceLimit:
      "Maximal 10 vertraute Geräte. Melde zuerst ein altes Gerät ab.",
    LOGIN_FAILED:
      "Anmeldung fehlgeschlagen. Prüfe Benutzername und Passwort oder versuche einen anderen Passkey.",
    ACCOUNT_LOCKED:
      "Die Anmeldung ist nach mehreren Fehlversuchen für 15 Minuten gesperrt. Bitte warte und versuche es danach erneut.",
    RATE_LIMITED:
      "Zu viele Versuche. Bitte warte eine Minute und versuche es erneut.",
    CREDENTIAL_RULES:
      "Verwende einen gültigen Benutzernamen und ein Passwort mit 12–128 Zeichen.",
    SETUP_KEY_INVALID: "Der Einrichtungsschlüssel ist nicht korrekt.",
    SETUP_COMPLETED:
      "Das Administratorkonto wurde bereits eingerichtet. Bitte melde dich an.",
    SETUP_REQUIRED: "Bitte richte zuerst das Administratorkonto ein.",
    AUTH_REQUIRED: "Deine Sitzung ist abgelaufen. Bitte melde dich erneut an.",
    REAUTH_REQUIRED: "Bitte melde dich vor dieser Änderung erneut an.",
    NO_PASSKEY:
      "Es wurde noch kein Passkey eingerichtet. Melde dich mit deinem Passwort an.",
    CHALLENGE_EXPIRED:
      "Der Passkey-Vorgang ist abgelaufen. Bitte starte ihn erneut.",
    CHALLENGE_INVALID:
      "Der Passkey-Vorgang ist nicht gültig. Bitte starte ihn erneut.",
    PASSKEY_INVALID:
      "Der Passkey konnte nicht bestätigt werden. Bitte versuche es erneut.",
    PASSKEY_EXISTS: "Dieser Passkey ist bereits gespeichert.",
    PASSKEY_LIMIT:
      "Maximal 10 Passkeys. Verwende einen bereits eingerichteten Passkey.",
    PASSKEY_UNAVAILABLE: "Passkeys sind zurzeit nicht verfügbar.",
    ORIGIN_REJECTED:
      "Diese Adresse ist für die Anmeldung nicht freigegeben. Verwende die konfigurierte Adresse des Servers.",
    STORAGE_ERROR:
      "Die Zugangsdaten konnten nicht sicher gespeichert werden. Prüfe das Datenverzeichnis des Servers.",
    DEVICE_LIMIT_OR_STORAGE:
      "Das Gerät konnte nicht gespeichert werden. Prüfe die Geräteanzahl und das Datenverzeichnis.",
    UNKNOWN:
      "Die Aktion konnte nicht abgeschlossen werden. Bitte versuche es erneut.",
  },
  en: {
    setup: "First-time setup",
    httpNotice:
      "Local-network HTTP: your password, session and credentials are transmitted without encryption. Passkeys are disabled here and require HTTPS with a trusted certificate.",
    certificateNotice:
      "Passkeys require HTTPS with a trusted certificate. A certificate warning may block passkeys. Exception: localhost during local development.",
    PASSKEY_REQUIRES_HTTPS:
      "Passkeys require HTTPS with a trusted certificate.",
    login: "Sign in",
    account: "Account & security",
    setupTitle: "Your access",
    loginTitle: "Welcome back",
    accountTitle: "Your account",
    setupIntro: "Create the only administrator account for your DDNS Updater.",
    loginIntro: "Sign in to manage your domains.",
    accountIntro: "Your passkeys and signed-in devices in one place.",
    username: "Username",
    password: "Password",
    confirm: "Confirm password",
    setupKey: "Setup key",
    setupKeyHelp:
      "Find the one-time key in the server output or container logs.",
    usernameHelp:
      "3–64 characters: letters, numbers, dot, hyphen or underscore.",
    passwordHelp:
      "At least 12 characters. A long, unique phrase is a good choice.",
    show: "Show",
    hide: "Hide",
    remember: "Stay signed in – 30 days",
    rememberHelp:
      "Only on your own device. You can revoke access under “Trusted devices”.",
    create: "Create account",
    signIn: "Sign in",
    passkeySignIn: "Sign in with a passkey",
    or: "or use your password",
    busy: "Please wait …",
    security:
      "After 5 failed attempts, sign-in is locked for 15 minutes. Without “Stay signed in”, sessions expire after 8 hours.",
    footer: "One account. Your network.",
    dashboard: "Open dashboard",
    passkeys: "Passkeys",
    passkeyHelp:
      "Sign in using your fingerprint, face recognition or device unlock. Your private key stays with your passkey provider.",
    addPasskey: "Add passkey",
    passkeyAdded: "Passkey saved. You can now use it to sign in.",
    passkeyCount: "Saved passkeys",
    reauth: "Please sign in again before adding a passkey.",
    reauthButton: "Sign in again",
    devices: "Trusted devices",
    devicesHelp:
      "These devices may stay signed in for up to 30 days. After revocation, they need your password or passkey again.",
    noDevices: "No trusted devices yet.",
    current: "This device",
    seen: "Last used",
    expires: "Valid until",
    revoke: "Sign out",
    revokeAll: "Sign out all devices",
    confirmAll: "Sign out all devices and sessions now?",
    logout: "Sign out",
    deviceRevoked: "Device signed out.",
    confirmMismatch: "The passwords do not match.",
    passkeyUnsupported:
      "Passkeys are not supported here. Use a compatible browser over HTTPS or localhost.",
    passkeyCancelled:
      "The passkey operation was cancelled or unavailable. Try again or use your password.",
    offline: "The server cannot be reached. Please try again.",
    welcome: "Account created",
    welcomeTitle: "You’re all set",
    welcomeIntro: "Add a passkey now to sign in without a password.",
    skip: "Set up later",
    deviceLimit:
      "A maximum of 10 trusted devices is allowed. Sign out an old device first.",
    LOGIN_FAILED:
      "Sign-in failed. Check your username and password or try another passkey.",
    ACCOUNT_LOCKED:
      "Sign-in is locked for 15 minutes after repeated failed attempts. Please wait and try again afterwards.",
    RATE_LIMITED: "Too many attempts. Please wait a minute and try again.",
    CREDENTIAL_RULES:
      "Use a valid username and a password with 12–128 characters.",
    SETUP_KEY_INVALID: "The setup key is incorrect.",
    SETUP_COMPLETED:
      "The administrator account is already set up. Please sign in.",
    SETUP_REQUIRED: "Set up the administrator account first.",
    AUTH_REQUIRED: "Your session has expired. Please sign in again.",
    REAUTH_REQUIRED: "Please sign in again before making this change.",
    NO_PASSKEY: "No passkey has been set up yet. Use your password to sign in.",
    CHALLENGE_EXPIRED: "The passkey request has expired. Please start again.",
    CHALLENGE_INVALID: "The passkey request is invalid. Please start again.",
    PASSKEY_INVALID: "The passkey could not be verified. Please try again.",
    PASSKEY_EXISTS: "This passkey has already been saved.",
    PASSKEY_LIMIT:
      "A maximum of 10 passkeys is allowed. Use an existing passkey.",
    PASSKEY_UNAVAILABLE: "Passkeys are currently unavailable.",
    ORIGIN_REJECTED:
      "This address is not allowed for sign-in. Use the server’s configured address.",
    STORAGE_ERROR:
      "Credentials could not be saved securely. Check the server’s data directory.",
    DEVICE_LIMIT_OR_STORAGE:
      "The device could not be saved. Check the device limit and data directory.",
    UNKNOWN: "The action could not be completed. Please try again.",
  },
};
messages.de.INVALID_INPUT =
  "Bitte prüfe deine Eingaben und versuche es erneut.";
Object.assign(messages.de, {
  deviceName: "Gerätename",
  nameDevice: "Gerät benennen",
  editDeviceName: "Gerätenamen bearbeiten",
  unnamedDevice: "Unbenanntes Gerät",
  deviceNameHelp:
    "Zum Beispiel Desktop, Büro-Laptop oder iPhone. Leer lassen, um den Namen zurückzusetzen.",
  saveName: "Speichern",
  cancel: "Abbrechen",
  deviceNameSaved: "Gerätename gespeichert.",
  DEVICE_NOT_FOUND: "Dieses Gerät wurde nicht gefunden. Lade die Seite neu.",
});
Object.assign(messages.en, {
  deviceName: "Device name",
  nameDevice: "Name this device",
  editDeviceName: "Edit device name",
  unnamedDevice: "Unnamed device",
  deviceNameHelp:
    "For example Desktop, Work laptop or iPhone. Leave empty to reset the name.",
  saveName: "Save",
  cancel: "Cancel",
  deviceNameSaved: "Device name saved.",
  DEVICE_NOT_FOUND: "This device was not found. Reload the page.",
});
messages.en.INVALID_INPUT = "Please check your input and try again.";
const t = (key) => messages[language]?.[key] || messages.en[key] || key;
Object.assign(messages.de, {
  backups: "Backup & Wiederherstellung",
  backupHelp:
    "Sichere Einträge, Konto, Passkeys und Gerätenamen als ZIP auf deinem Gerät.",
  backupPrivate:
    "Das ZIP ist nicht verschlüsselt und enthält geheime Zugangsdaten. Bewahre es sicher auf.",
  backupDownload: "Backup herunterladen",
  backupRestore: "Backup wiederherstellen",
  backupChoose: "ZIP-Backup auswählen",
  backupFile: "Backup-Datei",
  backupFileHelp: "ZIP-Datei dieses Updaters, maximal 8 MB.",
  backupConfirm: "Backup wirklich wiederherstellen?",
  backupConsequences:
    "Die aktuellen Einträge und das Konto werden durch das Backup ersetzt. Der bisherige Stand wird vorher gesichert. Danach meldest du dich mit dem Konto aus dem Backup erneut an. Alle wiederhergestellten DNS-Einträge starten pausiert.",
  backupDownloaded:
    "Backup erstellt und zum Speichern an deinen Browser übergeben.",
  backupReloading:
    "Backup wird übernommen. Der Dienst lädt neu; anschließend kannst du dich mit dem Konto aus dem Backup anmelden.",
  backupReauth:
    "Bitte melde dich für Backups erneut an. Die Freigabe gilt fünf Minuten.",
  BACKUP_INVALID:
    "Dieses Backup ist ungültig, unvollständig oder nicht kompatibel. Es wurde nichts geändert.",
  BACKUP_TOO_LARGE: "Die ZIP-Datei darf maximal 8 MB groß sein.",
  BACKUP_FAILED: "Das Backup konnte nicht erstellt werden.",
  BACKUP_BUSY: "Eine Wiederherstellung läuft bereits. Bitte warte kurz.",
  BACKUP_UNAVAILABLE: "Backups sind in dieser Instanz noch nicht verfügbar.",
  backupLogin: "Zur Anmeldung",
});
Object.assign(messages.en, {
  backups: "Backup & restore",
  backupHelp:
    "Save records, account, passkeys and device names as a ZIP on your device.",
  backupPrivate:
    "The ZIP is not encrypted and contains secret credentials. Store it securely.",
  backupDownload: "Download backup",
  backupRestore: "Restore backup",
  backupChoose: "Select ZIP backup",
  backupFile: "Backup file",
  backupFileHelp: "ZIP file from this updater, up to 8 MB.",
  backupConfirm: "Restore this backup?",
  backupConsequences:
    "The backup replaces the current records and account. The current state is saved first. Then sign in again using the account from the backup. All restored DNS records start paused.",
  backupDownloaded: "Backup created and handed to your browser for saving.",
  backupReloading:
    "Applying backup. The service is reloading; then sign in using the account from the backup.",
  backupReauth:
    "Sign in again to manage backups. Access is granted for five minutes.",
  BACKUP_INVALID:
    "This backup is invalid, incomplete or incompatible. Nothing was changed.",
  BACKUP_TOO_LARGE: "The ZIP file must not exceed 8 MB.",
  BACKUP_FAILED: "The backup could not be created.",
  BACKUP_BUSY: "A restore is already in progress. Please wait.",
  BACKUP_UNAVAILABLE: "Backups are not available in this instance yet.",
  backupLogin: "Go to sign in",
});
function flag() {
  return language === "de"
    ? '<svg viewBox="0 0 30 20"><path fill="#202327" d="M0 0h30v7H0z"/><path fill="#e34848" d="M0 7h30v6H0z"/><path fill="#ffd25a" d="M0 13h30v7H0z"/></svg>'
    : '<svg viewBox="0 0 30 20"><path fill="#284877" d="M0 0h30v20H0z"/><path stroke="#fff" stroke-width="5" d="m0 0 30 20M30 0 0 20"/><path stroke="#da4251" stroke-width="2" d="m0 0 30 20M30 0 0 20"/><path stroke="#fff" stroke-width="7" d="M15 0v20M0 10h30"/><path stroke="#da4251" stroke-width="4" d="M15 0v20M0 10h30"/></svg>';
}
function updateAppearance() {
  document.documentElement.lang = language;
  document.documentElement.dataset.theme = theme;
  document.querySelector("#flag").innerHTML = flag();
  document.querySelector("#language").value = preference;
  renderFooter();
  document.querySelector("#theme").ariaLabel =
    language === "de" ? "Hell-/Darkmode wechseln" : "Toggle light / dark mode";
}
document.querySelector("#theme").addEventListener("click", () => {
  theme = theme === "dark" ? "light" : "dark";
  try {
    localStorage.setItem("ddns-theme", theme);
  } catch {}
  updateAppearance();
});
document.querySelector("#language").addEventListener("change", (event) => {
  preference = event.target.value;
  language =
    preference === "auto"
      ? navigator.language.startsWith("de")
        ? "de"
        : "en"
      : preference;
  try {
    localStorage.setItem("ddns-language", preference);
  } catch {}
  updateAppearance();
  if (state) render();
});
updateAppearance();
async function api(path, data) {
  let response;
  try {
    response = await fetch(root + "/api/auth/" + path, {
      method: data === undefined ? "GET" : "POST",
      credentials: "same-origin",
      headers: data === undefined ? {} : { "Content-Type": "application/json" },
      body: data === undefined ? undefined : JSON.stringify(data),
      cache: "no-store",
    });
  } catch {
    throw new Error("offline");
  }
  let result = {};
  try {
    result = await response.json();
  } catch {}
  if (!response.ok) throw new Error(result.error || "UNKNOWN");
  return result;
}
function notice(message, error = false) {
  const box = document.querySelector("#message");
  if (box) {
    box.hidden = false;
    box.className = "status-message" + (error ? " error" : "");
    box.textContent = message;
    box.setAttribute("role", error ? "alert" : "status");
  }
}
function field(id, label, type = "text", help = "", extra = "") {
  return `<div class="field"><div class="floating"><input id="${id}" name="${id}" type="${type}" placeholder=" " required ${extra}><label for="${id}">${esc(t(label))}</label>${type === "password" ? `<button class="show-password" type="button" data-show="${id}" aria-label="${esc(t("show") + " " + t(label))}" aria-pressed="false">${eyeIcon}</button>` : ""}</div>${help ? `<small id="${id}-help">${esc(t(help))}</small>` : ""}</div>`;
}
function remember() {
  return `<label class="remember"><input type="checkbox" id="remember"><span>${esc(t("remember"))}<small>${esc(t("rememberHelp"))}</small></span></label>`;
}
function security() {
  return `<div class="security-note">${shieldIcon}<span>${esc(t("security"))}</span></div>`;
}
function transportNotice() {
  return state.insecureHTTP
    ? `<p class="transport-notice" role="note">${esc(t("httpNotice"))}</p>`
    : "";
}
function render() {
  renderFooter();
  const isAccount = location.pathname === root + "/account";
  main.classList.toggle("account-page", isAccount);
  if (isAccount) {
    if (!state.authenticated) {
      location.replace(root + "/login");
      return;
    }
    renderAccount();
    return;
  }
  const setup = state.setupRequired;
  if (setup && location.pathname !== root + "/setup") {
    location.replace(root + "/setup");
    return;
  }
  if (!setup && location.pathname === root + "/setup") {
    location.replace(root + "/login");
    return;
  }
  document.title = "DDNS Updater · " + t(setup ? "setup" : "login");
  main.innerHTML = `<div class="eyebrow">${esc(t(setup ? "setup" : "login"))}</div><h1>${esc(t(setup ? "setupTitle" : "loginTitle"))}</h1><p class="intro">${esc(t(setup ? "setupIntro" : "loginIntro"))}</p>${transportNotice()}<div id="message" hidden></div>${!setup && state.passkeysAvailable && state.passkeys ? `<button class="button primary wide" id="passkey-login" type="button">${keyIcon}${esc(t("passkeySignIn"))}</button><div class="divider">${esc(t("or"))}</div>` : ""}${state.passkeysAvailable ? `<p class="certificate-note">${esc(t("certificateNotice"))}</p>` : ""}<form id="login-form">${setup && state.setupKeyRequired ? field("setup-key", "setupKey", "password", "setupKeyHelp", 'autocomplete="off"') : ""}${field("username", "username", "text", setup ? "usernameHelp" : "", 'autocomplete="username" minlength="3" maxlength="64" pattern="[a-zA-Z0-9_.\\-]+" autocapitalize="none" spellcheck="false"')}${field("password", "password", "password", setup ? "passwordHelp" : "", `autocomplete="${setup ? "new-password" : "current-password"}" ${setup ? 'minlength="12"' : ""} maxlength="128"`)}${setup ? field("confirm", "confirm", "password", "", 'autocomplete="new-password" minlength="12" maxlength="128"') : ""}${remember()}<button class="button ${state.passkeysAvailable && state.passkeys && !setup ? "" : "primary"} wide" id="submit" type="submit">${esc(t(setup ? "create" : "signIn"))}</button></form>${security()}`;
  main.querySelector("form").addEventListener("submit", submit);
  main.querySelector("#passkey-login")?.addEventListener("click", passkeyLogin);
  if (setup) main.querySelector("#username").value = "admin";
}
async function submit(event) {
  event.preventDefault();
  if (busy) return;
  const setup = state.setupRequired;
  const password = document.querySelector("#password").value;
  if (setup && password !== document.querySelector("#confirm").value) {
    notice(t("confirmMismatch"), true);
    return;
  }
  busy = true;
  const button = document.querySelector("#submit");
  button.disabled = true;
  button.textContent = t("busy");
  try {
    const data = {
      username: document.querySelector("#username").value.trim(),
      password,
      remember: document.querySelector("#remember").checked,
    };
    if (setup)
      data.setupKey = document.querySelector("#setup-key")?.value || "";
    await api(setup ? "setup" : "login", data);
    location.assign(
      root +
        (setup
          ? "/account?welcome=1"
          : new URLSearchParams(location.search).has("reauth") ||
              new URLSearchParams(location.search).has("restored")
            ? "/account#backups"
            : "/"),
    );
  } catch (error) {
    notice(t(error.message), true);
    if (error.message === "SETUP_COMPLETED")
      setTimeout(() => location.assign(root + "/login"), 1600);
  } finally {
    busy = false;
    if (button.isConnected) {
      button.disabled = false;
      button.textContent = t(setup ? "create" : "signIn");
    }
  }
}
function decode64(value) {
  const text = value.replace(/-/g, "+").replace(/_/g, "/");
  return Uint8Array.from(atob(text), (char) => char.charCodeAt(0));
}
function encode64(buffer) {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}
function optionsFromJSON(value, registration) {
  const options = value.publicKey;
  if (!options) throw new Error("PASSKEY_UNAVAILABLE");
  options.challenge = decode64(options.challenge);
  if (registration) options.user.id = decode64(options.user.id);
  for (const list of [options.allowCredentials, options.excludeCredentials])
    if (list)
      list.forEach((credential) => (credential.id = decode64(credential.id)));
  return options;
}
function credentialJSON(credential) {
  const result = {
    id: credential.id,
    rawId: encode64(credential.rawId),
    type: credential.type,
    clientExtensionResults: credential.getClientExtensionResults(),
    response: { clientDataJSON: encode64(credential.response.clientDataJSON) },
  };
  const response = credential.response;
  if (response.attestationObject) {
    result.response.attestationObject = encode64(response.attestationObject);
    if (response.getTransports)
      result.response.transports = response.getTransports();
  } else {
    result.response.authenticatorData = encode64(response.authenticatorData);
    result.response.signature = encode64(response.signature);
    result.response.userHandle = response.userHandle
      ? encode64(response.userHandle)
      : null;
  }
  return result;
}
async function passkeyLogin() {
  if (busy) return;
  if (!window.PublicKeyCredential || !window.isSecureContext) {
    notice(t("passkeyUnsupported"), true);
    return;
  }
  busy = true;
  const button = document.querySelector("#passkey-login");
  button.disabled = true;
  try {
    const options = await api("passkey/login/begin", {
      remember: document.querySelector("#remember").checked,
    });
    const credential = await navigator.credentials.get({
      publicKey: optionsFromJSON(options, false),
    });
    if (!credential) throw new Error("passkeyCancelled");
    await api("passkey/login/finish", credentialJSON(credential));
    location.assign(
      root +
        (new URLSearchParams(location.search).has("reauth") ||
        new URLSearchParams(location.search).has("restored")
          ? "/account#backups"
          : "/"),
    );
  } catch (error) {
    notice(
      t(
        ["NotAllowedError", "AbortError", "SecurityError"].includes(error.name)
          ? "passkeyCancelled"
          : error.message,
      ),
      true,
    );
  } finally {
    busy = false;
    button.disabled = false;
  }
}
async function addPasskey() {
  if (busy) return;
  if (!window.PublicKeyCredential || !window.isSecureContext) {
    notice(t("passkeyUnsupported"), true);
    return;
  }
  busy = true;
  const button = document.querySelector("#add-passkey");
  button.disabled = true;
  try {
    const options = await api("passkey/register/begin", {});
    const credential = await navigator.credentials.create({
      publicKey: optionsFromJSON(options, true),
    });
    if (!credential) throw new Error("passkeyCancelled");
    await api("passkey/register/finish", credentialJSON(credential));
    state = await api("state");
    renderAccount();
    notice(t("passkeyAdded"));
  } catch (error) {
    notice(
      t(
        ["NotAllowedError", "AbortError", "SecurityError"].includes(error.name)
          ? "passkeyCancelled"
          : error.message,
      ),
      true,
    );
  } finally {
    busy = false;
    if (button.isConnected) button.disabled = false;
  }
}
function renderAccount() {
  const welcome = new URLSearchParams(location.search).get("welcome") === "1";
  document.title = "DDNS Updater · " + t("account");
  main.innerHTML = `<div class="account-heading"><div><div class="eyebrow">${esc(t(welcome ? "welcome" : "account"))}</div><h1>${esc(t(welcome ? "welcomeTitle" : "accountTitle"))}</h1></div><a class="button" href="${root}/">${esc(t("dashboard"))} ↗</a></div><p class="intro">${esc(welcome && state.passkeysAvailable ? t("welcomeIntro") : state.username + " · " + t("accountIntro"))}</p>${transportNotice()}<div id="message" hidden></div><section class="account-section"><div class="passkey-illustration">${keyIcon}</div><h2>${esc(t("passkeys"))}</h2><p>${esc(t("passkeyHelp"))}</p><p class="certificate-note">${esc(t("certificateNotice"))}</p><p>${esc(t("passkeyCount"))}: <strong>${state.passkeys}</strong></p>${!state.passkeysAvailable ? "" : state.fresh ? `<button class="button primary" id="add-passkey" type="button">${keyIcon}${esc(t("addPasskey"))}</button>` : `<p>${esc(t("reauth"))}</p><a class="button" href="${root}/login?reauth=1">${esc(t("reauthButton"))}</a>`}${welcome ? `<a class="text-action" href="${root}/">${esc(t("skip"))} ↗</a>` : ""}</section><section class="account-section"><h2>${esc(t("devices"))}</h2><p>${esc(t("devicesHelp"))}</p><div id="devices"></div><button type="button" class="text-action" id="revoke-all">${esc(t("revokeAll"))}</button></section><button type="button" class="text-action logout" id="logout">${esc(t("logout"))}</button>`;
  document.querySelector("#add-passkey")?.addEventListener("click", addPasskey);
  document.querySelector("#logout").addEventListener("click", async () => {
    try {
      await api("logout", {});
      location.assign(root + "/login");
    } catch (error) {
      notice(t(error.message), true);
    }
  });
  document.querySelector("#revoke-all").addEventListener("click", async () => {
    if (!confirm(t("confirmAll"))) return;
    try {
      await api("devices/revoke", { all: true });
      location.assign(root + "/login");
    } catch (error) {
      notice(t(error.message), true);
    }
  });
  loadDevices();
  renderBackups();
  organiseAccount();
}
async function loadDevices() {
  try {
    const result = await api("devices");
    const container = document.querySelector("#devices");
    if (!container) return;
    container.innerHTML = result.devices.length
      ? result.devices
          .map(
            (device) =>
              `<div class="device-row"><div class="device-copy"><div class="device-title"><strong>${esc(device.label || t("unnamedDevice"))}</strong><button class="device-edit" type="button" data-rename="${esc(device.id)}" aria-label="${esc(t("editDeviceName"))}" title="${esc(t("editDeviceName"))}"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 5 4 4M4 20l4-1L20 7a2.8 2.8 0 0 0-4-4L4 15Z"/></svg></button>${device.current ? `<span class="current-device">${esc(t("current"))}</span>` : ""}</div><small>${esc(device.name)}</small><small>${esc(t("seen"))}: ${esc(new Date(device.lastSeen).toLocaleString(language))}<br>${esc(t("expires"))}: ${esc(new Date(device.expiresAt).toLocaleDateString(language))}</small></div><button class="button" type="button" data-revoke="${esc(device.id)}">${esc(t("revoke"))}</button></div>`,
          )
          .join("")
      : `<p>${esc(t("noDevices"))}</p>`;
    container
      .querySelectorAll("[data-rename]")
      .forEach((button) =>
        button.addEventListener("click", () =>
          openDeviceName(
            result.devices.find((d) => d.id === button.dataset.rename),
          ),
        ),
      );
    container.querySelectorAll("[data-revoke]").forEach((button) =>
      button.addEventListener("click", async () => {
        button.disabled = true;
        try {
          await api("devices/revoke", { id: button.dataset.revoke });
          state = await api("state");
          if (!state.authenticated) {
            location.assign(root + "/login");
            return;
          }
          await loadDevices();
          notice(t("deviceRevoked"));
        } catch (error) {
          notice(t(error.message), true);
          button.disabled = false;
        }
      }),
    );
  } catch (error) {
    notice(t(error.message), true);
  }
}
function openDeviceName(device) {
  if (!device) return;
  let modal = document.querySelector("#device-name-dialog");
  if (!modal) {
    modal = document.createElement("dialog");
    modal.id = "device-name-dialog";
    modal.className = "name-dialog";
    document.body.append(modal);
  }
  modal.setAttribute("aria-labelledby", "device-name-title");
  modal.innerHTML = `<h2 id="device-name-title">${esc(t("nameDevice"))}</h2><p class="name-browser">${esc(device.name)}</p><form id="device-name-form"><div class="field"><div class="floating"><input id="device-name" type="text" placeholder=" " maxlength="64" value="${esc(device.label || "")}" autocomplete="off" aria-describedby="device-name-help"><label for="device-name">${esc(t("deviceName"))}</label></div><small id="device-name-help">${esc(t("deviceNameHelp"))}</small></div><p class="status-message error" id="device-name-error" role="alert" hidden></p><div class="inline-actions"><button type="button" class="button" id="cancel-device-name">${esc(t("cancel"))}</button><button type="submit" class="button primary">${esc(t("saveName"))}</button></div></form>`;
  let saving = false;
  modal.oncancel = (event) => {
    if (saving) event.preventDefault();
  };
  modal.querySelector("#cancel-device-name").addEventListener("click", () => {
    if (!saving) modal.close();
  });
  modal.querySelector("form").addEventListener("submit", async (event) => {
    event.preventDefault();
    if (saving) return;
    saving = true;
    const buttons = [...modal.querySelectorAll("button")];
    buttons.forEach((b) => (b.disabled = true));
    try {
      await api("devices/rename", {
        id: device.id,
        name: modal.querySelector("input").value,
      });
      modal.close();
      await loadDevices();
      notice(t("deviceNameSaved"));
    } catch (error) {
      const message = modal.querySelector("#device-name-error");
      message.hidden = false;
      message.textContent = t(error.message);
    } finally {
      saving = false;
      buttons.forEach((b) => (b.disabled = false));
    }
  });
  modal.showModal();
  modal.querySelector("input").focus();
}
main.addEventListener("click", (event) => {
  const button = event.target.closest("[data-show]");
  if (!button) return;
  const input = document.getElementById(button.dataset.show),
    show = input.type === "password";
  input.type = show ? "text" : "password";
  button.innerHTML = show ? eyeOffIcon : eyeIcon;
  button.setAttribute("aria-pressed", String(show));
  button.setAttribute(
    "aria-label",
    t(show ? "hide" : "show") + " " + input.labels[0].textContent,
  );
});
function renderBackups() {
  const section = document.createElement("section");
  section.className = "account-section";
  section.id = "backup-section";
  section.innerHTML = `<h2>${esc(t("backups"))}</h2><p>${esc(t("backupHelp"))}</p><p class="backup-private">${shieldIcon}<span>${esc(t("backupPrivate"))}</span></p>${!state.backupAvailable ? `<p>${esc(t("BACKUP_UNAVAILABLE"))}</p>` : !state.fresh ? `<p>${esc(t("backupReauth"))}</p><a class="button" href="${root}/login?reauth=1">${esc(t("reauthButton"))}</a>` : `<div class="inline-actions"><button class="button primary" type="button" id="download-backup">↓ ${esc(t("backupDownload"))}</button><button class="button" type="button" id="restore-backup">↶ ${esc(t("backupRestore"))}</button></div>`}`;
  main.querySelector("#logout").before(section);
  if (state.restoreResult) {
    const result = document.createElement("p");
    result.className =
      "status-message" + (state.restoreResult.success ? "" : " error");
    result.setAttribute("role", "status");
    result.textContent =
      language === "de"
        ? state.restoreResult.success
          ? "Backup erfolgreich wiederhergestellt. Alle DNS-Einträge sind pausiert."
          : "Die Wiederherstellung ist fehlgeschlagen. Prüfe den bisherigen Stand und versuche es erneut."
        : state.restoreResult.success
          ? "Backup restored successfully. All DNS records are paused."
          : "Restore failed. Check the previous state and try again.";
    section.append(result);
  }
  section
    .querySelector("#download-backup")
    ?.addEventListener("click", downloadBackup);
  section
    .querySelector("#restore-backup")
    ?.addEventListener("click", openBackupRestore);
}
function renderFooter() {
  messages.de.footer = "Dein Netzwerk bleibt verbunden.";
  messages.en.footer = "Your network stays connected.";
  const de = language === "de";
  const footer = document.querySelector(".auth-footer");
  footer.innerHTML = `<span class="footer-brand">↗ ddns updater</span><div class="footer-promo"><a href="https://ipv64.net/dyndns.php" target="_blank" rel="noopener noreferrer sponsored" referrerpolicy="no-referrer" aria-label="${de ? "Werbung: IPv64 entdecken" : "Advertisement: discover IPv64"}"><small>${de ? "WERBUNG" : "ADVERTISEMENT"}</small><img class="footer-logo-light" src="${root}/auth-assets/ipv64-wordmark.svg" width="44" height="15" alt="IPv64"><img class="footer-logo-dark" src="${root}/auth-assets/ipv64-wordmark-white.svg" width="44" height="15" alt="IPv64"> ↗</a><button class="footer-restore-promo" type="button" aria-label="${de ? "Werbekachel wieder anzeigen" : "Show promotional card again"}" title="${de ? "Werbekachel wieder anzeigen" : "Show promotional card again"}" hidden>⊞</button></div><span id="footer-note">${de ? "Dein Netzwerk bleibt verbunden." : "Your network stays connected."} <span class="footer-online">●</span></span><button type="button" class="footer-about">${de ? "Über dieses Projekt" : "About this project"}</button>`;
  let dismissed = false;
  try {
    dismissed = localStorage.getItem("ddns-ipv64-promo-dismissed") === "true";
  } catch {}
  const restore = footer.querySelector(".footer-restore-promo");
  restore.hidden = !dismissed;
  restore.onclick = () => {
    try {
      localStorage.setItem("ddns-ipv64-promo-dismissed", "false");
    } catch {}
    restore.hidden = true;
  };
  footer.querySelector(".footer-about").onclick = () => {
    let modal = document.querySelector("#about-project-dialog");
    if (!modal) {
      modal = document.createElement("dialog");
      modal.id = "about-project-dialog";
      modal.className = "name-dialog project-dialog";
      document.body.append(modal);
    }
    modal.setAttribute("aria-labelledby", "about-project-title");
    modal.innerHTML = `<div class="project-heading"><h2 id="about-project-title">${de ? "Über dieses Projekt" : "About this project"}</h2><button class="icon-button" type="button" aria-label="${de ? "Schließen" : "Close"}">×</button></div><p>${de ? "Weiterentwickelt von" : "Developed further by"} <strong>reactor2k3</strong></p><p>${de ? "Mit Unterstützung von ChatGPT/Codex." : "With support from ChatGPT/Codex."}</p><p>${de ? "Basierend auf" : "Based on"} <a href="https://github.com/qdm12/ddns-updater" target="_blank" rel="noopener noreferrer">ddns-updater</a> ${de ? "von Quentin McGaw (@qdm12) und den Mitwirkenden." : "by Quentin McGaw (@qdm12) and contributors."}</p><p>${de ? "Eigenständige Weiterentwicklung; keine offizielle Veröffentlichung des Originalprojekts." : "An independent extension; not an official release of the original project."}</p><a href="https://github.com/qdm12/ddns-updater/blob/master/LICENSE" target="_blank" rel="noopener noreferrer">${de ? "Originalprojekt & MIT-Lizenz" : "Original project & MIT license"}</a><small>Copyright © 2019 Quentin McGaw · ${de ? "Änderungen" : "Changes"} © 2026 reactor2k3</small>`;
    modal.querySelector("button").onclick = () => modal.close();
    modal.showModal();
  };
}
async function backupRequest(route, body, contentType) {
  let response;
  try {
    response = await fetch(root + "/api/auth/backup/" + route, {
      method: "POST",
      credentials: "same-origin",
      cache: "no-store",
      headers: { "Content-Type": contentType },
      body,
    });
  } catch {
    throw new Error("offline");
  }
  if (!response.ok) {
    let result = {};
    try {
      result = await response.json();
    } catch {}
    if (result.error === "REAUTH_REQUIRED") {
      state.fresh = false;
      document.querySelector("#backup-dialog")?.close();
      renderAccount();
      notice(t("backupReauth"), true);
    }
    throw new Error(result.error || "UNKNOWN");
  }
  return response;
}
async function downloadBackup() {
  if (busy) return;
  busy = true;
  const button = document.querySelector("#download-backup");
  button.disabled = true;
  try {
    const response = await backupRequest("download", "{}", "application/json");
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download =
      response.headers
        .get("Content-Disposition")
        ?.match(/filename="([^"]+)"/)?.[1] || "ddns-updater-backup.zip";
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 60000);
    notice(t("backupDownloaded"));
  } catch (error) {
    notice(t(error.message), true);
  } finally {
    busy = false;
    button.disabled = false;
  }
}
function openBackupRestore() {
  let modal = document.querySelector("#backup-dialog");
  if (!modal) {
    modal = document.createElement("dialog");
    modal.id = "backup-dialog";
    modal.className = "name-dialog";
    document.body.append(modal);
  }
  modal.setAttribute("aria-labelledby", "backup-title");
  modal.innerHTML = `<h2 id="backup-title">${esc(t("backupConfirm"))}</h2><p class="name-browser">${esc(t("backupConsequences"))}</p><form><div class="field"><label class="backup-file-label" for="backup-file">${esc(t("backupFile"))}</label><input id="backup-file" type="file" accept=".zip,application/zip" required aria-describedby="backup-file-help"><small id="backup-file-help">${esc(t("backupFileHelp"))}</small></div><p class="status-message error" id="backup-error" role="alert" hidden></p><div class="inline-actions"><button class="button" type="button" id="cancel-backup">${esc(t("cancel"))}</button><button class="button primary" type="submit">${esc(t("backupRestore"))}</button></div></form>`;
  let restoring = false;
  modal.oncancel = (event) => {
    if (restoring) event.preventDefault();
  };
  modal.querySelector("#cancel-backup").onclick = () => {
    if (!restoring) modal.close();
  };
  modal.querySelector("form").onsubmit = async (event) => {
    event.preventDefault();
    if (restoring || busy) return;
    const file = modal.querySelector("input").files[0];
    if (!file) return;
    const errorBox = modal.querySelector("#backup-error");
    if (file.size > 8 * 1024 * 1024) {
      errorBox.hidden = false;
      errorBox.textContent = t("BACKUP_TOO_LARGE");
      return;
    }
    restoring = busy = true;
    const buttons = [...modal.querySelectorAll("button")];
    buttons.forEach((button) => (button.disabled = true));
    try {
      await backupRequest("restore", file, "application/zip");
      modal.close();
      main.innerHTML = `<div class="eyebrow">${esc(t("backups"))}</div><h1>${esc(t("busy"))}</h1><p class="intro" role="status">${esc(t("backupReloading"))}</p><a class="button primary" href="${root}/login">${esc(t("backupLogin"))}</a>`;
      waitForRestoredServer();
    } catch (error) {
      errorBox.hidden = false;
      errorBox.textContent = t(error.message);
      restoring = busy = false;
      buttons.forEach((button) => (button.disabled = false));
    }
  };
  modal.showModal();
  modal.querySelector("input").focus();
}
async function waitForRestoredServer() {
  for (let attempt = 0; attempt < 60; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    try {
      const response = await fetch(root + "/api/auth/state", {
        credentials: "same-origin",
        cache: "no-store",
      });
      if (response.ok) {
        const result = await response.json();
        if (!result.setupRequired) {
          location.assign(root + "/login?restored=1");
          return;
        }
      }
    } catch {}
  }
}
function organiseAccount() {
  const sections = [...main.querySelectorAll(".account-section")];
  const items = [
    ["access", language === "de" ? "Zugang" : "Access"],
    ["devices", t("devices")],
    ["backups", language === "de" ? "Backups" : "Backups"],
  ];
  const layout = document.createElement("div");
  layout.className = "account-layout";
  const nav = document.createElement("nav");
  nav.className = "account-nav";
  nav.setAttribute("role", "tablist");
  nav.setAttribute("aria-label", t("account"));
  nav.setAttribute(
    "aria-orientation",
    matchMedia("(max-width:600px)").matches ? "horizontal" : "vertical",
  );
  const panels = document.createElement("div");
  panels.className = "account-panels";
  const activate = (id, focus = false) => {
    items.forEach(([key]) => {
      const active = key === id;
      const button = nav.querySelector("#tab-" + key);
      button.setAttribute("aria-selected", String(active));
      button.tabIndex = active ? 0 : -1;
      document.getElementById("panel-" + key).hidden = !active;
      if (active && focus) button.focus();
    });
    history.replaceState(null, "", "#" + id);
  };
  items.forEach(([id, label], index) => {
    const panel = sections[index];
    panel.id = "panel-" + id;
    panel.setAttribute("role", "tabpanel");
    panel.setAttribute("aria-labelledby", "tab-" + id);
    panel.tabIndex = 0;
    panels.append(panel);
    const button = document.createElement("button");
    button.id = "tab-" + id;
    button.type = "button";
    button.setAttribute("role", "tab");
    button.setAttribute("aria-controls", panel.id);
    button.textContent = label;
    button.onclick = () => activate(id);
    button.onkeydown = (event) => {
      if (
        ![
          "ArrowRight",
          "ArrowLeft",
          "ArrowDown",
          "ArrowUp",
          "Home",
          "End",
        ].includes(event.key)
      )
        return;
      event.preventDefault();
      const step = ["ArrowRight", "ArrowDown"].includes(event.key) ? 1 : -1;
      const next =
        event.key === "Home"
          ? 0
          : event.key === "End"
            ? items.length - 1
            : (index + step + items.length) % items.length;
      activate(items[next][0], true);
    };
    nav.append(button);
  });
  layout.append(nav, panels);
  main.querySelector("#logout").before(layout);
  const requested = location.hash.slice(1);
  activate(items.some(([id]) => id === requested) ? requested : "access");
}
api("state")
  .then((value) => {
    state = value;
    if (
      value.authenticated &&
      location.pathname === root + "/login" &&
      new URLSearchParams(location.search).get("reauth") !== "1"
    ) {
      location.replace(root + "/");
      return;
    }
    render();
  })
  .catch((error) => {
    main.innerHTML = '<div id="message"></div>';
    notice(t(error.message), true);
  });
