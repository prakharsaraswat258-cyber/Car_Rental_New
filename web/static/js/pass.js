// Pickup Pass reveal after paying: the pass rises and flips once, the
// wheel makes one turn, the phone gives a 15 ms buzz. No confetti.
import { all, once, play, vibrate, DUR, ENTER } from "./motion.js";

export function reveal(pass) {
  pass.style.transformOrigin = "50% 100%";
  play(pass, [
    { opacity: 0, transform: "translateY(80px) rotateX(72deg)" },
    { opacity: 1, transform: "translateY(-6px) rotateX(-6deg)", offset: 0.6 },
    { opacity: 1, transform: "translateY(0) rotateX(0deg)" },
  ], { duration: 900, easing: ENTER });
  const wheel = pass.querySelector(".brand-mark");
  if (wheel) play(wheel, [{ transform: "rotate(0deg)" }, { transform: "rotate(360deg)" }], { duration: DUR.hero + 300, easing: ENTER, delay: 250 });
  setTimeout(() => vibrate(15), 500);
}

export function initPass(root) {
  for (const pass of all(root, "[data-pass-reveal]")) {
    if (!once(pass)) continue;
    reveal(pass);
    // don't replay on refresh
    const url = new URL(location.href);
    if (url.searchParams.has("new")) {
      url.searchParams.delete("new");
      history.replaceState(null, "", url.pathname + (url.search || ""));
    }
  }
}
