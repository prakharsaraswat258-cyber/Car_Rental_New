// Before / after wipe: a range input (keyboard friendly) drives where the
// return photo is clipped over the pickup photo.
import { all, once } from "./motion.js";

export function initWipes(root) {
  for (const w of all(root, "[data-wipe]")) {
    if (!once(w, "wipeReady")) continue;
    const range = w.querySelector(".pc-range");
    const set = () => w.style.setProperty("--pos", `${range.value}%`);
    range.addEventListener("input", set);
    set();
  }
}
