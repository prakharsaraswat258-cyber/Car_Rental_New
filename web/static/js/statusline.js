// Status line: the road between reached steps draws like road paint, one
// segment after another; the current step pulses (CSS).
import { all, once, play, reduced, DUR, ENTER } from "./motion.js";

export function paint(line, fromStep = 0) {
  const step = Number(line.dataset.step || 0);
  const steps = [...line.querySelectorAll(".sl-step")];
  steps.forEach((li, i) => {
    const n = i + 1;
    const fill = li.querySelector(".sl-fill");
    li.classList.toggle("is-done", n <= step);
    li.classList.toggle("is-current", n === step && step < 6);
    if (!fill) return;
    const on = n < step;
    fill.style.transform = on ? "scaleX(1)" : "scaleX(0)";
    if (on && n >= fromStep && !reduced()) {
      play(fill, [{ transform: "scaleX(0)" }, { transform: "scaleX(1)" }],
        { duration: DUR.slow, easing: ENTER, delay: (n - fromStep) * 140, fill: "backwards" });
    }
  });
}

export function initStatusLines(root) {
  for (const line of all(root, "[data-statusline]")) {
    if (!once(line)) continue;
    paint(line, 1);
  }
}
