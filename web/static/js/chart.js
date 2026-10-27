// Admin charts: crosshair + dot + tooltip on hover/touch over the hit
// bands. The table view under each chart carries the same numbers for
// keyboard and screen-reader users.
import { all, once } from "./motion.js";

export function initCharts(root) {
  for (const fig of all(root, "[data-chart]")) {
    if (!once(fig, "chartReady")) continue;
    const svg = fig.querySelector("svg");
    const tip = fig.querySelector(".chart-tip");
    const cross = svg.querySelector(".chart-cross");
    const dot = svg.querySelector(".chart-dot");
    const plot = fig.querySelector(".chart-plot");
    // SVG elements have no .hidden property; toggle the attribute (CSS hides it)
    const vis = (el, on) => el.toggleAttribute("hidden", !on);
    const show = (hit) => {
      const x = Number(hit.dataset.x), y = Number(hit.dataset.y);
      cross.setAttribute("x1", x); cross.setAttribute("x2", x); vis(cross, true);
      dot.setAttribute("cx", x); dot.setAttribute("cy", y); vis(dot, true);
      const vb = svg.viewBox.baseVal, r = svg.getBoundingClientRect(), pr = plot.getBoundingClientRect();
      tip.innerHTML = "";
      const b = document.createElement("b");
      b.textContent = hit.dataset.value;
      tip.append(b, hit.dataset.day);
      tip.hidden = false;
      const left = r.left - pr.left + (x / vb.width) * r.width;
      const top = r.top - pr.top + (y / vb.height) * r.height;
      tip.style.left = `${Math.max(60, Math.min(pr.width - 60, left))}px`;
      tip.style.top = `${top}px`;
    };
    const hide = () => { tip.hidden = true; vis(cross, false); vis(dot, false); };
    svg.addEventListener("pointermove", (e) => { const h = e.target.closest(".chart-hit"); if (h) show(h); });
    svg.addEventListener("pointerdown", (e) => { const h = e.target.closest(".chart-hit"); if (h) show(h); });
    svg.addEventListener("pointerleave", hide);
  }
}
