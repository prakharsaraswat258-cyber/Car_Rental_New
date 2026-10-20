// Small behaviours: gallery thumbs, mobile map toggle, show/hide toggles,
// htmx swap-out fade for removed cards.
import { all, once, play, DUR, ENTER } from "./motion.js";
import { refreshMaps } from "./map.js";

export function initMisc(root) {
  for (const g of all(root, "[data-gallery]")) {
    if (!once(g)) continue;
    const hero = g.querySelector("[data-gallery-hero]");
    g.addEventListener("click", (e) => {
      const b = e.target.closest("[data-gallery-src]");
      if (!b || !hero) return;
      const img = hero.querySelector("img");
      if (img) {
        img.src = b.dataset.gallerySrc;
        play(img, [{ opacity: 0.3 }, { opacity: 1 }], { duration: DUR.base });
      }
      g.querySelectorAll("[data-gallery-src]").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
    });
  }

  for (const b of all(root, "[data-map-toggle]")) {
    if (!once(b)) continue;
    b.addEventListener("click", () => {
      const on = document.body.classList.toggle("show-map");
      b.querySelector("[data-map-toggle-label]").textContent = on ? "List" : "Map";
      b.setAttribute("aria-expanded", String(on));
      if (on) {
        // the map was hidden: let Leaflet measure it now
        document.querySelectorAll("[data-map]").forEach((m) => { if (!m._map) { delete m.dataset.mapReady; } });
        document.dispatchEvent(new CustomEvent("saarthi:init", { detail: document.getElementById("map-wrap") }));
        requestAnimationFrame(refreshMaps);
      }
    });
  }

  for (const b of all(root, "[data-toggle]")) {
    if (!once(b, "toggleReady")) continue;
    b.addEventListener("click", () => {
      const t = document.getElementById(b.dataset.toggle);
      if (!t) return;
      t.hidden = !t.hidden;
      b.setAttribute("aria-expanded", String(!t.hidden));
      if (!t.hidden) {
        play(t, [{ opacity: 0, transform: "translateY(-6px)" }, { opacity: 1, transform: "none" }], { duration: DUR.base, easing: ENTER });
        document.dispatchEvent(new CustomEvent("saarthi:init", { detail: t }));
      }
    });
  }

  if (!window.__miscWired) {
    window.__miscWired = true;
    // htmx "outerHTML swap:260ms" leaves time for this fade-out
    document.addEventListener("htmx:beforeSwap", (e) => {
      const t = e.detail.target;
      if (t?.classList?.contains("req-card") && !e.detail.serverResponse.trim()) t.classList.add("is-leaving");
    });
    document.addEventListener("htmx:afterSwap", (e) => {
      if (e.target.id === "results") {
        e.target.querySelectorAll("[data-stagger]").forEach((s) => { s.style.animation = "none"; void s.offsetWidth; s.style.animation = ""; });
      }
    });
  }
}
