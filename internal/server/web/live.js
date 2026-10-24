const $ = (s) => document.querySelector(s),
  $$ = (s) => [...document.querySelectorAll(s)];
const icon = (n) => `<svg aria-hidden="true"><use href="#i-${n}"/></svg>`;
const escapeHTML = (v) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
let entries = [],
  events = [],
  runtime = {},
  readOnly = false,
  filter = "all",
  editing = null,
  ipVersion = "IPv4",
  activeView = "entries",
  toastTimer,
  formBusy = false,
  loading = false,
  lastSnapshot = "";
let promoDismissed = false;
try {
  promoDismissed =
    localStorage.getItem("ddns-ipv64-promo-dismissed") === "true";
} catch {}
let renderedCardsKey = "";
const STATUS = {
  ready: "Aktuell",
  paused: "Pausiert",
  waiting: "Ausstehend",
  failure: "Fehler",
  updating: "Wird aktualisiert …",
};
const EVENT = {
  DNS_UPDATED: "DNS-Eintrag erfolgreich aktualisiert",
  DNS_FAILED:
    "DNS-Aktualisierung fehlgeschlagen. Prüfe die Zugangsdaten und die Anleitung deines Providers.",
  ENTRY_CREATED: "Eintrag angelegt",
  ENTRY_CHANGED: "Eintrag bearbeitet",
  ENTRY_PAUSED: "Eintrag pausiert",
  ENTRY_DELETED: "Eintrag gelöscht",
};
const ERRORS = {
  INVALID_INPUT: "Bitte prüfe deine Eingaben.",
  INVALID_PROVIDER_CONFIG:
    "Bitte prüfe die Angaben für deinen Provider. Die Anleitung unter ⓘ hilft dir dabei.",
  SAVE_FAILED:
    "Speichern fehlgeschlagen. Prüfe die Angaben, die gewählte IP-Version und die Schreibrechte im Datenverzeichnis.",
  DUPLICATE_DOMAIN:
    "Diese Domain ist bereits vorhanden. Öffne den bestehenden Eintrag zum Bearbeiten.",
  REVISION_CONFLICT:
    "Der Eintrag wurde inzwischen geändert. Lade die Liste neu und öffne ihn erneut.",
  CONFIG_READ_ONLY:
    "Die Konfiguration wird über CONFIG vorgegeben. Entferne diese Umgebungsvariable, um Einträge hier zu verwalten.",
  UPDATE_BUSY: "Ein Update läuft gerade. Bitte versuche es gleich erneut.",
  ENTRY_NOT_FOUND: "Der Eintrag wurde nicht gefunden. Lade die Liste neu.",
  ENTRY_LIMIT: "Maximal 200 Einträge sind möglich.",
  NETWORK: "Der Server ist nicht erreichbar. Bitte versuche es erneut.",
};
async function api(path, method = "GET", body) {
  let response;
  try {
    response = await fetch(new URL("api/" + path, document.baseURI), {
      method,
      credentials: "same-origin",
      cache: "no-store",
      headers: body === undefined ? {} : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw Error("NETWORK");
  }
  if (response.status === 401) {
    location.assign("./login");
    throw Error("AUTH_REQUIRED");
  }
  let result;
  try {
    result = await response.json();
  } catch {
    throw Error("NETWORK");
  }
  if (!response.ok) throw Error(result.error || "SAVE_FAILED");
  return result;
}
function notify(message, kind = "success") {
  clearTimeout(toastTimer);
  $("#toast-message").textContent = message;
  $("#toast").dataset.kind = kind;
  $("#toast .toast-icon").textContent = kind === "success" ? "✓" : "!";
  $("#toast").hidden = false;
  toastTimer = setTimeout(() => ($("#toast").hidden = true), 6500);
}
function errorText(error) {
  return ERRORS[error.message] || ERRORS.NETWORK;
}
function dateText(value) {
  return value && !value.startsWith("0001")
    ? new Date(value).toLocaleString(document.documentElement.lang, {
        dateStyle: "short",
        timeStyle: "short",
      })
    : uiText("Noch nicht geprüft");
}
function providerName(id) {
  return PROVIDERS.find((p) => p.id === id)?.name || id;
}
function providerMark(name) {
  return `<span class="provider-logo" aria-hidden="true">${escapeHTML(name[0] || "?")}</span>`;
}
function promoMarkup() {
  return `<aside class="record-card ipv64-promo" role="listitem" aria-label="Werbung für IPv64"><div class="ipv64-promo-inner"><header><img class="ipv64-logo-light" src="icons/ipv64-wordmark.svg" alt="IPv64" width="92" height="30"><img class="ipv64-logo-dark" src="icons/ipv64-wordmark-white.svg" alt="IPv64" width="92" height="30"><span class="promo-label">WERBUNG</span><button type="button" class="promo-dismiss" data-dismiss-promo aria-label="Werbekachel dauerhaft ausblenden" title="Werbekachel dauerhaft ausblenden">${icon("close")}</button></header><h3>Dein Zuhause. Deine Domain.</h3><p>DynDNS für IPv4 & IPv6.<br>Kostenlos starten. Verbunden bleiben.</p><a class="ipv64-promo-link" href="https://ipv64.net/dyndns.php" target="_blank" rel="noopener noreferrer sponsored" referrerpolicy="no-referrer"><span>IPv64 entdecken</span>${icon("arrow")}</a></div></aside>`;
}
const copyIcon =
  '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="8" y="8" width="12" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></svg>';
function ipBadges(e) {
  return `<span class="ip-protocols" role="group" aria-label="Verwendete IP-Versionen">${[
    "IPv4",
    "IPv6",
  ]
    .map((version) => {
      const address = version === "IPv4" ? e.ipv4 : e.ipv6,
        active =
          e.ip === "Dual Stack" ||
          e.ip === version ||
          (e.ip === "Automatisch" && !!address),
        label = uiText(
          version === "IPv4"
            ? "IPv4-Adresse kopieren"
            : "IPv6-Adresse kopieren",
        );
      return `<span class="ip-address-row"><span class="ip-protocol ${active ? "is-active" : ""}" aria-label="${version} ${active ? "aktiv" : "inaktiv"}"><span class="protocol-dot" aria-hidden="true"></span>${version}</span><code class="protocol-address ${active ? "" : "not-used"}">${escapeHTML(active ? address || uiText("Noch nicht ermittelt") : uiText("Nicht verwendet"))}</code><button type="button" class="icon-button copy-ip" data-copy-ip="${escapeHTML(active ? address || "" : "")}" aria-label="${escapeHTML(label)}" title="${escapeHTML(label)}" ${active && address ? "" : "disabled"}>${copyIcon}</button></span>`;
    })
    .join("")}</span>`;
}
function render() {
  const active = entries.filter((e) => e.enabled).length,
    paused = entries.length - active;
  $("#total-count").textContent = String(entries.length).padStart(2, "0");
  $("#active-count").textContent = entries.filter(
    (e) => e.status === "ready",
  ).length;
  $("#paused-count").textContent = paused;
  $("#nav-count").textContent = entries.length;
  $("#tab-all").textContent = entries.length;
  $("#tab-active").textContent = active;
  $("#tab-paused").textContent = paused;
  $("#provider-count").textContent =
    `über ${new Set(entries.map((e) => e.providerId)).size} Provider`;
  const query = $("#search").value.trim().toLocaleLowerCase(),
    visible = entries.filter(
      (e) =>
        (filter === "all" || (filter === "active" ? e.enabled : !e.enabled)) &&
        `${e.domain} ${providerName(e.providerId)} ${e.description}`
          .toLocaleLowerCase()
          .includes(query),
    );
  const cardsKey = JSON.stringify([visible, currentLanguage, promoDismissed]);
  if (cardsKey !== renderedCardsKey) {
    $("#records").innerHTML = visible
      .map(
        (e) =>
          `<article class="record-card" role="listitem" data-id="${escapeHTML(e.id)}"><div class="card-heading"><span class="card-domain-icon">${icon("globe")}</span><span class="status-pill ${e.status === "paused" ? "paused" : e.status === "ready" ? "" : "state-" + e.status}"><span class="dot"></span>${escapeHTML(STATUS[e.status] || STATUS.waiting)}</span><button class="row-action" aria-label="${escapeHTML(e.domain)} bearbeiten" title="Eintrag bearbeiten">${icon("arrow")}</button></div><div class="domain-name">${escapeHTML(e.domain)}</div><div class="domain-description">${escapeHTML(e.description)}</div><div class="card-ip">${ipBadges(e)}</div><div class="card-footer"><div class="provider-cell">${providerMark(providerName(e.providerId))}${escapeHTML(providerName(e.providerId))}</div><span class="update-time">${escapeHTML(dateText(e.checkedAt))}</span></div>${e.status === "failure" ? `<p class="record-error">${recordIssueMarkup(e)}</p>` : ""}</article>`,
      )
      .join("");
    $("#records").insertAdjacentHTML(
      "afterbegin",
      promoDismissed ? "" : promoMarkup(),
    );
    renderedCardsKey = cardsKey;
  }
  if ($("#restore-ipv64-promo"))
    $("#restore-ipv64-promo").hidden = !promoDismissed;
  $("#empty").hidden = visible.length > 0;
  $("#results-count").textContent =
    `${visible.length} ${visible.length === 1 ? "Eintrag" : "Einträge"}${visible.length !== entries.length ? " von " + entries.length : ""}`;
  $("#activity-list").innerHTML =
    events
      .map(
        (e) =>
          `<div class="activity-row"><span class="activity-badge">${icon(e.message === "DNS_FAILED" ? "info" : "check")}</span><div><strong>${escapeHTML(e.domain)}</strong><small>${e.version ? `<span>${escapeHTML(e.version)} · </span>` : ""}${escapeHTML(EVENT[e.message] || e.message)}</small></div><time>${escapeHTML(dateText(e.time))}</time></div>`,
      )
      .join("") || "<p>Noch keine Aktivitäten.</p>";
  $("#provider-list").innerHTML =
    [...new Set(entries.map((e) => e.providerId))]
      .map(
        (id) =>
          `<div class="provider-row">${providerMark(providerName(id))}<div><strong>${escapeHTML(providerName(id))}</strong><small>In deinem Workspace konfiguriert</small></div><span>${entries.filter((e) => e.providerId === id).length} Einträge</span></div>`,
      )
      .join("") || "<p>Noch keine Provider.</p>";
  $("#add-entry").disabled = readOnly;
  $("#connection-label").textContent = "DDNS-Dienst erreichbar";
  $("#connection-detail").textContent = readOnly
    ? ERRORS.CONFIG_READ_ONLY
    : runtime.running
      ? "DNS-Prüfung läuft …"
      : active
        ? "Automatische DNS-Prüfung aktiv"
        : entries.length
          ? "Alle Einträge pausiert"
          : "Noch keine Einträge";
  $("#connection-status").dataset.state = "online";
  $("#connection-dot").className = "dot";
  $("#connection-time").textContent =
    uiText("Geprüft um ") +
    new Date().toLocaleTimeString(document.documentElement.lang);
  $(".overview-cycle strong").textContent =
    runtime.periodSeconds === 300
      ? "Alle 5 Minuten"
      : `${uiText("Intervall")}: ${Math.round((runtime.periodSeconds || 300) / 60)} min`;
  $("#enabled").closest(".toggle-label").querySelector("small").textContent =
    `${uiText("Intervall")}: ${Math.round((runtime.periodSeconds || 300) / 60)} min`;
  $(".overview-cycle small").textContent = runtime.running
    ? "DNS-Prüfung läuft …"
    : dateText(runtime.lastCheck);
  renderPublicIPs();
  $(".table-footer>span:last-child").textContent = active
    ? "Automatische Updates aktiviert"
    : entries.length
      ? "Alle Einträge pausiert"
      : "Noch keine Einträge";
  manualUpdateButton.disabled = !!runtime.running || active === 0;
  manualUpdateButton.classList.toggle("spinning", !!runtime.running);
  manualUpdateButton.setAttribute("aria-busy", String(!!runtime.running));
}
async function loadEntries(silent = false) {
  if (loading) {
    if (silent) return;
    await loading;
    return loadEntries(silent);
  }
  loading = (async () => {
    try {
      const result = await api("entries"),
        signature = JSON.stringify(result);
      if (signature !== lastSnapshot) {
        entries = result.entries;
        events = result.events;
        runtime = result.status;
        readOnly = result.readOnly;
        lastSnapshot = signature;
        render();
      } else {
        $("#connection-time").textContent =
          uiText("Geprüft um ") +
          new Date().toLocaleTimeString(document.documentElement.lang);
        $("#connection-status").dataset.state = "online";
        $("#connection-label").textContent = "DDNS-Dienst erreichbar";
      }
    } catch (error) {
      $("#connection-label").textContent = "Keine Verbindung";
      $("#connection-status").dataset.state = "offline";
      if (!silent) notify(errorText(error), "warning");
    }
  })();
  try {
    await loading;
  } finally {
    loading = false;
  }
}
function setIp(version) {
  ipVersion = version;
  $$("[data-ip]").forEach((b) => {
    b.classList.toggle("selected", b.dataset.ip === version);
    b.setAttribute("aria-pressed", String(b.dataset.ip === version));
  });
  $("#ip-help").textContent = IP_HELP[version] || "";
  $("#section-ip-help").textContent = IP_HELP[version] || "";
}
function updateCredential() {
  renderProviderFields();
}
function openForm(id = null) {
  if (formBusy) return;
  editing = id;
  const e = entries.find((e) => e.id === id);
  $("#entry-form").reset();
  closeProviderPicker();
  $("#entry-error").hidden = true;
  $("#form-title").innerHTML =
    `${e ? "Eintrag bearbeiten" : "Neuer Eintrag"}<span class="heading-dot">.</span>`;
  $("#save-label").textContent = e ? "Änderungen speichern" : "Eintrag anlegen";
  $("#delete-entry").hidden = !e;
  $("#delete-entry").disabled = readOnly;
  $("#entry-form button[type=submit]").disabled = readOnly;
  $("#domain").value = e?.domain || "";
  $("#description").value = e?.description || "";
  $("#provider").value = providerName(e?.providerId || "cloudflare");
  $("#enabled").checked = e?.enabled ?? true;
  setIp(e?.ip || "IPv4");
  updateCredential();
  if (e)
    for (const input of $$("[data-config]")) {
      const key = input.dataset.config;
      if (key in e.config) {
        const value = e.config[key];
        if (input.type === "checkbox") input.checked = !!value;
        else
          input.value =
            typeof value === "object"
              ? JSON.stringify(value, null, 2)
              : String(value);
      }
      if (e.configuredSecrets.includes(key)) {
        input.required = false;
        input.dataset.saved = "true";
        input.closest(".field").querySelector(".field-example").textContent =
          "Zugangsdaten gespeichert. Leer lassen, um sie beizubehalten.";
      }
    }
  $("#entry-dialog").showModal();
}
function closeForm() {
  if (formBusy) return;
  clearProviderSecrets();
  $("#entry-dialog").close();
}
$("#records").addEventListener("click", async (event) => {
  const button = event.target.closest("[data-copy-ip]");
  if (!button) return;
  event.stopImmediatePropagation();
  const address = button.dataset.copyIp;
  if (!address || button.disabled) return;
  try {
    await navigator.clipboard.writeText(address);
    button.innerHTML = icon("check");
    notify(uiText("IP-Adresse kopiert."));
    setTimeout(() => {
      if (button.isConnected) button.innerHTML = copyIcon;
    }, 1800);
  } catch {
    notify(
      uiText(
        "Kopieren nicht möglich. Bitte markiere die Adresse und kopiere sie manuell.",
      ),
      "warning",
    );
  }
});
$("#add-entry").addEventListener("click", () => openForm());
$("#close-dialog").addEventListener("click", closeForm);
$("#cancel-dialog").addEventListener("click", closeForm);
$("#entry-dialog").addEventListener("cancel", (e) => {
  if (formBusy) e.preventDefault();
});
$("#records").addEventListener("click", (event) => {
  const card = event.target.closest("[data-id]");
  if (card) openForm(card.dataset.id);
});
$("#search").addEventListener("input", render);
$$("[data-filter]").forEach((b) =>
  b.addEventListener("click", () => {
    filter = b.dataset.filter;
    $$("[data-filter]").forEach((t) => {
      t.classList.toggle("active", t === b);
      t.setAttribute("aria-pressed", String(t === b));
    });
    render();
  }),
);
$$("[data-ip]").forEach((b) =>
  b.addEventListener("click", () => setIp(b.dataset.ip)),
);
$("#provider").addEventListener("change", updateCredential);
$("#domain").addEventListener("input", () =>
  $("#domain").setCustomValidity(""),
);
function formError(error) {
  $("#entry-error").textContent = errorText(error);
  $("#entry-error").hidden = false;
  $("#entry-error").scrollIntoView({ block: "nearest" });
}
$("#entry-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (formBusy) return;
  const p = currentProvider(),
    config = {};
  try {
    for (const f of p.fields) {
      const input = document.getElementById("provider-" + f.key);
      if (input.type === "checkbox") config[f.key] = input.checked;
      else if (input.value !== "")
        config[f.key] =
          f.type === "number"
            ? Number(input.value)
            : f.key === "credentials"
              ? JSON.parse(input.value)
              : input.value;
    }
  } catch {
    formError(Error("INVALID_PROVIDER_CONFIG"));
    return;
  }
  const previous = entries.find((e) => e.id === editing),
    entry = {
      domain: $("#domain").value.trim().toLowerCase(),
      description: $("#description").value.trim(),
      providerId: p.id,
      config,
      ip: ipVersion,
      enabled: $("#enabled").checked,
      revision: previous?.revision || 0,
    };
  formBusy = true;
  const button = $("#entry-form button[type=submit]");
  button.disabled = true;
  $("#entry-error").hidden = true;
  try {
    await api(
      "entries" + (editing ? "/" + editing : ""),
      editing ? "PUT" : "POST",
      entry,
    );
    formBusy = false;
    closeForm();
    await loadEntries();
    notify(previous ? "Änderungen gespeichert." : "Eintrag angelegt.");
  } catch (error) {
    formError(error);
  } finally {
    formBusy = false;
    button.disabled = readOnly;
  }
});
const deleteConfirmation = document.createElement("dialog");
deleteConfirmation.id = "delete-confirmation";
deleteConfirmation.className = "confirm-dialog";
deleteConfirmation.setAttribute("aria-labelledby", "delete-title");
deleteConfirmation.setAttribute("aria-describedby", "delete-explanation");
deleteConfirmation.innerHTML = `<h2 id="delete-title">Eintrag wirklich löschen?</h2><p class="delete-domain"><strong id="delete-domain"></strong></p><p id="delete-explanation">Nur aus diesem Updater entfernen. Der DNS-Eintrag beim Provider bleibt bestehen.</p><p class="entry-error" id="delete-error" role="alert" hidden></p><div class="confirm-actions"><button type="button" class="button secondary" id="cancel-delete">Abbrechen</button><button type="button" class="button danger-button" id="confirm-delete-entry">Eintrag löschen</button></div>`;
document.body.append(deleteConfirmation);
$("#delete-entry").addEventListener("click", () => {
  const e = entries.find((e) => e.id === editing);
  if (!e || formBusy) return;
  $("#delete-domain").textContent = e.domain;
  $("#delete-error").hidden = true;
  deleteConfirmation.showModal();
  $("#cancel-delete").focus();
});
$("#cancel-delete").addEventListener("click", () => {
  if (!formBusy) deleteConfirmation.close();
});
deleteConfirmation.addEventListener("cancel", (event) => {
  if (formBusy) event.preventDefault();
});
$("#confirm-delete-entry").addEventListener("click", async () => {
  const e = entries.find((e) => e.id === editing);
  if (!e || formBusy) return;
  formBusy = true;
  $("#confirm-delete-entry").disabled = true;
  $("#cancel-delete").disabled = true;
  try {
    await api("entries/" + e.id, "DELETE", { revision: e.revision });
    formBusy = false;
    deleteConfirmation.close();
    closeForm();
    await loadEntries();
    notify("Eintrag gelöscht.");
  } catch (error) {
    $("#delete-error").textContent = uiText(errorText(error));
    $("#delete-error").hidden = false;
  } finally {
    formBusy = false;
    $("#confirm-delete-entry").disabled = false;
    $("#cancel-delete").disabled = false;
  }
});
$("#refresh").title = "Liste neu laden";
$("#refresh").setAttribute("aria-label", "Liste neu laden");
$("#refresh").addEventListener("click", () => loadEntries());
$("#check-connection").addEventListener("click", () => loadEntries());
function updateThemeButton() {
  const dark = document.documentElement.dataset.theme === "dark";
  $("#theme-toggle").innerHTML = icon(dark ? "sun" : "moon");
  $("#theme-toggle").setAttribute(
    "aria-label",
    dark ? "Hellmodus aktivieren" : "Darkmode aktivieren",
  );
}
$("#theme-toggle").addEventListener("click", () => {
  const theme =
    document.documentElement.dataset.theme === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem("ddns-theme", theme);
  } catch {}
  updateThemeButton();
});
function closeNav() {
  $("#sidebar").classList.remove("open");
  $("#nav-backdrop").hidden = true;
  $("#menu-button").setAttribute("aria-expanded", "false");
}
$("#menu-button").addEventListener("click", () => {
  const open = $("#sidebar").classList.toggle("open");
  $("#nav-backdrop").hidden = !open;
  $("#menu-button").setAttribute("aria-expanded", String(open));
});
$("#nav-backdrop").addEventListener("click", closeNav);
$$("[data-view]").forEach((b) =>
  b.addEventListener("click", () => {
    activeView = b.dataset.view;
    $$("[data-view]").forEach((n) => {
      n.classList.toggle("active", n === b);
      n.setAttribute("aria-current", n === b ? "page" : "false");
    });
    ["entries", "activity", "providers"].forEach(
      (v) => ($(`#${v}-view`).hidden = v !== activeView),
    );
    const text = {
      entries: [
        "DNS-Einträge",
        "Alle Domains im Blick. Jede IP auf dem neuesten Stand.",
      ],
      activity: [
        "Aktivität",
        "Alle Änderungen und Updates im zeitlichen Verlauf.",
      ],
      providers: ["Provider", "Deine DNS-Anbieter an einem Ort."],
    }[activeView];
    $("#page-title").innerHTML = `${text[0]}<span class="heading-dot">.</span>`;
    $("#breadcrumb-title").textContent = text[0];
    $("#page-subtitle").textContent = text[1];
    closeNav();
  }),
);
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") closeNav();
  if (
    e.key === "/" &&
    !["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement.tagName) &&
    !$("#entry-dialog").open
  ) {
    e.preventDefault();
    $("#search").focus();
  }
});
const accountLink = document.createElement("a");
accountLink.className = "nav-item";
accountLink.href = "./account";
accountLink.innerHTML = icon("check") + "<span>Konto & Sicherheit</span>";
$(".sidebar-bottom").prepend(accountLink);
const manualUpdateButton = document.createElement("button");
manualUpdateButton.type = "button";
manualUpdateButton.id = "update-now";
manualUpdateButton.className = "button secondary manual-update";
manualUpdateButton.innerHTML =
  icon("refresh") + '<span class="sr-only">Jetzt aktualisieren</span>';
manualUpdateButton.setAttribute("aria-label", "Jetzt aktualisieren");
manualUpdateButton.title = "Jetzt aktualisieren";
$(".overview-cycle").append(manualUpdateButton);
manualUpdateButton.addEventListener("click", async () => {
  manualUpdateButton.disabled = true;
  try {
    await api("update", "POST", {});
    notify("DNS-Prüfung gestartet.");
    await loadEntries();
  } catch (error) {
    notify(errorText(error), "warning");
  } finally {
    manualUpdateButton.disabled = !!runtime.running;
  }
});
$(".preview-label").textContent = "DDNS UPDATER";
$$("[data-view]").forEach((n) =>
  n.setAttribute(
    "aria-current",
    n.classList.contains("active") ? "page" : "false",
  ),
);
$$("[data-filter]").forEach((n) =>
  n.setAttribute("aria-pressed", String(n.classList.contains("active"))),
);
const installButton = document.createElement("button");
installButton.className = "button secondary install-button";
installButton.textContent = "App installieren";
installButton.hidden = true;
$("#theme-toggle").before(installButton);
let installPrompt;
window.addEventListener("beforeinstallprompt", (e) => {
  e.preventDefault();
  installPrompt = e;
  installButton.hidden = matchMedia("(display-mode: standalone)").matches;
});
installButton.addEventListener("click", async () => {
  if (!installPrompt) return;
  const p = installPrompt;
  installPrompt = null;
  installButton.hidden = true;
  await p.prompt();
});
if ("serviceWorker" in navigator && window.isSecureContext)
  navigator.serviceWorker.register("./sw.js").catch(() => {});
function setPromoDismissed(value) {
  promoDismissed = value;
  try {
    localStorage.setItem("ddns-ipv64-promo-dismissed", String(value));
  } catch {}
  render();
}
$("#records").addEventListener("click", (event) => {
  if (event.target.closest("[data-dismiss-promo]")) setPromoDismissed(true);
});
window.addEventListener("storage", (event) => {
  if (event.key === "ddns-ipv64-promo-dismissed") {
    promoDismissed = event.newValue === "true";
    render();
  }
});
const footerPromo = document.createElement("div");
footerPromo.className = "footer-ipv64";
footerPromo.innerHTML = `<a href="https://ipv64.net/dyndns.php" target="_blank" rel="noopener noreferrer sponsored" referrerpolicy="no-referrer" aria-label="Werbung: IPv64 entdecken"><span>WERBUNG</span><img class="ipv64-logo-light" src="icons/ipv64-wordmark.svg" width="44" height="15" alt="IPv64"><img class="ipv64-logo-dark" src="icons/ipv64-wordmark-white.svg" width="44" height="15" alt="IPv64">${icon("arrow")}</a><button type="button" id="restore-ipv64-promo" class="restore-promo" aria-label="Werbekachel wieder anzeigen" title="Werbekachel wieder anzeigen" hidden>${icon("grid")}</button>`;
$(".main-footer").insertBefore(footerPromo, $(".main-footer").lastElementChild);
$("#restore-ipv64-promo").addEventListener("click", () =>
  setPromoDismissed(false),
);
setIp("IPv4");
updateThemeButton();
loadEntries();
setInterval(() => {
  if (!document.hidden) loadEntries(true);
}, 3000);
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) loadEntries(true);
});
$("#language-select").addEventListener("change", render);

function renderPublicIPs() {
  const checking = !!runtime.ipDiscoveryRunning,
    checked = runtime.ipCheckedAt && !runtime.ipCheckedAt.startsWith("0001");
  $$(".connection-strip .ip-detail").forEach((item, i) => {
    const value = i === 0 ? runtime.ipv4 : runtime.ipv6;
    item.querySelector("code").textContent =
      value ||
      uiText(checking || !checked ? "Wird ermittelt …" : "Nicht verfügbar");
    item.querySelector(".ip-tag").textContent = checking
      ? "PRÜFUNG"
      : value
        ? "ERKANNT"
        : checked
          ? "NICHT ERKANNT"
          : "AUSSTEHEND";
    item.dataset.state = checking
      ? "checking"
      : value
        ? "available"
        : checked
          ? "unavailable"
          : "unknown";
    item.title = value
      ? ""
      : uiText(
          "Über die Netzwerkverbindung des Servers wurde keine öffentliche Adresse dieser IP-Version ermittelt.",
        );
  });
  $("#public-ip-checked").textContent = checking
    ? "IP-Erkennung läuft …"
    : checked
      ? uiText("Ermittelt:") +
        " " +
        new Date(runtime.ipCheckedAt).toLocaleString(
          document.documentElement.lang,
          { dateStyle: "short", timeStyle: "medium" },
        )
      : "Erkennung beim Start";
  $("#public-ip-schedule").textContent =
    runtime.periodSeconds === 300
      ? "Beim Start und alle 5 Minuten"
      : `${uiText("Automatische Erkennung")} · ${Math.round((runtime.periodSeconds || 300) / 60)} min`;
  $("#refresh-public-ips").disabled = checking;
  $("#refresh-public-ips").classList.toggle("spinning", checking);
  $("#refresh-public-ips").setAttribute("aria-busy", String(checking));
}
$("#refresh-public-ips").addEventListener("click", async () => {
  const button = $("#refresh-public-ips");
  button.disabled = true;
  try {
    await api("public-ips/refresh", "POST", {});
    notify("IP-Erkennung gestartet. Es werden keine DNS-Einträge geändert.");
    await loadEntries();
  } catch (error) {
    notify(errorText(error), "warning");
    button.disabled = !!runtime.ipDiscoveryRunning;
  }
});
ERRORS.IP_REFRESH_BUSY = "Die IP-Erkennung läuft bereits. Bitte warte kurz.";
ERRORS.IP_REFRESH_LIMITED =
  "Bitte warte 30 Sekunden zwischen manuellen IP-Prüfungen.";

function recordIssueMarkup(entry) {
  const fallback =
    "DNS-Aktualisierung fehlgeschlagen. Prüfe die Zugangsdaten und die Anleitung deines Providers.";
  if (!entry.issues?.length) return escapeHTML(uiText(fallback));
  return entry.issues
    .map(
      (issue) =>
        escapeHTML(issue.version) +
        ": " +
        escapeHTML(
          uiText(
            issue.code === "PUBLIC_IP_UNAVAILABLE"
              ? "Öffentliche IP-Adresse nicht erreichbar. Prüfe, ob dein Anschluss diese IP-Version unterstützt."
              : fallback,
          ),
        ),
    )
    .join("<br>");
}

const aboutButton = document.createElement("button");
aboutButton.type = "button";
aboutButton.className = "about-link";
aboutButton.textContent = "Über dieses Projekt";
$(".main-footer").append(aboutButton);
const aboutDialog = document.createElement("dialog");
aboutDialog.className = "about-dialog";
aboutDialog.setAttribute("aria-labelledby", "about-title");
aboutDialog.innerHTML = `<header><h2 id="about-title">Über dieses Projekt</h2><button type="button" class="icon-button" aria-label="Schließen">${icon("close")}</button></header><p><span>Weiterentwickelt von</span> <strong>reactor2k3</strong></p><p>Mit Unterstützung von ChatGPT/Codex.</p><p><span>Basierend auf</span> <a href="https://github.com/qdm12/ddns-updater" target="_blank" rel="noopener noreferrer">ddns-updater</a> <span>von Quentin McGaw (@qdm12) und den Mitwirkenden.</span></p><p>Eigenständige Weiterentwicklung; keine offizielle Veröffentlichung des Originalprojekts.</p><a href="https://github.com/qdm12/ddns-updater/blob/master/LICENSE" target="_blank" rel="noopener noreferrer">Originalprojekt & MIT-Lizenz</a><small>Copyright © 2019 Quentin McGaw · Änderungen © 2026 reactor2k3</small>`;
document.body.append(aboutDialog);
aboutButton.addEventListener("click", () => aboutDialog.showModal());
aboutDialog
  .querySelector("button")
  .addEventListener("click", () => aboutDialog.close());
