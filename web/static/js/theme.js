// Theme switch: light / dark / system. The server already rendered
// data-theme from the cookie (no flash); this makes the switch instant,
// cross-fades with a View Transition, and saves the choice.

const html = document.documentElement;

function apply(choice) {
  if (choice === "system") delete html.dataset.theme;
  else html.dataset.theme = choice;
  html.dataset.themeChoice = choice;
  for (const btn of document.querySelectorAll("[data-theme-switch] button[value]")) {
    btn.setAttribute("aria-pressed", String(btn.value === choice));
  }
}

function switchTo(choice) {
  const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (document.startViewTransition && !reduce) {
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

export function initThemeSwitch(root) {
  for (const form of root.querySelectorAll("[data-theme-switch]")) {
    if (form.dataset.ready) continue;
    form.dataset.ready = "1";
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      const choice = e.submitter?.value;
      if (choice) switchTo(choice);
    });
  }
}
