"use strict";

const $ = (id) => document.getElementById(id);

let view = null;       // from App.Load: settings file, defaults, styles
let file = {};          // the settings file as it will be saved
let overridden = [];  // keys the daemon says are set by env or flags
let effective = null;  // the settings the daemon is using, when it's running
let timerStart = 0; // unix seconds of the shown activity's timer, or 0
let lastDaemonConfig = ""; // the daemon's settings at the last poll, to spot outside edits

const DROP_IN = "~/.config/systemd/user/unbeatable-rpc.service.d/override.conf";

function api() {
  return window.go.main.App;
}

async function waitForBindings() {
  for (let i = 0; i < 100 && !(window.go && window.go.main); i++) {
    await new Promise((r) => setTimeout(r, 20));
  }
}

/* settings */

function renderStyles() {
  const box = $("styles");
  box.replaceChildren();
  // A locked style shows what's in force, not what the file says.
  const current = overridden.includes("style") && effective
    ? effective.style
    : file.style || view.defaults.style;
  for (const s of view.styles) {
    const label = document.createElement("label");
    label.className = "style";

    const name = document.createElement("div");
    name.className = "style-name";
    const radio = document.createElement("input");
    radio.type = "radio";
    radio.name = "style";
    radio.value = s.name;
    radio.checked = s.name === current;
    radio.addEventListener("change", () => save({ style: s.name }));
    name.append(radio, s.name + (s.name === view.defaults.style ? " (default)" : ""));
    label.append(name);

    for (const p of s.previews) {
      const row = document.createElement("div");
      row.className = "preview";
      const where = document.createElement("div");
      where.className = "preview-where";
      where.textContent = p.where;
      const details = document.createElement("div");
      details.className = "presence-line";
      details.textContent = p.details;
      const state = document.createElement("div");
      state.className = "presence-line";
      state.textContent = p.state || " ";
      row.append(where, details, state);
      label.append(row);
    }
    box.append(label);
  }
  applyLocks();
}

function renderAdvanced() {
  const form = $("advanced");
  form.largeImage.value = file.largeImage || "";
  form.clientId.value = file.clientId || "";
  form.clientId.placeholder = view.defaults.clientId;
  form.socket.value = file.socket || "";
  form.verbose.checked = !!file.verbose;
  applyLocks();
}

function applyLocks() {
  const styleLocked = overridden.includes("style");
  for (const r of document.querySelectorAll('input[name="style"]')) r.disabled = styleLocked;
  const note = $("style-locked");
  note.hidden = !styleLocked;
  if (styleLocked) {
    note.textContent = `The style is set by ${view.envVars.style} (or --style) in the daemon's environment, which wins over this app. ` +
      `Remove it from ${DROP_IN}, then run: systemctl --user daemon-reload && systemctl --user restart unbeatable-rpc`;
  }
  const form = $("advanced");
  for (const key of ["largeImage", "clientId", "socket", "verbose"]) {
    const input = form[key];
    input.disabled = overridden.includes(key);
    input.title = input.disabled ? `Set by ${view.envVars[key]} in the daemon's environment` : "";
  }
}

let savedTimer = 0;

async function save(change) {
  file = { ...file, ...change };
  for (const k of Object.keys(file)) {
    if (file[k] === "" || file[k] === false) delete file[k];
  }
  try {
    await api().Save(file);
    const saved = $("saved");
    saved.hidden = false;
    clearTimeout(savedTimer);
    savedTimer = setTimeout(() => (saved.hidden = true), 1500);
  } catch (err) {
    alert("Couldn't save the settings: " + err);
  }
}

$("advanced").addEventListener("change", (e) => {
  const input = e.target;
  save({ [input.name]: input.type === "checkbox" ? input.checked : input.value.trim() });
});

/* status */

function ago(iso) {
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  return s < 2 ? "just now" : `${s} s ago`;
}

function clock(seconds) {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  const ss = String(s).padStart(2, "0");
  // Discord's format: 0:02, 12:34, 1:02:03
  return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

function tickTimer() {
  $("p-timer").hidden = !timerStart;
  $("p-timer-text").textContent = timerStart
    ? clock(Math.max(0, Math.floor(Date.now() / 1000) - timerStart))
    : "";
}

async function refreshStatus() {
  let ds;
  try {
    ds = await api().Status();
  } catch (err) {
    ds = { running: false, error: String(err) };
  }

  $("daemon-up").hidden = !ds.running;
  $("daemon-down").hidden = ds.running;
  $("daemon-version").hidden = !ds.running;
  if (!ds.running) {
    timerStart = 0;
    return;
  }

  const st = ds.status;
  $("daemon-version").textContent = "Daemon " + st.version;

  $("game").textContent = st.gameRunning
    ? "Running" + (st.lastHeard ? ` (last heard ${ago(st.lastHeard)})` : "")
    : "Not detected";
  $("discord").textContent = st.discord.connected
    ? "Connected"
    : st.gameRunning
      ? "Not connected" + (st.discord.lastError ? ": " + st.discord.lastError : "")
      : "Idle until the game starts";

  const a = st.activity;
  $("presence").hidden = !a;
  $("presence-empty").hidden = !!a;
  if (a) {
    $("p-details").textContent = a.details || "";
    $("p-state").textContent = a.state || "";
    timerStart = a.timestamps && a.timestamps.start ? a.timestamps.start : 0;
  } else {
    timerStart = 0;
  }
  tickTimer();

  const cfgErr = $("config-error");
  cfgErr.hidden = !st.configError;
  cfgErr.textContent = st.configError ? "Settings file problem: " + st.configError : "";

  effective = st.config;
  const newOverridden = st.overridden || [];
  if (newOverridden.join() !== overridden.join()) {
    overridden = newOverridden;
    renderStyles();
  }

  // The settings file was changed by something else (an editor, another
  // window): re-read it so the page shows what's in force.
  const daemonConfig = JSON.stringify(st.config);
  if (lastDaemonConfig && daemonConfig !== lastDaemonConfig) await reloadFile();
  lastDaemonConfig = daemonConfig;
}

async function reloadFile() {
  view = await api().Load();
  file = { ...view.file };
  renderStyles();
  if (!$("advanced").contains(document.activeElement)) renderAdvanced();
}

$("start-daemon").addEventListener("click", async () => {
  const btn = $("start-daemon");
  const err = $("start-error");
  btn.disabled = true;
  err.hidden = true;
  try {
    await api().StartDaemon();
    setTimeout(refreshStatus, 500);
  } catch (e) {
    err.textContent = String(e);
    err.hidden = false;
  } finally {
    btn.disabled = false;
  }
});

$("uninstall").addEventListener("click", async () => {
  const btn = $("uninstall");
  const err = $("uninstall-error");
  btn.disabled = true;
  err.hidden = true;
  try {
    // Go asks for confirmation, and quits the app once it's done
    await api().Uninstall($("uninstall-all").checked);
  } catch (e) {
    err.textContent = String(e);
    err.hidden = false;
  } finally {
    btn.disabled = false;
  }
});

async function renderLocations() {
  const box = $("locations");
  box.replaceChildren();
  for (const loc of await api().Locations()) {
    const text = document.createElement("div");
    text.className = "location-text";
    const label = document.createElement("div");
    label.textContent = loc.label;
    const path = document.createElement("code");
    // the bdi keeps the path's own order inside the right-to-left overflow trick
    const bdi = document.createElement("bdi");
    bdi.textContent = loc.path ? loc.short + (loc.exists ? "" : " (not found)") : "not found";
    path.append(bdi);
    path.title = loc.path;
    text.append(label, path);
    const browse = document.createElement("button");
    browse.className = "quiet";
    browse.textContent = "Browse";
    browse.disabled = !loc.exists;
    browse.addEventListener("click", async () => {
      const err = $("browse-error");
      err.hidden = true;
      try {
        await api().Browse(loc.key);
      } catch (e) {
        err.textContent = String(e);
        err.hidden = false;
      }
    });
    const row = document.createElement("div");
    row.className = "location";
    row.append(text, browse);
    box.append(row);
  }
}

/* start */

(async () => {
  await waitForBindings();
  view = await api().Load();
  file = { ...view.file };
  renderStyles();
  renderAdvanced();
  renderLocations();
  if (view.fileError) {
    $("config-error").textContent = "Settings file problem: " + view.fileError;
    $("config-error").hidden = false;
  }
  await refreshStatus();
  setInterval(refreshStatus, 2000);
  setInterval(tickTimer, 1000);
})();
