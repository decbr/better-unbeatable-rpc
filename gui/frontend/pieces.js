"use strict";

function escapeTemplate(s) {
  return s.replace(/[\\[\]{}|]/g, "\\$&");
}

function piecesToTemplate(pieces) {
  return pieces
    .map((p) => {
      if (p.field === undefined) return escapeTemplate(p.text);
      if (!p.before && !p.after && !p.fallback) return `{${p.field}}`;
      const fallback = p.fallback ? "|" + escapeTemplate(p.fallback) : "";
      return `[${escapeTemplate(p.before)}{${p.field}}${escapeTemplate(p.after)}${fallback}]`;
    })
    .join("");
}

function templateToPieces(src, placeholders) {
  let i = 0;

  // text reads literal text up to one of stops, undoing escapes
  const text = (stops) => {
    let out = "";
    while (i < src.length) {
      const c = src[i];
      if (c === "\\") {
        if (i + 1 >= src.length) return null;
        out += src[i + 1];
        i += 2;
      } else if (stops.includes(c)) {
        break;
      } else {
        out += c;
        i++;
      }
    }
    return out;
  };
  const field = () => {
    const end = src.indexOf("}", i);
    if (src[i] !== "{" || end < 0) return null;
    const name = src.slice(i + 1, end);
    if (!placeholders.includes(name)) return null;
    i = end + 1;
    return name;
  };

  const pieces = [];
  while (i < src.length) {
    const t = text("{}[]");
    if (t === null) return null;
    if (t) pieces.push({ text: t });
    if (i >= src.length) break;

    if (src[i] === "{") {
      const name = field();
      if (name === null) return null;
      pieces.push({ field: name, before: "", after: "", fallback: "" });
    } else if (src[i] === "[") {
      i++;
      const before = text("{}[]|");
      const name = before === null ? null : field();
      const after = name === null ? null : text("{}[]|");
      if (after === null) return null;
      let fallback = "";
      if (src[i] === "|") {
        i++;
        fallback = text("{}[]|");
        if (fallback === null) return null;
      }
      if (src[i] !== "]") return null;
      i++;
      pieces.push({ field: name, before, after, fallback });
    } else {
      return null;
    }
  }
  return pieces;
}

if (typeof module !== "undefined") module.exports = { piecesToTemplate, templateToPieces, escapeTemplate };
