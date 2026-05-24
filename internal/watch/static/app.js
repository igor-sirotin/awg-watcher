const token = new URLSearchParams(location.search).get("setup_token") || "";
const headers = token ? {"X-Setup-Token": token} : {};
const output = document.querySelector("#output");
const summary = document.querySelector("#summary");
const statusGrid = document.querySelector("#statusGrid");
const form = document.querySelector("#settingsForm");

async function api(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    headers: {"Content-Type": "application/json", ...headers, ...(options.headers || {})}
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || JSON.stringify(data));
  return data;
}

function show(value) {
  output.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

function render(data) {
  const cfg = data.config || {};
  const st = data.state || {};
  summary.textContent = `${st.status || "unknown"}${st.last_check ? " · " + new Date(st.last_check).toLocaleString() : ""}${data.fixture ? " · fixture mode" : ""}`;
  form.countries.value = (cfg.countries || []).join(", ");
  form.poll_interval_hours.value = cfg.poll_interval_hours || 6;
  form.gateway_endpoint.value = cfg.amnezia?.gateway_endpoint || "";
  form.gateway_public_key_filepath.value = cfg.amnezia?.gateway_public_key_filepath || "";

  const countries = Object.values(st.countries || {});
  statusGrid.innerHTML = "";
  if (!countries.length) {
    const div = document.createElement("div");
    div.className = "muted";
    div.textContent = "No baseline yet";
    statusGrid.appendChild(div);
  }
  for (const c of countries) {
    const div = document.createElement("div");
    div.className = "tile";
    div.innerHTML = `<b>${escapeHtml(c.code)} ${escapeHtml(c.name || "")}</b>
      <div class="${escapeHtml(c.status || "unknown")}">${escapeHtml(c.status || "unknown")}</div>
      <div class="muted">worker_last_updated: ${escapeHtml(c.worker_last_updated || "-")}</div>
      <div class="muted">last_downloaded: ${escapeHtml(c.last_downloaded || "-")}</div>`;
    statusGrid.appendChild(div);
  }
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[ch]));
}

async function refresh() {
  try {
    const data = await api(`/api/status${token ? "?setup_token=" + encodeURIComponent(token) : ""}`, {headers});
    render(data);
  } catch (err) {
    show(err.message);
  }
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const countries = form.countries.value.split(",").map(s => s.trim()).filter(Boolean);
  const body = {
    web_password: form.web_password.value,
    vpn_key: form.vpn_key.value,
    countries,
    poll_interval_hours: Number(form.poll_interval_hours.value || 6),
    telegram: {
      bot_token: form.bot_token.value,
      chat_id: form.chat_id.value
    },
    amnezia: {
      gateway_endpoint: form.gateway_endpoint.value,
      gateway_public_key_filepath: form.gateway_public_key_filepath.value
    }
  };
  try {
    const data = await api("/api/settings", {method: "POST", body: JSON.stringify(body)});
    show(data);
    form.vpn_key.value = "";
    form.bot_token.value = "";
    form.web_password.value = "";
    await refresh();
  } catch (err) {
    show(err.message);
  }
});

document.querySelector("#checkNow").addEventListener("click", async () => {
  try {
    show("Checking...");
    const data = await api("/api/check", {method: "POST", body: "{}"});
    show(data);
    await refresh();
  } catch (err) {
    show(err.message);
    await refresh();
  }
});

document.querySelector("#decodeKey").addEventListener("click", async () => {
  try {
    const data = await api("/api/decode", {method: "POST", body: JSON.stringify({vpn_key: form.vpn_key.value})});
    show(data);
  } catch (err) {
    show(err.message);
  }
});

document.querySelector("#telegramTest").addEventListener("click", async () => {
  try {
    show(await api("/api/telegram/test", {method: "POST", body: "{}"}));
  } catch (err) {
    show(err.message);
  }
});

document.querySelector("#diagnostics").addEventListener("click", (event) => {
  if (token) event.currentTarget.href = `/api/diagnostics?setup_token=${encodeURIComponent(token)}`;
});

refresh();
