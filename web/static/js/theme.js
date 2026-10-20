// Theme switch: light / dark / system. The server already rendered
// data-theme from the cookie (no flash); this makes the switch instant,
// cross-fades with a View Transition (200 ms), and saves the choice.
import { all, once, reduced } from "./motion.js";

const html = document.documentElement;

export function isDark() {
  const t = html.dataset.theme;
  if (t === "dark") return true;
  if (t === "light") return false;
  return matchMedia("(prefers-color-scheme: dark)").matches;
}

function apply(choice) {
  if (choice === "system") delete html.dataset.theme;
  else html.dataset.theme = choice;
  html.dataset.themeChoice = choice;
  for (const btn of document.querySelectorAll("[data-theme-switch] button[value]")) {
    btn.setAttribute("aria-pressed", String(btn.value === choice));
  }
  document.dispatchEvent(new CustomEvent("saarthi:theme", { detail: { dark: isDark() } }));
}

function switchTo(choice) {
  if (document.startViewTransition && !reduced()) {
    document.startViewTransition(() => apply(choice));
  } else {
    apply(choice);
  }
  fetch("/theme", {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", "X-Requested-With": "fetch" },
    body: new URLSearchParams({ theme: choice }),
  }).catch(() => {}); // the switch already happened; saving is best-effort
}

matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  document.dispatchEvent(new CustomEvent("saarthi:theme", { detail: { dark: isDark() } }));
});

export function initThemeSwitch(root) {
  for (const form of all(root, "[data-theme-switch]")) {
    if (!once(form)) continue;
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      const choice = e.submitter?.value;
      if (choice) switchTo(choice);
    });
  }
}
