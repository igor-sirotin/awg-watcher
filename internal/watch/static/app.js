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
const deleteKeyButton = document.querySelector("#deleteKey");
const output = document.querySelector("#output");
const settingsDialog = document.querySelector("#settingsDialog");
const settingsForm = document.querySelector("#settingsForm");
const setupBox = document.querySelector("#setupBox");

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
  const keys = cfg.keys || [];
  summary.textContent = `${st.status || "unknown"}${st.last_check ? " · " + new Date(st.last_check).toLocaleString() : ""}${model.fixture ? " · fixture mode" : ""}`;
  renderFatal(st);
  renderSetup();
  renderKeys(keys, st.keys || {});
  renderSettings(cfg);
  if (!selectedKeyID && keys.length) selectedKeyID = keys[0].id;
  renderEditor(keys.find(k => k.id === selectedKeyID) || null);
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
  if (model.setup_mode && !settingsDialog.open) settingsDialog.showModal();
}

function renderKeys(keys, keyStates) {
  keysList.innerHTML = "";
  if (!keys.length) {
    keysList.innerHTML = `<div class="empty-note">No keys yet. Add an AmneziaVPN key to start watching countries.</div>`;
    return;
  }
  for (const key of keys) {
    const state = keyStates[key.id] || {};
    const card = document.createElement("button");
    card.type = "button";
    card.className = `key-card ${selectedKeyID === key.id ? "selected" : ""}`;
    card.innerHTML = `
      <div>
        <b>${escapeHtml(key.name || "Key")}</b>
        <span class="muted">${escapeHtml((key.countries || []).join(", ") || "No countries selected")}</span>
      </div>
      <span class="pill ${escapeHtml(state.status || "unknown")}">${escapeHtml(state.status || "unknown")}</span>
      ${renderCountryRows(state.countries || {})}
    `;
    card.addEventListener("click", () => {
      selectedKeyID = key.id;
      render();
    });
    keysList.appendChild(card);
  }
}

function renderCountryRows(countries) {
  const rows = Object.values(countries).sort((a, b) => a.code.localeCompare(b.code));
  if (!rows.length) return `<div class="muted small">Run a check to create a baseline.</div>`;
  return `<div class="country-rows">${rows.map(c => `
    <span>${escapeHtml(c.code)}</span>
    <span class="${escapeHtml(c.status || "unknown")}">${escapeHtml(c.status || "unknown")}</span>
    <span class="muted">${escapeHtml(c.worker_last_updated || "-")}</span>
  `).join("")}</div>`;
}

function renderEditor(key) {
  keyForm.reset();
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
    countryPicker.innerHTML = `<div class="empty-note">Save the key and run a check to load available countries.</div>`;
    return;
  }
  for (const country of available) {
    const label = document.createElement("label");
    label.className = "country-option";
    label.innerHTML = `
      <input type="checkbox" value="${escapeHtml(country.code)}" ${selected.has(country.code) ? "checked" : ""}>
      <span>${escapeHtml(country.code)}</span>
      <span>${escapeHtml(country.name || "")}</span>
      ${issued.has(country.code) ? `<em>in use</em>` : `<em class="muted">available</em>`}
    `;
    countryPicker.appendChild(label);
  }
}

function renderSettings(cfg) {
  settingsForm.poll_interval_hours.value = cfg.poll_interval_hours || 6;
  settingsForm.gateway_endpoint.value = cfg.amnezia?.gateway_endpoint || "";
  settingsForm.gateway_public_key_filepath.value = cfg.amnezia?.gateway_public_key_filepath || "";
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
  await refresh();
});

document.querySelector("#addKey").addEventListener("click", () => {
  selectedKeyID = "";
  renderEditor(null);
});

settingsForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await saveConfig({
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
  if (Object.prototype.hasOwnProperty.call(patch, "keys")) {
    body.keys = patch.keys;
  }
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

function show(value) {
  output.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[ch]));
}

refresh();
setInterval(() => refresh({quiet: true}), 10000);
