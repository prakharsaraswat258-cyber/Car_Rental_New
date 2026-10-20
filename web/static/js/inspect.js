// Inspection: add / remove deduction rows; the settlement re-computes on
// the server (htmx) after every change.
import { all, once, play, DUR, ENTER } from "./motion.js";

export function initInspect(root) {
  for (const form of all(root, "form[data-inspect]")) {
    if (!once(form)) continue;
    const rows = form.querySelector("[data-ded-rows]");
    const tpl = form.querySelector("template[data-ded-template]");
    const recalc = () => window.htmx?.trigger(form, "recalc");
    form.querySelector("[data-ded-add]")?.addEventListener("click", () => {
      const row = tpl.content.firstElementChild.cloneNode(true);
      rows.appendChild(row);
      play(row, [{ opacity: 0, transform: "translateY(-6px)" }, { opacity: 1, transform: "none" }], { duration: DUR.base, easing: ENTER });
      row.querySelector("input[name=amount]")?.focus();
    });
    rows.addEventListener("click", async (e) => {
      const btn = e.target.closest("[data-ded-remove]");
      if (!btn) return;
      const row = btn.closest(".ded-row");
      await play(row, [{ opacity: 1 }, { opacity: 0, transform: "translateX(16px)" }], { duration: DUR.fast });
      row.remove();
      recalc();
    });
  }
}
