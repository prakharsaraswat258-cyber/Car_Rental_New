// Small behaviours: gallery-free bits that every page shares.
//  - sticky header border appears only after scrolling
//  - images fade in on load (sized boxes, so no layout shift)
//  - back from a vehicle restores the results scroll position
//  - skeletons: nothing under 300 ms; once shown, at least 400 ms
//  - optimistic filter-chip removal; composer clear button
//  - "How pickup works" dismissal; Retry / Try again buttons
import { all, once, play, DUR, ENTER } from "./motion.js";
import { refreshMaps } from "./map.js";

document.documentElement.classList.add("js");

function fadeImages(root) {
  for (const img of all(root, "img.fade-img")) {
    if (img.complete && img.naturalWidth) img.classList.add("is-loaded");
    else if (once(img, "fadeReady")) {
      img.addEventListener("load", () => img.classList.add("is-loaded"), { once: true });
      img.addEventListener("error", () => img.classList.add("is-loaded"), { once: true });
    }
  }
}

const SCROLL_KEY = "scroll:";

function skeletonTiming() {
  const results = () => document.getElementById("results");
  let timer = 0, shownAt = 0;
  document.addEventListener("htmx:beforeRequest", (e) => {
    const r = results();
    if (!r || (e.detail.target !== r && e.detail.target?.id !== "results")) return;
    r.setAttribute("aria-busy", "true");
    clearTimeout(timer);
    timer = setTimeout(() => { r.classList.add("is-waiting"); shownAt = performance.now(); }, 300);
  });
  document.addEventListener("htmx:afterSettle", (e) => {
    const r = results();
    if (!r || e.detail.target !== r) return;
    clearTimeout(timer);
    const left = shownAt ? 400 - (performance.now() - shownAt) : 0;
    setTimeout(() => {
      r.classList.remove("is-waiting");
      r.setAttribute("aria-busy", "false");
      shownAt = 0;
    }, Math.max(0, left));
  });
  document.addEventListener("htmx:responseError", () => {
    clearTimeout(timer);
    results()?.classList.remove("is-waiting");
  });
}

export function initMisc(root) {
  fadeImages(root);

  for (const b of all(root, "[data-map-toggle]")) {
    if (!once(b)) continue;
    b.addEventListener("click", () => {
      const on = document.body.classList.toggle("show-map");
      b.querySelector("[data-map-toggle-label]").textContent = on ? "List" : "Map";
      b.setAttribute("aria-expanded", String(on));
      if (on) {
        document.querySelectorAll("[data-map]").forEach((m) => { if (!m._map) delete m.dataset.mapReady; });
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

  // optimistic chip removal: the chip leaves now, htmx brings the results
  for (const x of all(root, "[data-chip-remove]")) {
    if (!once(x, "chipReady")) continue;
    x.addEventListener("click", () => {
      const chip = x.closest("[data-chip]");
      if (chip) play(chip, [{ opacity: 1 }, { opacity: 0, transform: "scale(0.9)" }], { duration: DUR.fast, fill: "forwards" });
    });
  }

  for (const form of all(root, "[data-composer]")) {
    const input = form.querySelector("input[name=q]");
    const clear = form.querySelector("[data-composer-clear]");
    if (!clear || !once(clear, "clearReady")) continue;
    const sync = () => { clear.hidden = !input.value; };
    input.addEventListener("input", sync);
    clear.addEventListener("click", () => { input.value = ""; sync(); input.focus(); });
    sync();
  }

  for (const b of all(root, "[data-tips-dismiss]")) {
    if (!once(b, "tipsReady")) continue;
    b.addEventListener("click", () => {
      document.cookie = "pickup_tips=hidden; Path=/; Max-Age=31536000; SameSite=Lax";
      const card = b.closest("[data-tips]");
      play(card, [{ opacity: 1 }, { opacity: 0, transform: "translateY(-8px)" }], { duration: DUR.base, fill: "forwards" }).then(() => card.remove());
    });
  }

  for (const b of all(root, "[data-reload]")) {
    if (once(b, "reloadReady")) b.addEventListener("click", () => location.reload());
  }

  if (window.__miscWired) return;
  window.__miscWired = true;

  // sticky header border after scrolling
  const sticky = document.querySelectorAll("[data-sticky]");
  const onScroll = () => sticky.forEach((h) => h.classList.toggle("is-scrolled", window.scrollY > 4));
  window.addEventListener("scroll", onScroll, { passive: true });
  onScroll();

  // keep the results scroll position for Back
  if (document.querySelector("[data-scroll-keep]")) {
    const key = SCROLL_KEY + location.pathname + location.search;
    window.addEventListener("pagehide", () => { try { sessionStorage.setItem(key, String(window.scrollY)); } catch { /* ignore */ } });
    const nav = performance.getEntriesByType?.("navigation")?.[0];
    if (nav?.type === "back_forward") {
      try {
        const y = Number(sessionStorage.getItem(key));
        if (y > 0) requestAnimationFrame(() => window.scrollTo(0, y));
      } catch { /* ignore */ }
    }
  }

  skeletonTiming();

  // htmx "outerHTML swap:260ms" leaves time for this fade-out
  document.addEventListener("htmx:beforeSwap", (e) => {
    const t = e.detail.target;
    if (t?.classList?.contains("req-card") && !e.detail.serverResponse.trim()) t.classList.add("is-leaving");
  });
}
