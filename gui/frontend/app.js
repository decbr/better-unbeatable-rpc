"use strict";

const $ = (id) => document.getElementById(id);

let view = null;       // from App.Load: settings file, defaults, styles
let file = {};          // the settings file as it will be saved
let overridden = [];  // keys the daemon says are set by env or flags
let effective = null;  // the settings the daemon is using, when it's running
let timerStart = 0; // unix seconds of the shown activity's timer, or 0
let lastDaemonConfig = ""; // the daemon's settings at the last poll, to spot outside edits
let lastStatus = null;  // the last App.Status result
let health = null;      // the last App.Health result: checks, mod version, game open

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
  renderShowSwitch();
  applyLocks();
}

function renderShowSwitch() {
  const hidden = overridden.includes("hidden") && effective ? effective.hidden : file.hidden;
  $("show-on-discord").checked = !hidden;
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
  const show = $("show-on-discord");
  show.disabled = overridden.includes("hidden");
  show.title = show.disabled ? `Set by ${view.envVars.hidden} in the daemon's environment` : "";
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

$("show-on-discord").addEventListener("change", (e) => save({ hidden: !e.target.checked }));

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

  lastStatus = ds;
  $("daemon-up").hidden = !ds.running;
  $("daemon-down").hidden = ds.running;
  renderHelp();
  renderVersions();
  if (!ds.running) {
    timerStart = 0;
    return;
  }

  const st = ds.status;

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
  $("presence-empty").textContent = st.hidden
    ? "Hidden from Discord. Tick Show on Discord to put it back."
    : "Nothing shown in Discord right now.";
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
  renderShowSwitch();

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

/* setup checks and help */

const MARKS = { ok: "✓", warn: "!", bad: "✕", unknown: "?" };

async function refreshHealth() {
  const wasOpen = health && health.gameOpen;
  health = await api().Health();
  const list = $("checks");
  list.replaceChildren();
  for (const c of health.checks) {
    const li = document.createElement("li");
    const mark = document.createElement("span");
    mark.className = "check-mark " + c.level;
    mark.textContent = MARKS[c.level] || "?";
    const text = document.createElement("span");
    const label = document.createElement("span");
    label.className = "check-label";
    label.textContent = c.label + ": ";
    text.append(label, c.detail);
    li.append(mark, text);
    if (c.fix && c.level !== "ok") {
      const fix = document.createElement("span");
      fix.className = "check-fix";
      fix.textContent = c.fix;
      li.append(fix);
    }
    list.append(li);
  }
  renderHelp();
  renderVersions();
  if (wasOpen !== health.gameOpen) loadPluginSettings();
}

// renderHelp explains the problem the status shows, where there is one.
function renderHelp() {
  const lines = [];
  const ds = lastStatus;
  if (ds && ds.running) {
    const st = ds.status;
    if (st.gameRunning && st.discord.lastError) {
      lines.push("Can't reach Discord. In Vesktop, turn on Settings → Rich Presence (arRPC). " +
        "In Discord, turn on User Settings → Activity Privacy → Share your detected activities. " +
        "If Discord's socket is somewhere unusual, set its path under Advanced.");
    }
    if (health && health.gameOpen && !st.gameRunning) {
      lines.push("The game is open but the mod isn't sending anything. Setup below shows what's wrong.");
    }
    if (st.activity && st.discord.connected) {
      lines.push("Not on your profile? Discord shows one game at a time, so another app's status can take its place.");
    }
  }
  const box = $("help");
  box.replaceChildren(...lines.map((t) => Object.assign(document.createElement("p"), { textContent: t })));
  box.hidden = lines.length === 0;
}

// Go checks the copy really landed; if it didn't, the report is shown
// selected so Ctrl+C works.
$("copy-diagnostics").addEventListener("click", async () => {
  const btn = $("copy-diagnostics");
  const err = $("diagnostics-error");
  const box = $("diagnostics-text");
  btn.disabled = true;
  err.hidden = true;
  box.hidden = true;
  const text = await api().Diagnostics();
  try {
    await api().CopyText(text);
    btn.textContent = "Copied";
    setTimeout(() => (btn.textContent = "Copy diagnostics"), 1500);
  } catch (e) {
    err.textContent = "Couldn't copy it automatically. It's selected below: press Ctrl+C.";
    err.hidden = false;
    box.value = text;
    box.hidden = false;
    box.focus();
    box.select();
  } finally {
    btn.disabled = false;
  }
});

/* advanced: daemon service, mod settings, versions */

async function loadService() {
  const info = await api().ServiceInfo();
  $("service-controls").hidden = !info.installed;
  $("start-at-login").checked = info.enabled;
}

function showServiceError(e) {
  const err = $("service-error");
  err.textContent = e ? String(e) : "";
  err.hidden = !e;
}

$("restart-daemon").addEventListener("click", async () => {
  const btn = $("restart-daemon");
  btn.disabled = true;
  showServiceError(null);
  try {
    await api().RestartDaemon();
    setTimeout(refreshStatus, 800);
  } catch (e) {
    showServiceError(e);
  } finally {
    btn.disabled = false;
  }
});

$("start-at-login").addEventListener("change", async (e) => {
  showServiceError(null);
  try {
    await api().SetStartAtLogin(e.target.checked);
  } catch (err) {
    showServiceError(err);
  }
  loadService();
});

async function loadPluginSettings() {
  const ps = await api().PluginSettings();
  const disable = $("disable-game-discord");
  const debug = $("debug-logging");
  disable.checked = ps.disableGameDiscord;
  debug.checked = ps.debugLogging;
  disable.disabled = debug.disabled = !ps.available || ps.locked;
  $("plugin-note").textContent = !ps.path
    ? "The game folder isn't known, so these can't be changed here."
    : !ps.available
      ? "Launch the game once to create the mod's settings."
      : ps.locked
        ? "Close the game to change these: it rewrites this file while running. They apply next launch."
        : "These apply the next time you launch the game.";
}

let pluginSavedTimer = 0;

for (const id of ["disable-game-discord", "debug-logging"]) {
  $(id).addEventListener("change", async () => {
    const err = $("plugin-error");
    err.hidden = true;
    try {
      await api().SavePluginSettings($("disable-game-discord").checked, $("debug-logging").checked);
      const saved = $("plugin-saved");
      saved.hidden = false;
      clearTimeout(pluginSavedTimer);
      pluginSavedTimer = setTimeout(() => (saved.hidden = true), 1500);
    } catch (e) {
      err.textContent = String(e);
      err.hidden = false;
    }
    loadPluginSettings();
  });
}

let appVersion = "";

function renderVersions() {
  const parts = [];
  if (appVersion) parts.push("App " + appVersion);
  if (lastStatus && lastStatus.running) parts.push("Daemon " + lastStatus.status.version);
  if (health && health.pluginVersion) parts.push("Mod " + health.pluginVersion + " (last launch)");
  $("versions").textContent = parts.join(" · ");
}

/* start */

(async () => {
  await waitForBindings();
  view = await api().Load();
  file = { ...view.file };
  renderStyles();
  renderAdvanced();
  renderLocations();
  appVersion = await api().AppVersion();
  loadService();
  loadPluginSettings();
  if (view.fileError) {
    $("config-error").textContent = "Settings file problem: " + view.fileError;
    $("config-error").hidden = false;
  }
  await refreshStatus();
  await refreshHealth();
  setInterval(refreshStatus, 2000);
  setInterval(refreshHealth, 2000);
  setInterval(tickTimer, 1000);
})();
