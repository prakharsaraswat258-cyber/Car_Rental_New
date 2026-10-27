// Motion helpers. Only transform and opacity ever animate. With
// prefers-reduced-motion every movement becomes a 120 ms fade.

export const EASE = "cubic-bezier(0.2, 0, 0, 1)";
export const ENTER = "cubic-bezier(0.05, 0.7, 0.1, 1)";
export const DUR = { fast: 180, base: 260, slow: 400, hero: 600 };
export const STAGGER = 30;

const rm = matchMedia("(prefers-reduced-motion: reduce)");
export const reduced = () => rm.matches;

// play runs WAAPI keyframes and resolves when done (or swapped for a fade).
export function play(el, keyframes, opts = {}) {
  if (!el || !el.animate) return Promise.resolve();
  if (reduced()) {
    const last = keyframes[keyframes.length - 1] || {};
    if (last.opacity === 0) {
      return el.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 120, fill: opts.fill || "none" }).finished.catch(() => {});
    }
    return el.animate([{ opacity: 0.4 }, { opacity: 1 }], { duration: 120 }).finished.catch(() => {});
  }
  return el.animate(keyframes, { duration: DUR.base, easing: EASE, fill: "none", ...opts }).finished.catch(() => {});
}

// --- on-demand vendor scripts (performance budget: Leaflet only on map
// pages, Motion only where a spring runs) ---------------------------------
const loaded = new Map();

export function loadScript(url) {
  if (!url) return Promise.reject(new Error("no url"));
  if (!loaded.has(url)) {
    loaded.set(url, new Promise((resolve, reject) => {
      const s = document.createElement("script");
      s.src = url;
      s.async = true;
      s.onload = resolve;
      s.onerror = () => { loaded.delete(url); reject(new Error(`failed to load ${url}`)); };
      document.head.appendChild(s);
    }));
  }
  return loaded.get(url);
}

export function loadCSS(url) {
  if (!url || document.querySelector(`link[href="${url}"]`)) return Promise.resolve();
  return new Promise((resolve) => {
    const l = document.createElement("link");
    l.rel = "stylesheet";
    l.href = url;
    l.onload = resolve;
    l.onerror = resolve;
    document.head.appendChild(l);
  });
}

export function motion() {
  if (window.Motion) return Promise.resolve(window.Motion);
  return loadScript(document.body.dataset.motionJs).then(() => window.Motion);
}

// spring animates a number with Motion's spring (stiffness 260, damping 30)
// and calls onUpdate each frame. Motion is fetched on first use; until it
// arrives (or if it can't), the value jumps to the end.
export async function spring(from, to, onUpdate, opts = {}) {
  if (reduced()) {
    onUpdate(to);
    return;
  }
  let M = window.Motion;
  if (!M) {
    try { M = await motion(); } catch { M = null; }
  }
  if (!M || typeof M.animate !== "function") {
    onUpdate(to);
    return;
  }
  return new Promise((resolve) => {
    try {
      M.animate(from, to, {
        type: "spring", stiffness: 260, damping: 30, ...opts,
        onUpdate, onComplete: () => { onUpdate(to); resolve(); },
      });
    } catch {
      onUpdate(to);
      resolve();
    }
  });
}

// bump gives an element a springy scale pop (counters, badges).
export function bump(el, scale = 1.35) {
  if (!el) return Promise.resolve();
  return spring(scale, 1, (v) => { el.style.transform = `scale(${v})`; }, { damping: 12 })
    .then(() => { el.style.transform = ""; });
}

export function vibrate(ms = 15) {
  try { navigator.vibrate?.(ms); } catch { /* not supported */ }
}

// icon builds a sprite <svg> the same way templates do.
export function icon(name, cls = "") {
  const sprite = document.body.dataset.sprite || "/static/img/icons.svg";
  return `<svg class="icon ${cls}" aria-hidden="true" focusable="false"><use href="${sprite}#${name}"></use></svg>`;
}

export function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

// all finds matches inside root, including root itself (htmx swaps hand us
// the new element).
export function all(root, sel) {
  const out = root.matches && root.matches(sel) ? [root] : [];
  return out.concat(Array.from(root.querySelectorAll(sel)));
}

// once marks an element as initialised and reports whether it was new.
export function once(el, key = "ready") {
  if (el.dataset[key]) return false;
  el.dataset[key] = "1";
  return true;
}

// fetchHTML posts/gets with the headers our handlers look for.
export async function request(url, { method = "GET", body, accept = "text/html" } = {}) {
  document.dispatchEvent(new CustomEvent("saarthi:busy"));
  try {
    const res = await fetch(url, {
      method, body,
      headers: { "X-Requested-With": "fetch", Accept: accept },
      credentials: "same-origin",
    });
    return res;
  } finally {
    document.dispatchEvent(new CustomEvent("saarthi:idle"));
  }
}

// swapHTML replaces el with markup and re-runs component init on it.
export function swapHTML(el, html) {
  const tpl = document.createElement("template");
  tpl.innerHTML = html.trim();
  const fresh = tpl.content.firstElementChild;
  if (!fresh) { el.remove(); return null; }
  el.replaceWith(fresh);
  if (window.htmx) window.htmx.process(fresh);
  document.dispatchEvent(new CustomEvent("saarthi:init", { detail: fresh }));
  return fresh;
}
