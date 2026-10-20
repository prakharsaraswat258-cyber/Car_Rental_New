// Bottom sheets and dialogs on native <dialog>: focus trap and Esc for
// free. They animate in (transform/opacity), close on backdrop tap, and
// sheets drag to dismiss from the handle.
import { all, once, reduced, spring } from "./motion.js";

function anyOpen() {
  return document.querySelector("dialog[open]") !== null;
}

export function openSheet(d) {
  if (!d) return;
  if (!d.open) d.showModal();
  document.body.classList.add("has-modal");
  requestAnimationFrame(() => requestAnimationFrame(() => d.classList.add("is-shown")));
  const focus = d.querySelector("[autofocus], .sheet-body input:not([type=hidden]), .sheet-body button, .dialog-panel button");
  focus?.focus({ preventScroll: true });
}

export function closeSheet(d) {
  if (!d || !d.open || d.dataset.closing) return;
  d.dataset.closing = "1";
  d.classList.remove("is-shown");
  const panel = d.querySelector(".sheet-panel, .dialog-panel");
  if (panel) panel.style.transform = "";
  const done = () => {
    delete d.dataset.closing;
    d.classList.remove("is-dragging");
    if (d.open) d.close();
    if (!anyOpen()) document.body.classList.remove("has-modal");
    if (d.dataset.ephemeral) d.remove();
  };
  setTimeout(done, reduced() ? 130 : 320);
}

function wire(d) {
  if (!once(d, "sheetReady")) return;
  d.addEventListener("click", (e) => {
    if (e.target === d) closeSheet(d); // backdrop
    if (e.target.closest("[data-sheet-close]")) closeSheet(d);
  });
  d.addEventListener("cancel", (e) => { e.preventDefault(); closeSheet(d); });

  const handle = d.querySelector("[data-sheet-handle]");
  const panel = d.querySelector(".sheet-panel");
  if (!handle || !panel) return;
  let startY = 0, dy = 0, t0 = 0, dragging = false;
  handle.addEventListener("pointerdown", (e) => {
    dragging = true; startY = e.clientY; dy = 0; t0 = performance.now();
    handle.setPointerCapture(e.pointerId);
    d.classList.add("is-dragging");
  });
  handle.addEventListener("pointermove", (e) => {
    if (!dragging) return;
    dy = Math.max(0, e.clientY - startY);
    panel.style.transform = `translateY(${dy}px)`;
  });
  const end = () => {
    if (!dragging) return;
    dragging = false;
    d.classList.remove("is-dragging");
    const v = dy / Math.max(1, performance.now() - t0);
    if (dy > 120 || v > 0.6) {
      panel.style.transform = "";
      closeSheet(d);
    } else {
      spring(dy, 0, (y) => { panel.style.transform = `translateY(${y}px)`; }).then(() => { panel.style.transform = ""; });
    }
  };
  handle.addEventListener("pointerup", end);
  handle.addEventListener("pointercancel", end);
}

export function initSheets(root) {
  for (const d of all(root, "dialog[data-sheet]")) {
    wire(d);
    if (d.hasAttribute("data-autoopen") && once(d, "autoOpened")) {
      // a fresh sheet from htmx replaces any open one with the same id
      document.querySelectorAll(`dialog[open]`).forEach((o) => { if (o !== d && o.id === d.id) o.remove(); });
      d.dataset.ephemeral = d.closest("#sheet-root") ? "1" : "";
      openSheet(d);
    }
  }
  for (const btn of all(root, "[data-sheet-open]")) {
    if (!once(btn, "sheetBtn")) continue;
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      const d = document.getElementById(btn.dataset.sheetOpen);
      if (!d) return;
      if (btn.dataset.next !== undefined) {
        d.querySelectorAll("input[name=next]").forEach((i) => {
          // demo provider/admin logins keep their own landing page
          if (!i.closest("form[action^='/login/demo/provider'], form[action^='/login/demo/admin']")) i.value = btn.dataset.next;
        });
      }
      openSheet(d);
    });
  }
}
