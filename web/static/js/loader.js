// Wheel loader: any wait longer than 300 ms shows the turning wheel with a
// dashed road under it. Hooks htmx requests and our own fetches. Buttons in
// forms marked data-submit-loading swap their label for the wheel on submit.
import { all, once } from "./motion.js";

let pending = 0;
let timer;
let wired = false;

function el() { return document.getElementById("loader"); }

export function busy() {
  pending++;
  clearTimeout(timer);
  timer = setTimeout(() => el()?.classList.add("is-on"), 300);
}

export function idle() {
  pending = Math.max(0, pending - 1);
  if (pending === 0) {
    clearTimeout(timer);
    el()?.classList.remove("is-on");
  }
}

export function initLoader(root) {
  if (!wired) {
    wired = true;
    document.addEventListener("htmx:beforeRequest", busy);
    document.addEventListener("htmx:afterRequest", idle);
    document.addEventListener("saarthi:busy", busy);
    document.addEventListener("saarthi:idle", idle);
    window.addEventListener("pageshow", () => {
      pending = 0;
      idle();
      document.querySelectorAll(".btn.is-loading").forEach((b) => b.classList.remove("is-loading"));
    });
  }
  for (const form of all(root, "form[data-submit-loading]")) {
    if (!once(form, "loadingReady")) continue;
    form.addEventListener("submit", (e) => {
      if (e.defaultPrevented || !form.checkValidity()) return;
      const btn = e.submitter || form.querySelector("button[type=submit], button:not([type])");
      btn?.classList.add("is-loading");
    });
  }
  for (const btn of all(root, "[data-submit-loading-btn]")) {
    if (!once(btn, "loadingReady")) continue;
    const form = btn.form;
    form?.addEventListener("submit", (e) => { if (e.submitter === btn) btn.classList.add("is-loading"); });
  }
}
