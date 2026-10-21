// Vehicle gallery: swipeable carousel with dots on phones, 1 + 4 grid on
// desktop; tap any picture for a full-screen lightbox (swipe, arrow keys,
// pull down to close).
import { all, once, play, spring, DUR, ENTER, reduced } from "./motion.js";

function lightbox(dialog, start) {
  const slides = [...dialog.querySelectorAll("[data-lb-slide]")];
  const count = dialog.querySelector("[data-lb-count]");
  const stage = dialog.querySelector("[data-lb-stage]");
  let i = start;
  const show = (n, dir = 0) => {
    i = (n + slides.length) % slides.length;
    slides.forEach((s, k) => { s.hidden = k !== i; });
    count.textContent = `${i + 1} of ${slides.length}`;
    if (dir) play(slides[i], [{ opacity: 0, transform: `translateX(${dir * 40}px)` }, { opacity: 1, transform: "none" }], { duration: DUR.base, easing: ENTER });
  };
  show(i);
  if (!dialog.open) dialog.showModal();
  document.body.classList.add("has-modal");
  play(dialog, [{ opacity: 0 }, { opacity: 1 }], { duration: DUR.fast });

  if (once(dialog, "lbReady")) {
    const closeLb = () => {
      play(dialog, [{ opacity: 1 }, { opacity: 0 }], { duration: DUR.fast }).then(() => {
        dialog.close();
        stage.style.transform = "";
        document.body.classList.remove("has-modal");
      });
    };
    dialog.addEventListener("cancel", (e) => { e.preventDefault(); closeLb(); });
    dialog.querySelector("[data-lb-close]").addEventListener("click", closeLb);
    dialog.querySelector("[data-lb-prev]").addEventListener("click", () => show(i - 1, -1));
    dialog.querySelector("[data-lb-next]").addEventListener("click", () => show(i + 1, 1));
    dialog.addEventListener("keydown", (e) => {
      if (e.key === "ArrowRight") show(i + 1, 1);
      if (e.key === "ArrowLeft") show(i - 1, -1);
    });
    // swipe sideways to change, pull down to close
    let x0 = 0, y0 = 0, dx = 0, dy = 0, track = false;
    stage.addEventListener("pointerdown", (e) => {
      if (e.pointerType === "mouse") return;
      track = true; x0 = e.clientX; y0 = e.clientY; dx = dy = 0;
    });
    stage.addEventListener("pointermove", (e) => {
      if (!track) return;
      dx = e.clientX - x0; dy = e.clientY - y0;
      if (dy > 0 && Math.abs(dy) > Math.abs(dx)) stage.style.transform = `translateY(${dy}px)`;
    });
    const end = () => {
      if (!track) return;
      track = false;
      if (dy > 120 && Math.abs(dy) > Math.abs(dx)) { closeLb(); return; }
      if (Math.abs(dx) > 60 && Math.abs(dx) > Math.abs(dy)) show(i + (dx < 0 ? 1 : -1), dx < 0 ? 1 : -1);
      spring(Math.max(0, dy), 0, (v) => { stage.style.transform = v ? `translateY(${v}px)` : ""; });
    };
    stage.addEventListener("pointerup", end);
    stage.addEventListener("pointercancel", end);
  }
}

export function initGallery(root) {
  for (const g of all(root, "[data-gallery]")) {
    if (!once(g, "galleryReady")) continue;
    const track = g.querySelector("[data-gallery-track]");
    const dots = [...g.querySelectorAll(".gallery-dots span")];
    const dialog = document.querySelector("[data-lightbox]");
    track?.addEventListener("scroll", () => {
      const n = Math.round(track.scrollLeft / Math.max(1, track.clientWidth));
      dots.forEach((d, k) => d.classList.toggle("is-on", k === n));
    }, { passive: true });
    g.addEventListener("click", (e) => {
      const b = e.target.closest("[data-gallery-open]");
      if (!b || !dialog) return;
      lightbox(dialog, Number(b.dataset.galleryOpen));
    });
    if (reduced()) track?.style.setProperty("scroll-behavior", "auto");
  }
}
