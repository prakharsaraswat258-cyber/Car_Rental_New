// Toasts slide up bottom-centre, optionally with Undo. Sources:
//  - htmx responses with HX-Trigger: {"toast": {...}}
//  - a flash cookie after a redirect (rendered into #toasts[data-flash])
//  - toast({...}) from other modules
import { esc, icon, request } from "./motion.js";

const ICONS = { good: "circle-check", bad: "circle-alert", warn: "triangle-alert", info: "info" };

export function toast({ message, tone = "info", undo = null, timeout } = {}) {
  const host = document.getElementById("toasts");
  if (!host || !message) return;
  const el = document.createElement("div");
  el.className = `toast tone-${tone}`;
  el.setAttribute("role", tone === "bad" ? "alert" : "status");
  el.innerHTML = `${icon(ICONS[tone] || "info")}<span class="toast-msg">${esc(message)}</span>` +
    (undo ? `<button class="toast-undo" type="button">Undo</button>` : "") +
    `<button class="icon-btn" type="button" aria-label="Dismiss">${icon("x", "sm")}</button>`;
  host.appendChild(el);
  while (host.children.length > 3) host.firstElementChild.remove();

  let timer;
  const close = () => {
    clearTimeout(timer);
    el.classList.add("is-leaving");
    el.addEventListener("animationend", () => el.remove(), { once: true });
    setTimeout(() => el.remove(), 400);
  };
  el.querySelector(".icon-btn").addEventListener("click", close);
  if (undo) {
    el.querySelector(".toast-undo").addEventListener("click", async () => {
      close();
      const body = new URLSearchParams(undo.body || {});
      const res = await request(undo.url, { method: undo.method || "POST", body });
      if (res.ok) {
        document.dispatchEvent(new CustomEvent("saarthi:undone", { detail: undo }));
        if (undo.reload !== false && !document.querySelector("[data-pcal]")) location.reload();
      } else {
        toast({ message: "That couldn't be undone. Refresh and try again.", tone: "bad" });
      }
    });
  }
  timer = setTimeout(close, timeout || (undo ? 7000 : tone === "bad" ? 6000 : 4000));
  el.addEventListener("mouseenter", () => clearTimeout(timer));
  el.addEventListener("mouseleave", () => { timer = setTimeout(close, 2500); });
}

let wired = false;
export function initToasts() {
  if (!wired) {
    wired = true;
    document.addEventListener("toast", (e) => toast(e.detail || {}));
    // htmx error responses: show something calm instead of nothing
    document.addEventListener("htmx:responseError", (e) => {
      const xhr = e.detail.xhr;
      if (xhr && xhr.getResponseHeader("HX-Trigger")) return;
      const text = (xhr && xhr.status === 422 && xhr.responseText) || "Something went wrong on our side. Please try again.";
      toast({ message: text.slice(0, 200), tone: "bad" });
    });
    document.addEventListener("htmx:sendError", () => toast({ message: "You seem to be offline. Check the connection and try again.", tone: "bad" }));
  }
  const host = document.getElementById("toasts");
  if (host && host.dataset.flash && !host.dataset.flashShown) {
    host.dataset.flashShown = "1";
    try { toast(JSON.parse(host.dataset.flash)); } catch { /* ignore bad flash */ }
  }
}
