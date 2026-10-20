// Provider vehicle editor: 3 steps (Basics, Photos, Pricing) on one page,
// a live card preview, and the publish gate (name, prices, 1 photo).
import { all, once, play, DUR, ENTER } from "./motion.js";

const rupee = (v) => {
  const n = Math.round(Number(v) || 0);
  const s = String(n);
  if (s.length <= 3) return `₹${s}`;
  const head = s.slice(0, -3).replace(/\B(?=(\d{2})+(?!\d))/g, ",");
  return `₹${head},${s.slice(-3)}`;
};

function artFor(type, seats) {
  if (type === "ev") return Number(seats) >= 4 ? "ev-car" : "ev-scooter";
  return type || "scooter";
}

export function initEditor(root) {
  for (const ed of all(root, "[data-editor]")) {
    if (!once(ed)) continue;
    const form = document.getElementById("vehicle-form");
    const steps = document.querySelector("[data-steps]");
    const stepInput = form?.querySelector("[data-step-input]");

    const show = (name) => {
      document.querySelectorAll("[data-step-panel]").forEach((p) => {
        const on = p.dataset.stepPanel === name;
        p.hidden = !on;
        if (on) play(p, [{ opacity: 0, transform: "translateY(8px)" }, { opacity: 1, transform: "none" }], { duration: DUR.base, easing: ENTER });
      });
      steps?.querySelectorAll("[data-step-btn]").forEach((b) => b.setAttribute("aria-selected", String(b.dataset.stepBtn === name)));
      if (stepInput) stepInput.value = name;
      const url = new URL(location.href);
      url.searchParams.set("step", name);
      history.replaceState(null, "", url);
    };
    steps?.addEventListener("click", (e) => {
      const b = e.target.closest("[data-step-btn]");
      if (b && !b.disabled) show(b.dataset.stepBtn);
    });
    document.addEventListener("click", (e) => {
      const n = e.target.closest("[data-step-next]");
      if (n) show(n.dataset.stepNext);
    });

    const card = ed.querySelector("[data-preview-card]");
    const out = (k) => card?.querySelector(`[data-preview-out="${k}"]`);
    const val = (name) => {
      const el = form?.querySelector(`[name="${name}"]:checked`) || form?.querySelector(`[name="${name}"]`);
      return el ? el.value : "";
    };
    const update = () => {
      const name = `${val("make")} ${val("model")}`.trim();
      if (out("name")) out("name").textContent = name || "New vehicle";
      if (out("daily")) out("daily").textContent = rupee(val("daily"));
      if (out("deposit")) out("deposit").textContent = rupee(val("deposit"));
      const km = Number(val("km"));
      if (out("km")) out("km").textContent = km > 0 ? `${km} km a day` : "unlimited km";
      const art = artFor(val("type"), val("seats"));
      const hasPhoto = !!card?.querySelector("[data-preview-photo]");
      card?.querySelectorAll("[data-preview-art]").forEach((a) => { a.hidden = hasPhoto || a.dataset.previewArt !== art; });
      const media = card?.querySelector("[data-preview-media]");
      if (media) media.className = `vcard-media tint-${art}`;
      gate();
    };
    const gate = () => {
      const photos = Number(document.getElementById("photo-manage")?.dataset.photoCount || 0);
      const checks = [
        !!(val("make").trim() && val("model").trim()),
        Number(val("hourly")) > 0 && Number(val("daily")) > 0,
        photos > 0,
      ];
      document.querySelectorAll("[data-gate] li").forEach((li, i) => li.classList.toggle("is-done", checks[i]));
      const pub = document.querySelector("[data-publish]");
      if (pub) pub.disabled = !checks.every(Boolean);
    };
    form?.addEventListener("input", update);
    form?.addEventListener("change", update);
    document.addEventListener("saarthi:photos", () => {
      const first = document.querySelector("#photo-manage img");
      const media = card?.querySelector("[data-preview-media]");
      media?.querySelector("[data-preview-photo]")?.remove();
      if (first && media) {
        const img = document.createElement("img");
        img.src = first.src;
        img.alt = "";
        img.dataset.previewPhoto = "";
        media.prepend(img);
      }
      update();
    });
    update();
  }
}
