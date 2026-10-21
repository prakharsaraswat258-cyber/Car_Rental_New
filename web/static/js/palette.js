// Command palette (Ctrl/Cmd K) and keyboard shortcuts for the provider and
// admin consoles. Results come from an htmx partial; this module only
// handles opening, arrow-key selection and the shortcut keys.
import { all, once } from "./motion.js";
import { openSheet, closeSheet } from "./sheet.js";

function typing(e) {
  const t = e.target;
  return t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName);
}

function items(dialog) { return [...dialog.querySelectorAll('#palette-list [role="option"]')]; }

function select(dialog, i) {
  const list = items(dialog);
  if (!list.length) return;
  i = (i + list.length) % list.length;
  list.forEach((li, k) => li.setAttribute("aria-selected", String(k === i)));
  dialog.querySelector("[data-palette-input]").setAttribute("aria-activedescendant", list[i].id);
  list[i].scrollIntoView({ block: "nearest" });
}

function current(dialog) {
  const list = items(dialog);
  return Math.max(0, list.findIndex((li) => li.getAttribute("aria-selected") === "true"));
}

export function openPalette() {
  const dialog = document.getElementById("palette");
  if (!dialog) return;
  const input = dialog.querySelector("[data-palette-input]");
  openSheet(dialog);
  input.value = "";
  input.focus();
  window.htmx?.trigger(input, "palette:open");
}

export function initPalette(root) {
  for (const dialog of all(root, "[data-palette]")) {
    if (!once(dialog, "paletteReady")) continue;
    const input = dialog.querySelector("[data-palette-input]");
    input.addEventListener("keydown", (e) => {
      if (e.key === "ArrowDown") { e.preventDefault(); select(dialog, current(dialog) + 1); }
      if (e.key === "ArrowUp") { e.preventDefault(); select(dialog, current(dialog) - 1); }
      if (e.key === "Enter") {
        e.preventDefault();
        const a = items(dialog)[current(dialog)]?.querySelector("a");
        if (a) location.assign(a.href);
      }
    });
    dialog.addEventListener("htmx:afterSwap", () => select(dialog, 0));
  }
  for (const b of all(root, "[data-palette-open]")) {
    if (!once(b, "paletteBtn")) continue;
    b.addEventListener("click", openPalette);
  }
  if (window.__shortcuts || !document.getElementById("palette")) return;
  window.__shortcuts = true;
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      openPalette();
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey || typing(e) || document.querySelector("dialog[open]")) return;
    const admin = document.querySelector('#palette input[hx-get="/admin/palette"]');
    switch (e.key) {
      case "/":
        e.preventDefault();
        openPalette();
        break;
      case "?":
        e.preventDefault();
        openSheet(document.getElementById("shortcuts"));
        break;
      case "n":
      case "N":
        if (!admin) { e.preventDefault(); location.assign("/p/vehicles/new"); }
        break;
      case "b":
      case "B":
        if (admin) break;
        e.preventDefault();
        if (location.pathname === "/p/calendar") document.querySelector(".pcal-cell")?.focus();
        else location.assign("/p/calendar#block");
        break;
    }
  });
  document.addEventListener("click", (e) => {
    const a = e.target.closest("#palette-list a");
    if (a) closeSheet(document.getElementById("palette"));
  });
}
