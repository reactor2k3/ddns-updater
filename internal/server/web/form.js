// Provider examples come from the repository documentation, with beginner-friendly field guidance.
const safe = (value) =>
  String(value).replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );

document
  .querySelector("#entry-dialog")
  .setAttribute("aria-labelledby", "form-title");
document
  .querySelector("#entry-dialog")
  .setAttribute("aria-describedby", "form-description");
document.querySelector("#provider-trigger").setAttribute("autofocus", "");

function attachInfoHelp(root) {
  root.querySelectorAll(".field-help").forEach((help) => {
    help.hidden = true;
  });
}
attachInfoHelp(document.querySelector(".drawer-content"));
const IP_HELP = {
  IPv4: "IPv4 ist für die meisten Heimanschlüsse ein guter Start. Beispiel: 203.0.113.42.",
  IPv6: "Wähle IPv6, wenn dein Anschluss und dein DNS-Provider IPv6 unterstützen. Beispiel: 2001:db8::42.",
  "Dual Stack":
    "IPv4 und IPv6 werden getrennt aktualisiert. Dein Anschluss und dein Anbieter müssen beide IP-Versionen unterstützen.",
  Automatisch:
    "Der Updater wählt je nach erkannter öffentlicher IP entweder IPv4 oder IPv6. Es werden nicht automatisch beide Eintragstypen aktualisiert.",
};

// Native popovers support touch, keyboard, Escape and outside-click dismissal.
// Position them in viewport space so neither modal scrolling nor narrow screens clip them.
document.querySelector("#entry-dialog").addEventListener(
  "toggle",
  (event) => {
    const popup = event.target;
    if (!popup.matches(".info-popover")) return;
    const trigger = [...document.querySelectorAll("[popovertarget]")].find(
      (button) => button.getAttribute("popovertarget") === popup.id,
    );
    if (!trigger) return;
    const open = event.newState === "open";
    trigger.setAttribute("aria-expanded", String(open));
    if (!open) return;
    const anchor = trigger.getBoundingClientRect();
    const size = popup.getBoundingClientRect();
    const viewport = window.visualViewport;
    const leftEdge = viewport?.offsetLeft || 0;
    const topEdge = viewport?.offsetTop || 0;
    const width = viewport?.width || innerWidth;
    const height = viewport?.height || innerHeight;
    const left = Math.max(
      leftEdge + 12,
      Math.min(anchor.left, leftEdge + width - size.width - 12),
    );
    const below = anchor.bottom + 8;
    const top =
      below + size.height <= topEdge + height - 12
        ? below
        : Math.max(topEdge + 12, anchor.top - size.height - 8);
    popup.style.left = `${left}px`;
    popup.style.top = `${top}px`;
  },
  true,
);
function closeInfoHelp() {
  document
    .querySelectorAll(".info-popover:popover-open")
    .forEach((popup) => popup.hidePopover());
}
document
  .querySelector(".drawer-content")
  .addEventListener("scroll", closeInfoHelp, { passive: true });
window.addEventListener("resize", closeInfoHelp);
document
  .querySelector("#entry-dialog")
  .addEventListener("close", closeInfoHelp);

function currentProvider() {
  return (
    PROVIDERS.find(
      (p) => p.name === document.querySelector("#provider").value,
    ) || PROVIDERS.find((p) => p.id === "cloudflare")
  );
}
function fieldMarkup(field) {
  const id = `provider-${field.key}`,
    required = field.required ? " required" : "",
    type = field.type;
  if (type === "checkbox")
    return `<div class="field checkbox-field"><label for="${id}"><input type="checkbox" id="${id}" data-config="${safe(field.key)}" aria-describedby="${id}-help ${id}-example"> ${safe(field.label)}</label><p id="${id}-help" class="field-help">${safe(field.help)}</p><p id="${id}-example" class="field-example">Beispiel: ${field.example ? "Aktiviert" : "Deaktiviert"}</p></div>`;
  const label = `<label for="${id}">${safe(field.label)}${field.required ? ' <span aria-hidden="true">*</span>' : ' <span class="optional-label">Optional</span>'}</label>`;
  const shared = `id="${id}" data-config="${safe(field.key)}" placeholder=" "${required} aria-describedby="${id}-help ${id}-example" autocomplete="off" spellcheck="false"`;
  const input =
    type === "textarea"
      ? `<textarea ${shared} rows="5"></textarea>`
      : `<input type="${type}" ${shared}${type === "number" ? ' min="1" step="1"' : ""} value="${safe(field.default)}">`;
  return `<div class="field"><div class="floating-control ${type === "password" ? "has-secret" : ""}">${input}${label}${type === "password" ? `<button type="button" class="reveal-secret" data-reveal="${id}" aria-label="${safe(field.label)} anzeigen" aria-pressed="false">Anzeigen</button>` : ""}</div><p id="${id}-help" class="field-help">${safe(field.help)}</p><p id="${id}-example" class="field-example">Beispiel: <code>${safe(field.example)}</code></p></div>`;
}
function renderProviderFields() {
  closeInfoHelp();
  const provider = currentProvider();
  document.querySelector("#provider-name").textContent = provider.name;
  document.querySelector("#provider-guide-title").textContent =
    `So geht’s bei ${provider.name}`;
  document.querySelector("#provider-guide-text").textContent = provider.guide;
  document.querySelector("#provider-docs").href = provider.docs;
  document.querySelector("#credentials-help-list").innerHTML = provider.fields
    .map(
      (field) =>
        `<strong>${safe(field.label)}</strong><p>${safe(field.help)}</p>`,
    )
    .join("");
  document.querySelector("#domain-example").innerHTML =
    `Beispiel: <code>${safe(provider.domainExample)}</code>`;
  document.querySelector("#provider-fields").innerHTML = provider.fields
    .filter((f) => f.required)
    .map(fieldMarkup)
    .join("");
  document.querySelector("#advanced-fields").innerHTML = provider.fields
    .filter((f) => !f.required)
    .map(fieldMarkup)
    .join("");
  document.querySelector("#advanced-settings").hidden = !provider.fields.some(
    (f) => !f.required,
  );
  document.querySelector("#advanced-settings").open = false;
  document.querySelector("#domain").setCustomValidity("");
  attachInfoHelp(document.querySelector("#provider-fields"));
  attachInfoHelp(document.querySelector("#advanced-fields"));
  filterProviders();
}
function clearProviderSecrets() {
  document.querySelectorAll("[data-config]").forEach((input) => {
    input.value = "";
    if (input.type === "checkbox") input.checked = false;
  });
}
function filterProviders() {
  const query = document
    .querySelector("#provider-search")
    .value.trim()
    .toLocaleLowerCase();
  const visible = PROVIDERS.filter((p) =>
    `${p.name} ${uiText(p.name)} ${p.id}`.toLocaleLowerCase().includes(query),
  );
  document.querySelector("#provider-options").innerHTML =
    visible
      .map(
        (p) =>
          `<button type="button" role="option" class="provider-option" id="option-${safe(p.id)}" data-provider="${safe(p.name)}" aria-selected="${p.name === currentProvider().name}"><span>${safe(p.name)}</span>${p.name === currentProvider().name ? '<span aria-hidden="true">✓</span>' : ""}</button>`,
      )
      .join("") ||
    '<p class="no-providers">Kein Anbieter gefunden. Versuche einen anderen Namen.</p>';
  document.querySelector("#provider-results").textContent =
    `${visible.length} von ${PROVIDERS.length} Anbietern`;
}
function closeProviderPicker() {
  document.querySelector("#provider-popup").hidden = true;
  document
    .querySelector("#provider-trigger")
    .setAttribute("aria-expanded", "false");
}
function openProviderPicker() {
  closeInfoHelp();
  document.querySelector("#provider-search").value = "";
  filterProviders();
  document.querySelector("#provider-popup").hidden = false;
  document
    .querySelector("#provider-trigger")
    .setAttribute("aria-expanded", "true");
  document.querySelector("#provider-search").focus();
}
document
  .querySelector("#provider-trigger")
  .addEventListener("click", () =>
    document.querySelector("#provider-popup").hidden
      ? openProviderPicker()
      : closeProviderPicker(),
  );
document
  .querySelector("#provider-search")
  .addEventListener("input", filterProviders);
document
  .querySelector("#provider-options")
  .addEventListener("click", (event) => {
    const option = event.target.closest("[data-provider]");
    if (!option) return;
    document.querySelector("#provider").value = option.dataset.provider;
    document.querySelector("#provider").dispatchEvent(new Event("change"));
    closeProviderPicker();
    document.querySelector("#provider-trigger").focus();
  });
document
  .querySelector("#provider-picker")
  .addEventListener("keydown", (event) => {
    if (
      event.key === "Escape" &&
      !document.querySelector("#provider-popup").hidden
    ) {
      event.preventDefault();
      event.stopPropagation();
      closeProviderPicker();
      document.querySelector("#provider-trigger").focus();
    }
    if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
      if (document.querySelector("#provider-popup").hidden) {
        if (event.key.startsWith("Arrow")) {
          event.preventDefault();
          openProviderPicker();
        }
        return;
      }
      if (
        (event.key === "Home" || event.key === "End") &&
        event.target.id === "provider-search"
      )
        return;
      event.preventDefault();
      const options = [...document.querySelectorAll(".provider-option")];
      if (!options.length) return;
      const current = options.indexOf(document.activeElement);
      let next =
        event.key === "Home"
          ? 0
          : event.key === "End"
            ? options.length - 1
            : event.key === "ArrowDown"
              ? (current + 1) % options.length
              : current <= 0
                ? options.length - 1
                : current - 1;
      options[next].focus();
    }
    if (
      event.key === "Tab" &&
      !document.querySelector("#provider-popup").hidden
    ) {
      const picker = document.querySelector("#provider-picker");
      setTimeout(() => {
        if (!picker.contains(document.activeElement)) closeProviderPicker();
      }, 0);
    }
  });
document.addEventListener("click", (event) => {
  if (!document.querySelector("#provider-picker").contains(event.target))
    closeProviderPicker();
});
document.querySelector("#entry-dialog").addEventListener("close", () => {
  clearProviderSecrets();
  closeProviderPicker();
});
document.querySelector("#entry-dialog").addEventListener("click", (event) => {
  const button = event.target.closest("[data-reveal]");
  if (!button) return;
  const input = document.getElementById(button.dataset.reveal);
  const show = input.type === "password";
  input.type = show ? "text" : "password";
  button.textContent = show ? "Verbergen" : "Anzeigen";
  button.setAttribute("aria-pressed", String(show));
  button.setAttribute(
    "aria-label",
    `${currentProvider().fields.find((f) => `provider-${f.key}` === input.id).label} ${show ? "verbergen" : "anzeigen"}`,
  );
});

document
  .querySelector("#provider-fields")
  .addEventListener("input", (event) => {
    if (event.target.tagName === "TEXTAREA") event.target.setCustomValidity("");
  });
document.querySelector("#entry-form").addEventListener("submit", (event) => {
  const jsonField = document.querySelector('[data-config="credentials"]');
  if (jsonField && !(jsonField.dataset.saved && jsonField.value === "")) {
    try {
      const value = JSON.parse(jsonField.value);
      if (typeof value !== "object" || Array.isArray(value) || !value)
        throw new Error();
    } catch {
      event.preventDefault();
      event.stopImmediatePropagation();
      jsonField.setCustomValidity(
        uiText(
          "Bitte den vollständigen Inhalt einer gültigen JSON-Schlüsseldatei einfügen.",
        ),
      );
      jsonField.reportValidity();
    }
  }
});
