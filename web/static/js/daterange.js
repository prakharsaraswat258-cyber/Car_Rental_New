// Date range picker. Upgrades two datetime-local inputs (still the form
// fields) into a trigger + calendar sheet. Pick a start, then an end: the
// range fills as one sliding pill per week row. Booked days are struck
// through. Values are IST wall-clock strings: "2026-10-21T09:00".
import { all, once, play, esc, icon, reduced, DUR, ENTER } from "./motion.js";
import { openSheet, closeSheet } from "./sheet.js";

const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
const SHORT_M = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

// days are "YYYY-MM-DD" strings; math happens on UTC dates (no DST traps)
const toDate = (s) => { const [y, m, d] = s.split("-").map(Number); return new Date(Date.UTC(y, m - 1, d)); };
const toStr = (d) => d.toISOString().slice(0, 10);
const addDays = (s, n) => { const d = toDate(s); d.setUTCDate(d.getUTCDate() + n); return toStr(d); };

function clockLabel(hhmm) {
  let [h, m] = hhmm.split(":").map(Number);
  const suf = h >= 12 ? "pm" : "am";
  h = h % 12 || 12;
  return m ? `${h}:${String(m).padStart(2, "0")} ${suf}` : `${h} ${suf}`;
}
export function dayLabel(day) {
  const d = toDate(day);
  return `${DOW[d.getUTCDay()]} ${d.getUTCDate()} ${SHORT_M[d.getUTCMonth()]}`;
}

function times() {
  const out = [];
  for (let h = 6; h <= 22; h++) for (const m of [0, 30]) {
    if (h === 22 && m) continue;
    out.push(`${String(h).padStart(2, "0")}:${m ? "30" : "00"}`);
  }
  return out;
}

class Picker {
  constructor(root) {
    this.root = root;
    this.startIn = root.querySelector("[data-drp-start]");
    this.endIn = root.querySelector("[data-drp-end]");
    this.today = root.dataset.today || toStr(new Date());
    try { this.busy = new Set(JSON.parse(root.dataset.busy || "[]")); } catch { this.busy = new Set(); }
    this.read();
    root.classList.add("is-enhanced");
    root.querySelector("[data-drp-open]").addEventListener("click", () => this.open());
  }

  read() {
    const s = this.startIn.value, e = this.endIn.value;
    this.start = s ? s.slice(0, 10) : null;
    this.end = e ? e.slice(0, 10) : null;
    this.startTime = s ? s.slice(11, 16) : "09:00";
    this.endTime = e ? e.slice(11, 16) : "09:00";
    const base = this.start || this.today;
    this.view = base.slice(0, 7) + "-01";
  }

  build() {
    const d = document.createElement("dialog");
    d.className = "sheet wide";
    d.setAttribute("data-sheet", "");
    d.setAttribute("aria-label", "Choose pickup and return");
    const opts = times().map((t) => `<option value="${t}">${clockLabel(t)}</option>`).join("");
    d.innerHTML = `<div class="sheet-panel">
      <div class="sheet-handle" data-sheet-handle></div>
      <header class="sheet-head"><h2>When do you need it?</h2><button class="icon-btn" type="button" data-sheet-close aria-label="Close">${icon("x")}</button></header>
      <div class="sheet-body">
        <div class="drp-nav"><button class="icon-btn" type="button" data-prev aria-label="Previous month">${icon("chevron-left")}</button>
          <p class="drp-summary" data-summary aria-live="polite"></p>
          <button class="icon-btn" type="button" data-next aria-label="Next month">${icon("chevron-right")}</button></div>
        <div class="drp-months" data-months></div>
        <div class="drp-times">
          <label class="field"><span class="field-label">Pickup time</span><select class="input" data-start-time>${opts}</select></label>
          <label class="field"><span class="field-label">Return time</span><select class="input" data-end-time>${opts}</select></label>
        </div>
      </div>
      <footer class="sheet-foot"><button class="btn btn-ghost" type="button" data-clear>Clear</button><button class="btn btn-primary grow" type="button" data-done>Done</button></footer>
    </div>`;
    document.body.appendChild(d);
    this.dialog = d;
    d.querySelector("[data-prev]").addEventListener("click", () => this.shift(-1));
    d.querySelector("[data-next]").addEventListener("click", () => this.shift(1));
    d.querySelector("[data-start-time]").addEventListener("change", (e) => { this.startTime = e.target.value; this.summary(); });
    d.querySelector("[data-end-time]").addEventListener("change", (e) => { this.endTime = e.target.value; this.summary(); });
    d.querySelector("[data-clear]").addEventListener("click", () => { this.start = this.end = null; this.render(); });
    d.querySelector("[data-done]").addEventListener("click", () => this.done());
    d.querySelector("[data-months]").addEventListener("click", (e) => {
      const b = e.target.closest("[data-day]");
      if (b && !b.disabled) this.pick(b.dataset.day);
    });
    document.dispatchEvent(new CustomEvent("saarthi:init", { detail: d }));
  }

  open() {
    this.read();
    if (!this.dialog) this.build();
    for (const [sel, v] of [["[data-start-time]", this.startTime], ["[data-end-time]", this.endTime]]) {
      const s = this.dialog.querySelector(sel);
      if (![...s.options].some((o) => o.value === v)) s.add(new Option(clockLabel(v), v));
      s.value = v;
    }
    openSheet(this.dialog);
    this.render(); // after opening: pills need real layout
  }

  shift(n) {
    const d = toDate(this.view);
    d.setUTCMonth(d.getUTCMonth() + n);
    const min = this.today.slice(0, 7) + "-01";
    const v = toStr(d);
    this.view = v < min ? min : v;
    this.render();
  }

  month(first) {
    const d = toDate(first);
    const y = d.getUTCFullYear(), m = d.getUTCMonth();
    const lead = (d.getUTCDay() + 6) % 7; // Monday first
    const days = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
    let cells = "";
    for (const w of ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"]) cells += `<div class="cal-dow">${w}</div>`;
    for (let i = 0; i < lead; i++) cells += `<span></span>`;
    for (let day = 1; day <= days; day++) {
      const s = `${y}-${String(m + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
      const cls = ["drp-day"];
      if (s === this.today) cls.push("is-today");
      if (this.busy.has(s)) cls.push("is-busy");
      if (s === this.start) cls.push("is-start");
      if (s === this.end) cls.push("is-end");
      const past = s < this.today;
      const label = `${dayLabel(s)}${this.busy.has(s) ? ", partly or fully booked" : ""}`;
      cells += `<button type="button" class="${cls.join(" ")}" data-day="${s}" ${past ? "disabled" : ""} aria-label="${esc(label)}" aria-pressed="${s === this.start || s === this.end}">${day}</button>`;
    }
    return `<div class="drp-month"><h3>${MONTHS[m]} ${y}</h3><div class="drp-grid">${cells}</div></div>`;
  }

  render(animate = false) {
    const box = this.dialog.querySelector("[data-months]");
    const second = toDate(this.view);
    second.setUTCMonth(second.getUTCMonth() + 1);
    box.innerHTML = this.month(this.view) + this.month(toStr(second));
    this.dialog.querySelector("[data-prev]").disabled = this.view <= this.today.slice(0, 7) + "-01";
    this.pills(animate);
    this.summary();
  }

  // one pill per week row covering start..end
  pills(animate) {
    if (!this.start || !this.end) return;
    let row = 0;
    for (const grid of this.dialog.querySelectorAll(".drp-grid")) {
      const inRange = [...grid.querySelectorAll("[data-day]")].filter((b) => b.dataset.day >= this.start && b.dataset.day <= this.end);
      const rows = new Map();
      for (const b of inRange) {
        const top = b.offsetTop;
        if (!rows.has(top)) rows.set(top, []);
        rows.get(top).push(b);
      }
      for (const [top, cells] of rows) {
        const a = cells[0], z = cells[cells.length - 1];
        const pill = document.createElement("span");
        pill.className = "drp-range";
        pill.style.left = `${a.offsetLeft}px`;
        pill.style.top = `${top}px`;
        pill.style.width = `${z.offsetLeft + z.offsetWidth - a.offsetLeft}px`;
        grid.appendChild(pill);
        if (animate && !reduced()) {
          play(pill, [{ transform: "scaleX(0)", opacity: 0.4 }, { transform: "scaleX(1)", opacity: 1 }],
            { duration: DUR.slow, easing: ENTER, delay: row * 60, fill: "backwards" });
        }
        row++;
      }
    }
  }

  pick(day) {
    if (!this.start || (this.start && this.end) || day < this.start) {
      this.start = day;
      this.end = null;
      this.render();
      return;
    }
    this.end = day;
    this.render(true);
  }

  summary() {
    const el = this.dialog.querySelector("[data-summary]");
    if (!this.start) { el.innerHTML = "Pick a pickup day"; return; }
    if (!this.end) { el.innerHTML = `<b>${dayLabel(this.start)}</b> → pick a return day`; return; }
    const s = `${this.start}T${this.startTime}`, e = `${this.end}T${this.endTime}`;
    el.innerHTML = e <= s
      ? `<span style="color:var(--bad)">Return must be after pickup</span>`
      : `<b>${dayLabel(this.start)}, ${clockLabel(this.startTime)}</b> → <b>${dayLabel(this.end)}, ${clockLabel(this.endTime)}</b>`;
  }

  done() {
    if (this.start && !this.end) this.end = addDays(this.start, 1);
    if (this.start && this.end) {
      let s = `${this.start}T${this.startTime}`, e = `${this.end}T${this.endTime}`;
      if (e <= s) { e = `${addDays(this.start, 1)}T${this.startTime}`; }
      this.startIn.value = s;
      this.endIn.value = e;
      this.root.querySelector('[data-drp-label="start"]').textContent = `${dayLabel(this.start)}, ${clockLabel(this.startTime)}`;
      this.root.querySelector('[data-drp-label="end"]').textContent = `${dayLabel(e.slice(0, 10))}, ${clockLabel(e.slice(11, 16))}`;
    } else {
      this.startIn.value = this.endIn.value = "";
      this.root.querySelector('[data-drp-label="start"]').textContent = "Add date";
      this.root.querySelector('[data-drp-label="end"]').textContent = "Add date";
    }
    closeSheet(this.dialog);
    this.startIn.dispatchEvent(new Event("change", { bubbles: true }));
  }
}

export function initDatePickers(root) {
  for (const el of all(root, "[data-drp]")) {
    if (!once(el)) continue;
    el._picker = new Picker(el);
  }
  for (const b of all(root, "[data-drp-trigger-for]")) {
    if (!once(b)) continue;
    b.addEventListener("click", () => {
      const host = document.getElementById(b.dataset.drpTriggerFor) || document;
      host.querySelector("[data-drp]")?._picker?.open();
    });
  }
}
