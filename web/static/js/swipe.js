// Swipe to confirm: the thumb follows the finger and the track fills;
// released early it springs back. Keyboard: focus the thumb and hold Enter
// (or Space) for one second.
import { all, once, spring, vibrate, play, reduced } from "./motion.js";
import { toast } from "./toast.js";

export function resetSwipe(sw) {
  sw.classList.remove("is-done");
  sw._set?.(0);
  const label = sw.querySelector(".swipe-label");
  if (label && sw.dataset.label) label.textContent = sw.dataset.label;
}

export function initSwipes(root) {
  for (const sw of all(root, "[data-swipe]")) {
    if (!once(sw)) continue;
    const thumb = sw.querySelector(".swipe-thumb");
    const fill = sw.querySelector(".swipe-fill");
    const label = sw.querySelector(".swipe-label");
    sw.dataset.label = label.textContent;
    let x = 0, startX = 0, dragging = false, holdRAF = 0;
    const max = () => sw.clientWidth - thumb.offsetWidth - 12;
    const set = (v) => {
      x = Math.max(0, Math.min(v, max()));
      thumb.style.transform = `translateX(${x}px)`;
      fill.style.transform = `scaleX(${(x + thumb.offsetWidth + 6) / sw.clientWidth})`;
      label.style.opacity = String(Math.max(0, 1 - x / (max() * 0.6)));
    };
    sw._set = set;
    set(0);
    fill.style.transform = "scaleX(0)";

    const disabled = () => sw.getAttribute("aria-disabled") === "true" || sw.classList.contains("is-done");
    const refuse = () => {
      if (sw.classList.contains("is-done")) return;
      play(thumb, [{ transform: "translateX(0)" }, { transform: "translateX(8px)" }, { transform: "translateX(0)" }], { duration: 260 });
      if (sw.dataset.hint) toast({ message: sw.dataset.hint, tone: "warn" });
    };
    const complete = () => {
      set(max());
      sw.classList.add("is-done");
      label.style.opacity = "1";
      label.textContent = sw.dataset.doneLabel || "Done";
      vibrate(15);
      if (sw.hasAttribute("data-demo-swipe")) {
        toast({ message: "Confirmed (demo). Nothing was sent.", tone: "good" });
        return;
      }
      const form = document.getElementById(sw.dataset.form);
      if (form) {
        if (!form.reportValidity()) { resetSwipe(sw); return; }
        form.requestSubmit();
      }
    };
    const back = () => spring(x, 0, set);

    thumb.addEventListener("pointerdown", (e) => {
      if (disabled()) { refuse(); return; }
      dragging = true;
      startX = e.clientX - x;
      thumb.setPointerCapture(e.pointerId);
    });
    thumb.addEventListener("pointermove", (e) => { if (dragging) set(e.clientX - startX); });
    const release = () => {
      if (!dragging) return;
      dragging = false;
      if (x >= max() * 0.9) complete();
      else back();
    };
    thumb.addEventListener("pointerup", release);
    thumb.addEventListener("pointercancel", release);

    // keyboard: hold Enter or Space for 1 s
    const holdStart = (e) => {
      if ((e.key !== "Enter" && e.key !== " ") || e.repeat) return;
      e.preventDefault();
      if (disabled()) { refuse(); return; }
      const t0 = performance.now();
      const tick = (t) => {
        const p = Math.min(1, (t - t0) / 1000);
        set(max() * p);
        if (p >= 1) { complete(); return; }
        holdRAF = requestAnimationFrame(tick);
      };
      holdRAF = requestAnimationFrame(tick);
    };
    const holdEnd = (e) => {
      if (e.key !== "Enter" && e.key !== " ") return;
      cancelAnimationFrame(holdRAF);
      if (!sw.classList.contains("is-done")) back();
    };
    thumb.addEventListener("keydown", holdStart);
    thumb.addEventListener("keyup", holdEnd);
    if (reduced()) thumb.style.transition = "none";
  }
}
