// Deposit Tracker: a coin travels Held → Inspection → Released → Credited,
// hopping stop to stop, and the amount counts down when something was
// kept. The last stop seen is remembered per booking, so polling refreshes
// only animate the new part of the journey.
import { all, once, play, reduced, ENTER } from "./motion.js";
import { setOdometer } from "./odometer.js";

function stopX(track, i) {
  const stops = track.querySelectorAll(".dep-stop");
  const s = stops[i];
  if (!s) return 0;
  return s.offsetLeft + s.offsetWidth / 2 - 20;
}

export async function journey(el, from = 0) {
  const stop = Number(el.dataset.stop);
  const track = el.querySelector(".dep-track");
  const coin = el.querySelector(".dep-coin");
  const rail = el.querySelector(".dep-rail-fill");
  const amount = el.querySelector(".dep-amount .odo");
  if (!track || !coin || stop < 0) return;
  const railScale = (i) => `scaleX(${i / 3})`;
  const place = (i) => {
    coin.style.transform = `translate(${stopX(track, i)}px, 0)`;
    if (rail) rail.style.transform = railScale(i);
  };
  const held = el.dataset.held, refund = el.dataset.refund;
  if (reduced() || from >= stop) {
    place(stop);
    if (amount && stop >= 2) setOdometer(amount, refund);
    return;
  }
  place(from);
  if (amount) setOdometer(amount, held);
  for (let i = from; i < stop; i++) {
    const x0 = stopX(track, i), x1 = stopX(track, i + 1);
    const hop = play(coin, [
      { transform: `translate(${x0}px, 0) rotate(0deg)` },
      { transform: `translate(${(x0 + x1) / 2}px, -14px) rotate(180deg)`, offset: 0.5 },
      { transform: `translate(${x1}px, 0) rotate(360deg)` },
    ], { duration: 520, easing: ENTER, fill: "forwards" });
    if (rail) play(rail, [{ transform: railScale(i) }, { transform: railScale(i + 1) }], { duration: 520, easing: ENTER, fill: "forwards" });
    await hop;
    place(i + 1);
    if (i + 1 === 2 && amount && held !== refund) setOdometer(amount, refund);
  }
}

export function initDeposits(root) {
  for (const el of all(root, "[data-deposit]")) {
    if (!once(el)) continue;
    const key = `dep:${el.dataset.key}`;
    let seen = 0;
    try { seen = Number(sessionStorage.getItem(key) || 0); } catch { /* private mode */ }
    const stop = Number(el.dataset.stop);
    const from = seen > stop ? 0 : seen;
    const run = () => {
      journey(el, from);
      try { sessionStorage.setItem(key, String(stop)); } catch { /* ignore */ }
    };
    if ("IntersectionObserver" in window) {
      const io = new IntersectionObserver((entries) => {
        if (entries.some((e) => e.isIntersecting)) { io.disconnect(); run(); }
      }, { threshold: 0.4 });
      io.observe(el);
    } else run();
  }
}
