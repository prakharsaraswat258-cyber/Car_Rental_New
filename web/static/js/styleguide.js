// Demo controls on /dev/styleguide.
import { all, once, play, DUR } from "./motion.js";
import { toast } from "./toast.js";
import { setOdometer } from "./odometer.js";
import { busy, idle } from "./loader.js";
import { paint } from "./statusline.js";
import { journey } from "./deposit.js";
import { reveal } from "./pass.js";
import { resetSwipe } from "./swipe.js";

const rupee = (n) => {
  const s = String(n);
  if (s.length <= 3) return `₹${s}`;
  return `₹${s.slice(0, -3).replace(/\B(?=(\d{2})+(?!\d))/g, ",")},${s.slice(-3)}`;
};

export function initStyleguide(root) {
  const sg = document.querySelector("[data-styleguide]");
  if (!sg || !once(sg, "sgReady")) return;
  sg.querySelectorAll("form[data-demo-form]").forEach((f) => f.addEventListener("submit", (e) => e.preventDefault()));

  sg.addEventListener("click", (e) => {
    const b = e.target.closest("[data-demo]");
    if (!b) return;
    switch (b.dataset.demo) {
      case "loading":
        b.classList.add("is-loading");
        setTimeout(() => b.classList.remove("is-loading"), 1600);
        break;
      case "odo": {
        const el = document.getElementById(b.dataset.target);
        setOdometer(el, rupee(400 + Math.floor(Math.random() * 4600)));
        break;
      }
      case "toast":
        toast({ message: "Accepted. Riya has her Pickup Pass.", tone: "good" });
        break;
      case "toast-undo":
        toast({ message: "Block removed", undo: { url: "/dev/styleguide", method: "GET", reload: false } });
        break;
      case "loader":
        busy();
        setTimeout(idle, 2000);
        break;
      case "status-next":
      case "status-prev": {
        const line = sg.querySelector("[data-sg-status]");
        const cur = Number(line.dataset.step);
        const next = Math.max(0, Math.min(6, cur + (b.dataset.demo === "status-next" ? 1 : -1)));
        line.dataset.step = String(next);
        paint(line, next);
        break;
      }
      case "deposit-replay":
        journey(sg.querySelector("[data-deposit]"), 0);
        break;
      case "angle":
        sg.querySelector(`[data-silhouette] [data-zone="${b.dataset.angle}"]`)?.classList.add("is-filled");
        break;
      case "angle-reset":
        sg.querySelectorAll("[data-silhouette] .zone").forEach((z) => z.classList.remove("is-filled"));
        break;
      case "swipe-reset":
        sg.querySelectorAll("[data-demo-swipe]").forEach(resetSwipe);
        break;
      case "pass":
        reveal(sg.querySelector("[data-sg-pass]"));
        break;
      case "shake": {
        const bar = sg.querySelector("[data-sg-bar]");
        bar.classList.remove("is-shaking");
        void bar.offsetWidth;
        bar.classList.add("is-shaking");
        toast({ message: "Already booked", tone: "bad" });
        break;
      }
    }
  });
  // reveal sections gently as they scroll in
  const io = "IntersectionObserver" in window ? new IntersectionObserver((entries) => {
    for (const en of entries) {
      if (!en.isIntersecting) continue;
      io.unobserve(en.target);
      play(en.target, [{ opacity: 0.001, transform: "translateY(12px)" }, { opacity: 1, transform: "none" }], { duration: DUR.slow });
    }
  }, { threshold: 0.1 }) : null;
  all(sg, ".sg-section").forEach((s) => io?.observe(s));
}
