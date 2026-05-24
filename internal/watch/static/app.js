const token = new URLSearchParams(location.search).get("setup_token") || "";
const headers = token ? {"X-Setup-Token": token} : {};

let model = null;
let selectedKeyID = "";

const summary = document.querySelector("#summary");
const fatalBanner = document.querySelector("#fatalBanner");
const keysList = document.querySelector("#keysList");
const keyForm = document.querySelector("#keyForm");
const editorTitle = document.querySelector("#editorTitle");
const countryPicker = document.querySelector("#countryPicker");
const keyFormMessage = document.querySelector("#keyFormMessage");
const deleteKeyButton = document.querySelector("#deleteKey");
const output = document.querySelector("#output");
const settingsDialog = document.querySelector("#settingsDialog");
const settingsForm = document.querySelector("#settingsForm");
const setupBox = document.querySelector("#setupBox");
const keyDialog = document.querySelector("#keyDialog");
const keyDetailsDialog = document.querySelector("#keyDetailsDialog");
const keyDetails = document.querySelector("#keyDetails");
const detailsTitle = document.querySelector("#detailsTitle");

async function api(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    headers: {"Content-Type": "application/json", ...headers, ...(options.headers || {})}
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(data.error || JSON.stringify(data));
    err.data = data;
    throw err;
  }
  return data;
}

async function refresh({quiet = false} = {}) {
  try {
    model = await api(`/api/status${token ? "?setup_token=" + encodeURIComponent(token) : ""}`);
    render();
  } catch (err) {
    showFatal(err.message);
    if (!quiet) show(err.message);
  }
}

function render() {
  const st = model.state || {};
  const cfg = model.config || {};
  summary.textContent = `${titleStatus(st.status || "unknown")} · last ${formatTime(st.last_check)} · next ${formatTime(model.next_check)}${model.fixture ? " · fixture" : ""}`;
  renderFatal(st);
  renderSetup();
  renderKeys(cfg.keys || [], st.keys || {});
  renderSettings(cfg);
  if (keyDialog.open) renderKeyEditor(currentKey());
  if (keyDetailsDialog.open) renderKeyDetails(selectedKeyID);
}

function renderFatal(st) {
  if (st.status === "api_error" && st.last_error) {
    showFatal(st.last_error);
    return;
  }
  fatalBanner.classList.add("hidden");
}

function showFatal(message) {
  fatalBanner.textContent = message;
  fatalBanner.classList.remove("hidden");
}

function renderSetup() {
  const req = model.setup_requirements || {};
  const missing = [];
  if (!req.admin_password) missing.push("admin password");
  if (!req.gateway_public_keys) missing.push("gateway public keys");
  if (!req.amnezia_keys) missing.push("AmneziaVPN key");
  if (!missing.length) {
    setupBox.classList.add("hidden");
    return;
  }
  setupBox.classList.remove("hidden");
  setupBox.textContent = `Initial setup: add ${missing.join(", ")}.`;
  if (model.setup_mode && !settingsDialog.open && !keyDialog.open) settingsDialog.showModal();
}

function renderKeys(keys, keyStates) {
  keysList.innerHTML = "";
  if (!keys.length) {
    keysList.innerHTML = `<div class="empty-note">No keys yet. Add an AmneziaVPN key to start watching countries.</div>`;
    return;
  }
  for (const key of keys) {
    const state = keyStates[key.id] || {};
    const account = state.last_account || {};
    const watched = key.countries || [];
    const changedCount = Object.values(state.countries || {}).filter(c => c.status === "changed" || c.status === "missing").length;
    const card = document.createElement("article");
    card.className = "key-card";
    card.innerHTML = `
      <div class="key-card-main" data-action="details" data-id="${escapeHtml(key.id)}">
        <div class="key-title-row">
          <div>
            <h3>${escapeHtml(key.name || "Key")}</h3>
            <div class="muted small">${escapeHtml(watched.map(countryLabel).join(", ") || "No countries selected")}</div>
          </div>
          <span class="pill ${escapeHtml(state.status || "unknown")}">${titleStatus(state.status || "unknown")}</span>
        </div>
        <div class="metric-row">
          ${metric("Last check", formatTime(state.last_check))}
          ${metric("Next check", formatTime(model.next_check))}
          ${metric("Devices", account.max_device_count ? `${account.active_device_count || 0}/${account.max_device_count}` : "-")}
          ${metric("Subscription", formatDate(account.subscription_end_date))}
          ${metric("Available", account.available_countries?.length ?? "-")}
          ${metric("Issued", account.issued_country_configs?.length ?? "-")}
          ${metric("Changed", changedCount)}
        </div>
        ${state.last_error ? `<div class="inline-error">${escapeHtml(state.last_error)}</div>` : ""}
        ${renderCountryRows(state.countries || {})}
      </div>
      <div class="card-actions">
        <button type="button" class="secondary" data-action="details" data-id="${escapeHtml(key.id)}">Details</button>
        <button type="button" data-action="edit" data-id="${escapeHtml(key.id)}">Edit</button>
      </div>
    `;
    keysList.appendChild(card);
  }
}

function metric(label, value) {
  return `<div class="metric"><span>${escapeHtml(label)}</span><strong>${escapeHtml(value || "-")}</strong></div>`;
}

function renderCountryRows(countries) {
  const rows = Object.values(countries).sort((a, b) => a.code.localeCompare(b.code));
  if (!rows.length) return `<div class="muted small">Run a check to create a baseline.</div>`;
  return `<div class="country-rows">${rows.map(c => `
    <span>${countryFlag(c.code)} ${escapeHtml(c.code)}</span>
    <span class="${escapeHtml(c.status || "unknown")}">${titleStatus(c.status || "unknown")}</span>
    <span class="muted">worker ${formatDateTime(c.worker_last_updated)}</span>
    <span class="muted">downloaded ${formatDateTime(c.last_downloaded)}</span>
  `).join("")}</div>`;
}

function openKeyEditor(key) {
  selectedKeyID = key?.id || "";
  renderKeyEditor(key || null);
  keyDialog.showModal();
}

function renderKeyEditor(key) {
  keyForm.reset();
  keyFormMessage.classList.add("hidden");
  deleteKeyButton.classList.toggle("hidden", !key);
  editorTitle.textContent = key ? "Edit key" : "Add key";
  keyForm.elements.id.value = key?.id || "";
  keyForm.elements.name.value = key?.name || "";
  renderCountryPicker(key);
}

function renderCountryPicker(key) {
  const state = (model.state?.keys || {})[key?.id || ""] || {};
  const available = state.last_account?.available_countries || [];
  const issued = new Set((state.last_account?.issued_country_configs || []).map(c => c.code));
  const selected = new Set(key?.countries || []);
  countryPicker.innerHTML = "";
  if (!available.length) {
    countryPicker.innerHTML = `<div class="empty-note">Save the key once. The app will check it and load available countries automatically.</div>`;
    return;
  }
  for (const country of available) {
    const label = document.createElement("label");
    label.className = "country-option";
    label.innerHTML = `
      <input type="checkbox" value="${escapeHtml(country.code)}" ${selected.has(country.code) ? "checked" : ""}>
      <span>${countryFlag(country.code)} ${escapeHtml(country.code)}</span>
      <span>${escapeHtml(country.name || "")}</span>
      ${issued.has(country.code) ? `<em>in use</em>` : `<em class="muted">available</em>`}
    `;
    countryPicker.appendChild(label);
  }
}

function renderKeyDetails(keyID) {
  const key = (model.config?.keys || []).find(k => k.id === keyID);
  const state = (model.state?.keys || {})[keyID] || {};
  const account = state.last_account || {};
  if (!key) return;
  detailsTitle.textContent = key.name || "Key status";
  const issued = account.issued_country_configs || [];
  keyDetails.innerHTML = `
    <div class="details-grid">
      ${metric("Status", titleStatus(state.status || "unknown"))}
      ${metric("Last check", formatTime(state.last_check))}
      ${metric("Next check", formatTime(model.next_check))}
      ${metric("Errors", state.error_count || 0)}
      ${metric("Devices", account.max_device_count ? `${account.active_device_count || 0}/${account.max_device_count}` : "-")}
      ${metric("Subscription ends", formatDate(account.subscription_end_date))}
      ${metric("Available countries", account.available_countries?.length ?? "-")}
      ${metric("Issued configs", issued.length)}
    </div>
    ${state.last_error ? `<div class="inline-error">${escapeHtml(state.last_error)}</div>` : ""}
    <h3>Issued country configs</h3>
    <div class="detail-table">
      <span>Country</span><span>Worker updated</span><span>Last downloaded</span><span>UUID</span>
      ${issued.map(c => `
        <span>${countryFlag(c.code)} ${escapeHtml(c.code)} ${escapeHtml(c.name || "")}</span>
        <span>${formatDateTime(c.worker_last_updated)}</span>
        <span>${formatDateTime(c.last_downloaded)}</span>
        <span class="mono">${escapeHtml(c.installation_uuid || "-")}</span>
      `).join("")}
    </div>
  `;
}

function renderSettings(cfg) {
  settingsForm.poll_interval_hours.value = cfg.poll_interval_hours || 6;
  settingsForm.gateway_endpoint.value = cfg.amnezia?.gateway_endpoint || "";
  settingsForm.gateway_public_key_filepath.value = cfg.amnezia?.gateway_public_key_filepath || "";
}

function currentKey() {
  return (model.config?.keys || []).find(k => k.id === selectedKeyID) || null;
}

function currentKeysFromUI(nextKey) {
  const keys = (model.config?.keys || []).map(k => ({
    id: k.id,
    name: k.name,
    countries: k.countries || []
  }));
  if (!nextKey) return keys;
  const idx = keys.findIndex(k => k.id === nextKey.id);
  if (idx >= 0) keys[idx] = nextKey;
  else keys.push(nextKey);
  return keys;
}

keyForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const countries = [...countryPicker.querySelectorAll("input:checked")].map(input => input.value);
  const nextKey = {
    id: keyForm.elements.id.value,
    name: keyForm.elements.name.value,
    vpn_key: keyForm.elements.vpn_key.value,
    countries
  };
  try {
    const data = await saveConfig({keys: currentKeysFromUI(nextKey)});
    selectedKeyID = data.config.keys.find(k => k.name === nextKey.name)?.id || nextKey.id || selectedKeyID;
    await api("/api/check", {method: "POST", body: "{}"}).catch(() => null);
    await refresh();
    keyForm.elements.vpn_key.value = "";
    keyFormMessage.textContent = "Saved. Available countries and current config status are updated.";
    keyFormMessage.classList.remove("hidden");
  } catch (err) {
    showFatal(err.message);
  }
});

deleteKeyButton.addEventListener("click", async () => {
  const keys = (model.config?.keys || []).filter(k => k.id !== keyForm.elements.id.value).map(k => ({
    id: k.id,
    name: k.name,
    countries: k.countries || []
  }));
  selectedKeyID = keys[0]?.id || "";
  await saveConfig({keys});
  keyDialog.close();
  await refresh();
});

keysList.addEventListener("click", (event) => {
  const target = event.target.closest("[data-action]");
  if (!target) return;
  const key = (model.config?.keys || []).find(k => k.id === target.dataset.id);
  if (!key) return;
  selectedKeyID = key.id;
  if (target.dataset.action === "edit") openKeyEditor(key);
  if (target.dataset.action === "details") {
    renderKeyDetails(key.id);
    keyDetailsDialog.showModal();
  }
});

document.querySelector("#addKey").addEventListener("click", () => openKeyEditor(null));

settingsForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    const data = await saveConfig({
      web_password: settingsForm.web_password.value,
      poll_interval_hours: Number(settingsForm.poll_interval_hours.value || 6),
      gateway_public_keys: settingsForm.gateway_public_keys.value,
      telegram: {
        bot_token: settingsForm.bot_token.value,
        chat_id: settingsForm.chat_id.value
      },
      amnezia: {
        gateway_endpoint: settingsForm.gateway_endpoint.value,
        gateway_public_key_filepath: settingsForm.gateway_public_key_filepath.value
      }
    });
    settingsForm.web_password.value = "";
    settingsForm.gateway_public_keys.value = "";
    settingsForm.bot_token.value = "";
    settingsDialog.close();
    if (token && (data.setup_complete || data.password_changed)) {
      window.history.replaceState({}, document.title, window.location.pathname);
      window.location.replace(window.location.pathname);
      return;
    }
    await refresh();
  } catch (err) {
    showFatal(err.message);
  }
});

async function saveConfig(patch) {
  const cfg = model?.config || {};
  const body = {
    poll_interval_hours: patch.poll_interval_hours ?? cfg.poll_interval_hours ?? 6,
    telegram: patch.telegram || {},
    amnezia: patch.amnezia || {},
    web_password: patch.web_password || "",
    gateway_public_keys: patch.gateway_public_keys || ""
  };
  if (Object.prototype.hasOwnProperty.call(patch, "keys")) body.keys = patch.keys;
  return api("/api/settings", {method: "POST", body: JSON.stringify(body)});
}

document.querySelector("#checkNow").addEventListener("click", async () => {
  try {
    show("Checking...");
    show(await api("/api/check", {method: "POST", body: "{}"}));
    await refresh();
  } catch (err) {
    show(err.data || err.message);
    await refresh({quiet: true});
  }
});

document.querySelector("#telegramTest").addEventListener("click", async () => {
  try {
    show(await api("/api/telegram/test", {method: "POST", body: "{}"}));
  } catch (err) {
    show(err.message);
  }
});

document.querySelector("#openSettings").addEventListener("click", () => settingsDialog.showModal());
document.querySelector("#closeSettings").addEventListener("click", () => settingsDialog.close());
document.querySelector("#closeKeyDialog").addEventListener("click", () => keyDialog.close());
document.querySelector("#closeDetailsDialog").addEventListener("click", () => keyDetailsDialog.close());
document.querySelector("#diagnostics").addEventListener("click", (event) => {
  if (token) event.currentTarget.href = `/api/diagnostics?setup_token=${encodeURIComponent(token)}`;
});

document.querySelectorAll(".tab").forEach(tab => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach(t => t.classList.remove("active"));
    document.querySelectorAll(".tab-panel").forEach(panel => panel.classList.remove("active"));
    tab.classList.add("active");
    document.querySelector(`#tab-${tab.dataset.tab}`).classList.add("active");
  });
});

function titleStatus(status) {
  return String(status || "unknown").replace(/_/g, " ");
}

function formatTime(value) {
  if (!value || value.startsWith?.("0001-")) return "not yet";
  return new Intl.DateTimeFormat(undefined, {month: "short", day: "numeric", hour: "2-digit", minute: "2-digit"}).format(new Date(value));
}

function formatDate(value) {
  if (!value) return "-";
  return new Intl.DateTimeFormat(undefined, {year: "numeric", month: "short", day: "numeric"}).format(new Date(value));
}

function formatDateTime(value) {
  if (!value) return "-";
  return new Intl.DateTimeFormat(undefined, {month: "short", day: "numeric", hour: "2-digit", minute: "2-digit"}).format(new Date(value));
}

function countryLabel(code) {
  return `${countryFlag(code)} ${code}`;
}

function countryFlag(code) {
  const cc = String(code || "").toUpperCase();
  if (!/^[A-Z]{2}$/.test(cc)) return "";
  return cc.replace(/./g, ch => String.fromCodePoint(127397 + ch.charCodeAt(0)));
}

function show(value) {
  output.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[ch]));
}

refresh();
setInterval(() => refresh({quiet: true}), 10000);
