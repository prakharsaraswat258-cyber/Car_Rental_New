// Help popovers: a "?" button opens a short explanation. Tap, click or
// Enter/Space toggles; Esc or a tap outside closes and returns focus.
import { all, once, play, DUR, ENTER } from "./motion.js";

let open = null;

function close(restore = false) {
  if (!open) return;
  const { btn, pop } = open;
  pop.hidden = true;
  btn.setAttribute("aria-expanded", "false");
  if (restore) btn.focus();
  open = null;
}

function place(btn, pop) {
  pop.classList.remove("below", "is-left", "is-right");
  const r = btn.getBoundingClientRect();
  if (r.top < 140) pop.classList.add("below");
  const p = pop.getBoundingClientRect();
  if (p.right > window.innerWidth - 8) pop.classList.add("is-left");
  else if (p.left < 8) pop.classList.add("is-right");
}

export function initHelp(root) {
  for (const wrap of all(root, "[data-help]")) {
    if (!once(wrap)) continue;
    const btn = wrap.querySelector(".help-btn");
    const pop = wrap.querySelector(".help-pop");
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      const wasOpen = open && open.pop === pop;
      close();
      if (wasOpen) return;
      pop.hidden = false;
      btn.setAttribute("aria-expanded", "true");
      place(btn, pop);
      play(pop, [{ opacity: 0 }, { opacity: 1 }], { duration: DUR.fast, easing: ENTER });
      open = { btn, pop };
    });
  }
  if (!window.__helpWired) {
    window.__helpWired = true;
    document.addEventListener("click", (e) => { if (open && !open.pop.contains(e.target)) close(); });
    document.addEventListener("keydown", (e) => { if (e.key === "Escape" && open) { e.stopPropagation(); close(true); } }, true);
    window.addEventListener("scroll", () => close(), { passive: true });
  }
}
