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
let platform = "linux";

// renderPlatform swaps in the Windows wording.
function renderPlatform() {
  if (platform !== "windows") return;
  $("socket-label").textContent = "Discord pipe path";
  $("uninstall-all-what").textContent = "BepInEx, if the installer added it";
}

function api() {
  return window.go.main.App;
}

async function waitForBindings() {
  for (let i = 0; i < 100 && !(window.go && window.go.main); i++) {
    await new Promise((r) => setTimeout(r, 20));
  }
}

/* settings */

function previewRows(previews) {
  return previews.map((p, i) => {
    const row = document.createElement("div");
    row.className = i ? "preview more-row" : "preview";
    const where = document.createElement("div");
    where.className = "preview-where";
    where.textContent = p.where;
    const details = document.createElement("div");
    details.className = "presence-line";
    details.textContent = p.details || " ";
    const state = document.createElement("div");
    state.className = "presence-line";
    state.textContent = p.state || " ";
    row.append(where, details, state);
    return row;
  });
}

function actionButton(text, onClick) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "quiet";
  btn.textContent = text;
  btn.addEventListener("click", (e) => {
    e.preventDefault(); // it's inside the card's label: don't pick the style too
    onClick(btn);
  });
  return btn;
}

function currentStyle() {
  // A locked style shows what's in force, not what the file says.
  return overridden.includes("style") && effective
    ? effective.style
    : file.style || view.defaults.style;
}

function renderStyles() {
  const builtins = $("styles-builtin");
  const customs = $("styles-custom");
  builtins.replaceChildren();
  customs.replaceChildren();
  const current = currentStyle().toLowerCase();
  for (const s of view.styles) {
    const label = document.createElement("label");
    label.className = "style";

    const name = document.createElement("div");
    name.className = "style-name";
    const radio = document.createElement("input");
    radio.type = "radio";
    radio.name = "style";
    radio.value = s.name;
    radio.checked = s.name.toLowerCase() === current;
    radio.dataset.broken = s.error ? "1" : "";
    radio.addEventListener("change", () => save({ style: s.name }));
    const actions = document.createElement("span");
    actions.className = "style-actions";
    if (s.custom) {
      actions.append(
        actionButton("Edit", () => openEditor(s.template, s.name)),
        actionButton("Share", (btn) =>
          openMenu(btn, [
            ["Copy as text", () => copyStyle(s.template, btn)],
            ["Save as file…", () => saveStyleFile(s.template, btn)],
          ]),
        ),
        actionButton("Delete", (btn) => {
          if (btn.dataset.armed) return deleteCustom(s.name);
          btn.dataset.armed = "1";
          btn.textContent = "Really delete?";
          setTimeout(() => {
            delete btn.dataset.armed;
            btn.textContent = "Delete";
          }, 3000);
        }),
      );
    } else {
      actions.append(actionButton("Duplicate", () => openEditor({ ...s.template, name: copyName(s.name) }, null)));
    }
    name.append(radio, s.name + (s.name === view.defaults.style ? " (default)" : ""), actions);
    label.append(name);

    if (s.error) {
      const err = document.createElement("p");
      err.className = "error small";
      err.textContent = s.error;
      label.append(err);
    }
    label.append(...previewRows(s.previews || []));
    (s.custom ? customs : builtins).append(label);
  }
  $("custom-empty").hidden = customs.childElementCount > 0;
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
  for (const r of document.querySelectorAll('input[name="style"]')) r.disabled = styleLocked || !!r.dataset.broken;
  const note = $("style-locked");
  note.hidden = !styleLocked;
  if (styleLocked) {
    note.textContent = `The style is set by ${view.envVars.style} (or --style) in the daemon's environment, which wins over this app. ` +
      (platform === "windows"
        ? "Remove it from your environment variables, then restart the daemon on the Daemon tab."
        : `Remove it from ${DROP_IN}, then run: systemctl --user daemon-reload && systemctl --user restart unbeatable-rpc`);
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

// saveFile writes the settings with change applied; it throws if it can't.
async function saveFile(change) {
  const next = { ...file, ...change };
  for (const k of Object.keys(next)) {
    if (next[k] === "" || next[k] === false || (Array.isArray(next[k]) && next[k].length === 0)) delete next[k];
  }
  await api().Save(next);
  file = next;
  const saved = $("saved");
  saved.hidden = false;
  clearTimeout(savedTimer);
  savedTimer = setTimeout(() => (saved.hidden = true), 1500);
}

async function save(change) {
  try {
    await saveFile(change);
  } catch (err) {
    alert("Couldn't save the settings: " + err);
  }
}

$("show-on-discord").addEventListener("change", (e) => save({ hidden: !e.target.checked }));

$("advanced").addEventListener("change", (e) => {
  const input = e.target;
  save({ [input.name]: input.type === "checkbox" ? input.checked : input.value.trim() });
});

/* custom styles */

const PARTS = [
  ["song", "Song", ""],
  ["results", "Results", "Empty: same as Song"],
  ["storySong", "Story song", "A song in story mode. Empty: same as Story"],
  ["story", "Story", "Empty: as the detailed style"],
  ["menus", "Menus", "Empty: as the detailed style"],
];

const LINES = [["details", "First line"], ["state", "Second line"]];

let editing = null; // the name of the custom style being edited, or "" for a new one
let lines = {}; // by "part.line": {text, pieces}; pieces is null for a line only text can show
let textMode = false; // edit every line as template text
let activeLine = "song.details"; // where a clicked piece in the palette goes
let selected = null; // {key, index}: the piece whose settings are open
let lastInput = null; // in text mode, the line a clicked placeholder goes into
let previewTimer = 0;

try {
  textMode = localStorage.getItem("textMode") === "1";
} catch {}

function customStyles() {
  return file.customStyles || [];
}

// freeName is name, or name with a number when a style already has it.
function freeName(name) {
  const taken = new Set(view.styles.map((s) => s.name.toLowerCase()));
  let free = name;
  for (let i = 2; taken.has(free.toLowerCase()); i++) free = `${name} ${i}`;
  return free;
}

function copyName(base) {
  return freeName(base + " copy");
}

/* sharing: a style is JSON text, to paste in a chat or save as a file */

function showCustomError(e) {
  const err = $("custom-error");
  err.textContent = e ? String(e) : "";
  err.hidden = !e;
}

function flash(btn, text) {
  const was = btn.textContent;
  btn.textContent = text;
  setTimeout(() => (btn.textContent = was), 1500);
}

async function copyStyle(style, btn) {
  showCustomError(null);
  try {
    await api().CopyText(await api().ExportStyle(style));
    flash(btn, "Copied");
  } catch (e) {
    showCustomError("Couldn't copy it: " + e + ". Save… puts it in a file instead.");
  }
}

async function saveStyleFile(style, btn) {
  showCustomError(null);
  try {
    if (await api().SaveStyleFile(style)) flash(btn, "Saved");
  } catch (e) {
    showCustomError(e);
  }
}

// openMenu shows a small menu under btn; any click or Escape closes it.
let closeMenu = null;

function openMenu(btn, items) {
  const wasOpen = closeMenu && closeMenu.btn === btn;
  if (closeMenu) closeMenu();
  if (wasOpen) return;
  const menu = el("div", "menu");
  menu.setAttribute("role", "menu");
  for (const [text, action] of items) {
    const item = el("button", "", text);
    item.type = "button";
    item.setAttribute("role", "menuitem");
    item.addEventListener("click", (e) => {
      e.preventDefault(); // inside the card's label
      closeMenu();
      action();
    });
    menu.append(item);
  }
  btn.after(menu);
  btn.setAttribute("aria-expanded", "true");
  const outside = (e) => {
    if (!menu.contains(e.target) && e.target !== btn) closeMenu();
  };
  const escape = (e) => {
    if (e.key === "Escape") closeMenu();
  };
  closeMenu = () => {
    menu.remove();
    btn.removeAttribute("aria-expanded");
    document.removeEventListener("pointerdown", outside, true);
    document.removeEventListener("keydown", escape);
    closeMenu = null;
  };
  closeMenu.btn = btn;
  document.addEventListener("pointerdown", outside, true);
  document.addEventListener("keydown", escape);
  menu.querySelector("button").focus();
}

function showImport(open) {
  $("import").hidden = !open;
  $("import-error").hidden = true;
  $("import-text").value = "";
  if (open) $("import-text").focus();
}

// imported opens a shared style in the editor, to look at before saving it.
function imported(style) {
  showImport(false);
  openEditor({ ...style, name: freeName(style.name || "imported") }, null);
  $("editor-title").textContent = "Import " + (style.name || "style");
}

async function importFrom(read) {
  const err = $("import-error");
  err.hidden = true;
  try {
    const style = await read();
    if (style) imported(style);
  } catch (e) {
    err.textContent = String(e);
    err.hidden = false;
  }
}

$("import-style").addEventListener("click", () => showImport($("import").hidden));
$("import-cancel").addEventListener("click", () => showImport(false));
$("import-paste").addEventListener("click", () => importFrom(() => api().ImportStyle($("import-text").value)));
$("import-file").addEventListener("click", () => importFrom(() => api().OpenStyleFile()));

function schedulePreview() {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(updatePreview, 150);
}

function el(tag, className, text) {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text !== undefined) e.textContent = text;
  return e;
}

function renderEditorParts() {
  const box = $("parts");
  box.replaceChildren();
  for (const [key, title, hint] of PARTS) {
    const part = el("details", "part");
    part.dataset.part = key;
    const summary = el("summary", "", title);
    if (hint) summary.append(" ", el("span", "dim small", hint));
    const body = el("div", "part-lines");
    for (const [line, label] of LINES) {
      const wrap = el("div", "line-wrap");
      wrap.dataset.key = `${key}.${line}`;
      wrap.append(el("div", "line-label", label), el("div", "line-body"));
      body.append(wrap);
    }
    const preview = el("div", "part-preview");
    preview.dataset.where = title;
    body.append(preview);
    part.append(summary, body);
    box.append(part);
  }
}

/* the editor: pieces */

function sync(key) {
  lines[key].text = piecesToTemplate(lines[key].pieces);
  schedulePreview();
}

function renderLine(key) {
  const body = document.querySelector(`.line-wrap[data-key="${key}"] .line-body`);
  const line = lines[key];
  if (textMode || !line.pieces) {
    const input = el("input", "template");
    input.value = line.text;
    input.spellcheck = false;
    input.autocomplete = "off";
    input.addEventListener("input", () => {
      line.text = input.value;
      schedulePreview();
    });
    input.addEventListener("focus", () => (lastInput = input));
    body.replaceChildren(input);
    if (!textMode) {
      const note = el("p", "dim small line-note", "This line uses brackets inside brackets, so it can only be edited as text. ");
      const restart = el("button", "quiet", "Start it again with pieces");
      restart.type = "button";
      restart.addEventListener("click", () => {
        line.pieces = [];
        sync(key);
        activeLine = key;
        renderLine(key);
      });
      note.append(restart);
      body.append(note);
    }
    return;
  }
  body.replaceChildren(pieceRow(key));
  if (selected && selected.key === key && line.pieces[selected.index]) body.append(pieceSettings(key, selected.index));
}

function renderRow(key) {
  const row = document.querySelector(`.line[data-key="${key}"]`);
  if (row) row.replaceWith(pieceRow(key));
}

function renderAllLines() {
  for (const key of Object.keys(lines)) renderLine(key);
  $("palette-text").hidden = textMode;
  $("help-pieces").hidden = textMode;
  $("help-text").hidden = !textMode;
  $("editor-text-mode").checked = textMode;
}

function pieceRow(key) {
  const row = el("div", "line");
  row.dataset.key = key;
  row.classList.toggle("active", key === activeLine);
  lines[key].pieces.forEach((p, i) => row.append(pieceChip(key, p, i)));
  if (!lines[key].pieces.length) row.append(el("span", "line-empty", "Empty. Drag pieces here."));
  row.addEventListener("click", (e) => {
    if (e.target === row || e.target.classList.contains("line-empty")) setActive(key);
  });
  return row;
}

function pieceChip(key, p, i) {
  const chip = el("button", "piece " + (p.field ? "field" : "text"));
  chip.type = "button";
  chip.classList.toggle("selected", !!selected && selected.key === key && selected.index === i);
  if (p.field) {
    if (p.before) chip.append(el("span", "piece-extra", p.before));
    chip.append(el("span", "piece-tag", p.field));
    if (p.after) chip.append(el("span", "piece-extra", p.after));
    if (p.fallback) chip.append(el("span", "piece-fallback", "or " + p.fallback));
  } else {
    chip.append(el("span", p.text ? "piece-extra" : "piece-fallback", p.text || "text"));
  }
  chip.title = "Drag to move, click to change";
  draggable(chip, { key, index: i }, () => select(key, i));
  return chip;
}

function setActive(key) {
  const old = activeLine;
  activeLine = key;
  renderRow(old);
  renderRow(key);
}

function select(key, index) {
  const old = selected;
  selected = old && old.key === key && old.index === index ? null : { key, index };
  activeLine = key;
  if (old && old.key !== key) renderLine(old.key);
  renderLine(key);
}

function pieceSettings(key, index) {
  const p = lines[key].pieces[index];
  const box = el("div", "piece-settings");
  const field = (label, prop, placeholder) => {
    const l = el("label", "", label);
    const input = el("input");
    input.value = p[prop];
    input.placeholder = placeholder || "";
    input.spellcheck = false;
    input.autocomplete = "off";
    input.addEventListener("input", () => {
      p[prop] = input.value;
      sync(key);
      renderRow(key);
    });
    input.addEventListener("keydown", (e) => {
      if (e.key !== "Enter") return;
      e.preventDefault(); // the form would save the style
      select(key, index);
    });
    l.append(input);
    return l;
  };

  if (p.field) {
    const l = el("label", "", "Placeholder");
    const pick = el("select");
    for (const name of view.placeholders) pick.append(new Option(name, name, false, name === p.field));
    pick.addEventListener("change", () => {
      p.field = pick.value;
      sync(key);
      renderLine(key);
    });
    l.append(pick);
    box.append(
      l,
      field("Text before", "before", "none"),
      field("Text after", "after", "none"),
      field(`If there's no ${p.field}, show`, "fallback", "nothing: the piece is left out"),
    );
  } else {
    box.append(field("Text", "text"));
  }

  const buttons = el("div", "piece-buttons");
  const button = (text, onClick, disabled) => {
    const b = el("button", "quiet", text);
    b.type = "button";
    b.disabled = !!disabled;
    b.addEventListener("click", onClick);
    buttons.append(b);
  };
  const pieces = lines[key].pieces;
  const move = (by) => () => {
    pieces.splice(index + by, 0, ...pieces.splice(index, 1));
    selected = { key, index: index + by };
    sync(key);
    renderLine(key);
  };
  button("Move left", move(-1), index === 0);
  button("Move right", move(1), index === pieces.length - 1);
  button("Remove", () => {
    pieces.splice(index, 1);
    selected = null;
    sync(key);
    renderLine(key);
  });
  button("Done", () => select(key, index));
  box.append(buttons);
  return box;
}

function newPiece(source, pieces, index) {
  if (!source.field) return { text: "" };
  // a space before it, unless it starts the line or follows one
  const prev = pieces[index - 1];
  const prevText = prev ? piecesToTemplate([prev]) : "";
  const before = index > 0 && !/\s\]?$/.test(prevText) ? " " : "";
  return { field: source.field, before, after: "", fallback: "" };
}

function addPiece(source, key, index) {
  const pieces = lines[key].pieces;
  pieces.splice(index, 0, source.key ? lines[source.key].pieces.splice(source.index, 1)[0] : newPiece(source, pieces, index));
  const old = selected;
  selected = { key, index };
  activeLine = key;
  sync(key);
  if (source.key && source.key !== key) sync(source.key);
  for (const k of new Set([key, source.key, old && old.key].filter(Boolean))) renderLine(k);
  $(`parts`).querySelector(`.line-wrap[data-key="${key}"]`).closest("details").open = true;
  const first = document.querySelector(".piece-settings input");
  if (first && !source.key && !source.field) first.focus();
}

function paletteClick(source) {
  if (textMode) {
    const input = lastInput || $("parts").querySelector("input.template");
    if (!input || !source.field) return;
    input.closest("details").open = true;
    input.focus();
    input.setRangeText(`{${source.field}}`, input.selectionStart, input.selectionEnd, "end");
    input.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }
  if (!lines[activeLine].pieces) return;
  addPiece(source, activeLine, lines[activeLine].pieces.length);
}

/* dragging pieces: pointer events, which work the same everywhere */

let drag = null;
let justDragged = false;

function draggable(chip, source, onClick) {
  chip.addEventListener("click", () => {
    if (!justDragged) onClick();
  });
  chip.addEventListener("pointerdown", (e) => {
    if (e.button !== 0 || textMode) return;
    drag = { source, chip, x: e.clientX, y: e.clientY, ghost: null, target: null };
    chip.setPointerCapture(e.pointerId);
  });
  chip.addEventListener("pointermove", (e) => {
    if (!drag || drag.chip !== chip) return;
    if (!drag.ghost) {
      if (Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < 5) return;
      drag.ghost = chip.cloneNode(true);
      drag.ghost.classList.add("ghost");
      document.body.append(drag.ghost);
      chip.classList.add("dragging");
    }
    drag.ghost.style.left = e.clientX + 6 + "px";
    drag.ghost.style.top = e.clientY + 6 + "px";
    drag.target = dropTarget(e.clientX, e.clientY);
    showDropTarget(drag.target);
  });
  chip.addEventListener("pointerup", () => {
    const d = drag;
    endDrag();
    if (!d || !d.ghost) return;
    // the click that follows a drag isn't one
    justDragged = true;
    setTimeout(() => (justDragged = false));
    if (d.target) {
      let { key, index } = d.target;
      if (d.source.key === key && d.source.index < index) index--;
      addPiece(d.source, key, index);
    }
  });
  chip.addEventListener("pointercancel", endDrag);
}

function endDrag() {
  if (!drag) return;
  if (drag.ghost) drag.ghost.remove();
  drag.chip.classList.remove("dragging");
  showDropTarget(null);
  drag = null;
}

function dropTarget(x, y) {
  const hit = document.elementFromPoint(x, y);
  const row = hit && hit.closest(".line");
  if (!row) return null;
  const chips = [...row.querySelectorAll(".piece")];
  let index = chips.length;
  for (let i = 0; i < chips.length; i++) {
    const r = chips[i].getBoundingClientRect();
    if (y < r.top || (y <= r.bottom && x < r.left + r.width / 2)) {
      index = i;
      break;
    }
  }
  return { key: row.dataset.key, index };
}

function showDropTarget(target) {
  for (const e of document.querySelectorAll(".drop-target, .drop-before, .drop-after")) {
    e.classList.remove("drop-target", "drop-before", "drop-after");
  }
  if (!target) return;
  const row = document.querySelector(`.line[data-key="${target.key}"]`);
  row.classList.add("drop-target");
  const chips = row.querySelectorAll(".piece");
  if (target.index < chips.length) chips[target.index].classList.add("drop-before");
  else if (chips.length) chips[chips.length - 1].classList.add("drop-after");
}

function renderPalette() {
  const box = $("chips");
  box.replaceChildren();
  const add = (source, text, className) => {
    const chip = el("button", "chip " + className, text);
    chip.type = "button";
    if (!source.field) chip.id = "palette-text";
    draggable(chip, source, () => paletteClick(source));
    box.append(chip);
  };
  add({}, "Text", "text");
  for (const name of view.placeholders) add({ field: name }, name, "field");
}

function openEditor(template, original) {
  editing = original || "";
  selected = null;
  lastInput = null;
  activeLine = "song.details";
  lines = {};
  $("editor-title").textContent = original ? "Edit " + original : "New style";
  $("editor-name").value = template.name || "";
  for (const part of $("parts").children) {
    const t = template[part.dataset.part] || {};
    for (const [line] of LINES) {
      const text = t[line] || "";
      lines[`${part.dataset.part}.${line}`] = { text, pieces: templateToPieces(text, view.placeholders) };
    }
    part.open = part.dataset.part === "song" || !!(t.details || t.state);
  }
  renderAllLines();
  $("editor-error").hidden = true;
  $("editor").hidden = false;
  $("editor").scrollIntoView({ block: "start" });
  $("editor-name").focus();
  updatePreview();
}

function closeEditor() {
  editing = null;
  $("editor").hidden = true;
}

function draft() {
  const d = { name: $("editor-name").value.trim() };
  for (const [part] of PARTS) {
    d[part] = {};
    for (const [line] of LINES) d[part][line] = lines[`${part}.${line}`].text;
  }
  return d;
}

function showEditorError(e) {
  const err = $("editor-error");
  err.textContent = e ? String(e) : "";
  err.hidden = !e;
}

async function updatePreview() {
  try {
    // each state's preview goes under its own lines; Go names them as PARTS does
    for (const p of await api().PreviewCustom(draft())) {
      const box = $("parts").querySelector(`.part-preview[data-where="${p.where}"]`);
      if (!box) continue;
      box.replaceChildren(
        el("div", "part-preview-label", "Preview"),
        el("div", "presence-line", p.details || " "),
        el("div", "presence-line", p.state || " "),
      );
    }
    showEditorError(null);
  } catch (e) {
    showEditorError(e);
  }
}

async function saveEditor() {
  const d = draft();
  const list = customStyles().slice();
  const i = editing ? list.findIndex((c) => c.name === editing) : -1;
  if (i >= 0) list[i] = d;
  else list.push(d);
  const change = { customStyles: list };
  // a new style is put to use; a renamed one stays in use
  const inUse = editing && (file.style || "").toLowerCase() === editing.toLowerCase();
  if (!editing || inUse) change.style = d.name;
  try {
    await api().CheckCustomStyles(list);
    await saveFile(change);
  } catch (e) {
    showEditorError(e);
    return;
  }
  closeEditor();
  await reloadFile();
}

async function deleteCustom(name) {
  const change = { customStyles: customStyles().filter((c) => c.name !== name) };
  if ((file.style || "").toLowerCase() === name.toLowerCase()) change.style = "";
  if (editing === name) closeEditor();
  await save(change);
  await reloadFile();
}

$("editor-name").addEventListener("input", schedulePreview);
$("editor-text-mode").addEventListener("change", (e) => {
  textMode = e.target.checked;
  try {
    localStorage.setItem("textMode", textMode ? "1" : "");
  } catch {}
  // back to pieces: lines changed as text are read again
  if (!textMode) for (const line of Object.values(lines)) line.pieces = templateToPieces(line.text, view.placeholders);
  selected = null;
  renderAllLines();
});
$("editor-form").addEventListener("submit", (e) => {
  e.preventDefault();
  saveEditor();
});
$("editor-save").addEventListener("click", saveEditor);
$("editor-cancel").addEventListener("click", closeEditor);
$("new-style").addEventListener("click", () => openEditor({ name: "", song: { details: "{title}[ - {artist}]" } }, null));

/* tabs */

function showTab(name) {
  for (const tab of document.querySelectorAll(".tab")) {
    tab.setAttribute("aria-selected", String(tab.dataset.tab === name));
  }
  $("tab-status").hidden = name !== "status";
  $("tab-styles").hidden = name !== "styles";
  $("tab-daemon").hidden = name !== "daemon";
  if (name === "daemon") refreshLog();
}

for (const tab of document.querySelectorAll(".tab")) {
  tab.addEventListener("click", () => showTab(tab.dataset.tab));
}

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
      lines.push(platform === "windows"
        ? "Can't reach Discord. Check the Discord app is running, and turn on " +
          "User Settings → Activity Privacy → Share your detected activities."
        : "Can't reach Discord. In Vesktop, turn on Settings → Rich Presence (arRPC). " +
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

/* daemon tab: log, service controls */

// journal (short-iso): "<time> <host> <unit>[<pid>]: <message>"
const JOURNAL_RE = /^(\d{4}-\d\d-\d\d)T(\d\d:\d\d:\d\d)\S* \S+ ([^\[:]+)(?:\[\d+\])?: (.*)$/;
// Go's log prefix, in the journal and the launch wrapper's file: "unbeatable-rpc: 2026/10/14 16:01:27 <message>"
const GOLOG_RE = /^unbeatable-rpc: (\d{4})\/(\d\d)\/(\d\d) (\d\d:\d\d:\d\d) (.*)$/;
const LOG_LEVELS = [
  ["bad", /fail|cannot|lost|error|ignoring|not reloaded/i],
  ["warn", /stopp|game closed|no heartbeat|cleared/i],
  ["good", /activity set|connected to|game is running|^started|listening on/i],
];

function span(cls, text) {
  const el = document.createElement("span");
  el.className = cls;
  el.textContent = text;
  return el;
}

// logLine turns one raw line into a row: time, source, then the message,
// with its "key:" prefix dimmed and the whole thing coloured by what it means.
function logLine(raw, prev) {
  let date = "", time = "", source = "", msg = raw;
  let m = JOURNAL_RE.exec(raw);
  if (m) [, date, time, source, msg] = m;
  const g = GOLOG_RE.exec(msg);
  if (g) {
    date = date || `${g[1]}-${g[2]}-${g[3]}`;
    time = time || g[4];
    source = "daemon";
    msg = g[5];
  } else if (source === "unbeatable-rpc") {
    source = "daemon";
  }

  const rows = [];
  if (date && date !== prev.date) {
    rows.push(span("log-date", date));
    prev.date = date;
  }
  const row = document.createElement("div");
  row.className = "log-row";
  if (time) row.append(span("log-time", time));
  if (source) row.append(span("log-source " + source, source));
  const level = (LOG_LEVELS.find(([, re]) => re.test(msg)) || [""])[0];
  const text = span("log-msg " + level, "");
  const key = /^([a-zA-Z_ ]+): (.*)$/.exec(msg);
  if (key) text.append(span("log-key", key[1] + ": "), key[2]);
  else text.textContent = msg;
  row.append(text);
  rows.push(row);
  return rows;
}

let logText = null;

async function refreshLog() {
  if ($("tab-daemon").hidden) return;
  const box = $("daemon-log");
  const text = await api().DaemonLog();
  $("log-empty").hidden = text !== "";
  box.hidden = text === "";
  if (text === logText) return;
  logText = text;
  // stay at the bottom for new lines, unless scrolled up to read
  const pinned = box.scrollHeight - box.scrollTop - box.clientHeight < 20;
  const prev = { date: "" };
  box.replaceChildren(...text.split("\n").flatMap((l) => logLine(l, prev)));
  if (pinned) box.scrollTop = box.scrollHeight;
}

$("copy-log").addEventListener("click", async () => {
  const btn = $("copy-log");
  showServiceError(null);
  try {
    await api().CopyText(logText || "");
    btn.textContent = "Copied";
    setTimeout(() => (btn.textContent = "Copy"), 1500);
  } catch (e) {
    showServiceError(e);
  }
});

/* advanced: mod settings, versions */

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
  // always Status first: it's where problems show
  showTab("status");
  view = await api().Load();
  file = { ...view.file };
  renderEditorParts();
  renderPalette();
  renderStyles();
  renderAdvanced();
  renderLocations();
  appVersion = await api().AppVersion();
  platform = await api().Platform();
  renderPlatform();
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
  setInterval(refreshLog, 2000);
  setInterval(tickTimer, 1000);
})();
