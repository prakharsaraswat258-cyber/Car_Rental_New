// Price odometer: when a total changes, the changed digits roll vertically
// like a car odometer (240 ms per digit, 20 ms stagger). The previous value
// is remembered per element id across htmx swaps.
import { all, reduced } from "./motion.js";

const last = new Map();
const DIGITS = "0123456789";

function strip(d) {
  let s = "";
  for (const n of DIGITS) s += `<span>${n}</span>`;
  return `<span class="odo-digit" data-d="${d}"><span class="odo-strip" style="transform:translateY(-${Number(d) * 1.15}em)">${s}</span></span>`;
}

function render(el, text) {
  let html = "";
  for (const ch of text) {
    html += DIGITS.includes(ch) ? strip(ch) : `<span class="odo-char">${ch === " " ? "&nbsp;" : ch.replace(/[<>&]/g, "")}</span>`;
  }
  el.innerHTML = `<span aria-hidden="true" style="display:inline-flex">${html}</span>`;
  el.setAttribute("aria-label", text);
}

// roll animates el from the text it shows to `to`. Digits are aligned from
// the right so ₹980 → ₹1,062 rolls the right columns.
export function roll(el, from, to) {
  if (reduced() || from === to) {
    render(el, to);
    if (from !== to) el.animate?.([{ opacity: 0.4 }, { opacity: 1 }], { duration: 120 });
    return;
  }
  const fromDigits = from.replace(/\D/g, "");
  render(el, to);
  const strips = [...el.querySelectorAll(".odo-digit")];
  const n = strips.length;
  strips.forEach((s, i) => {
    const fromD = fromDigits[fromDigits.length - (n - i)] ?? "0";
    const target = s.dataset.d;
    const st = s.firstElementChild;
    if (fromD === target) return;
    st.style.transition = "none";
    st.style.transform = `translateY(-${Number(fromD) * 1.15}em)`;
    st.getBoundingClientRect(); // commit the start position
    const delay = (n - 1 - i) * 20;
    st.style.transition = `transform 240ms cubic-bezier(0.2, 0, 0, 1) ${delay}ms`;
    st.style.transform = `translateY(-${Number(target) * 1.15}em)`;
  });
}

export function setOdometer(el, value) {
  const from = el.getAttribute("aria-label") || el.textContent.trim();
  el.dataset.odo = value;
  if (el.id) last.set(el.id, value);
  requestAnimationFrame(() => roll(el, from, value));
}

export function initOdometers(root) {
  for (const el of all(root, "[data-odo]")) {
    if (el.dataset.odoReady) continue;
    el.dataset.odoReady = "1";
    const value = el.dataset.odo;
    const prev = el.id ? last.get(el.id) : undefined;
    if (el.id) last.set(el.id, value);
    if (prev && prev !== value) {
      render(el, prev);
      requestAnimationFrame(() => requestAnimationFrame(() => roll(el, prev, value)));
    } else {
      render(el, value);
    }
  }
}
