// Saarthi front-end entry. Each component is a small module wired through
// data-* attributes. init() runs on load and again on every htmx swap, so
// server-rendered fragments come alive the same way full pages do.
import { initThemeSwitch } from "./theme.js";

const components = [initThemeSwitch];

function init(root) {
  for (const c of components) {
    try {
      c(root);
    } catch (err) {
      console.error("[saarthi] component failed", c.name, err);
    }
  }
}

init(document);
document.addEventListener("htmx:afterSwap", (e) => init(e.target));
