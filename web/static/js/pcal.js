// Provider calendar: tap a start day, then an end day on the same row to
// block those days. A clash with a booking shakes that bar 3 times and
// toasts "Already booked". Tapping a block removes it, with Undo.
import { all, once, request } from "./motion.js";
import { toast } from "./toast.js";

let pick = null;

function clearPick(grid) {
  pick = null;
  grid.querySelectorAll(".is-pick, .is-range").forEach((c) => c.classList.remove("is-pick", "is-range"));
}

async function reload(grid) {
  const res = await request(location.pathname + location.search, { method: "GET" });
  if (!res.ok) return;
  const html = await res.text();
  const doc = new DOMParser().parseFromString(html, "text/html");
  const fresh = doc.getElementById("pcal");
  if (fresh) {
    grid.replaceWith(fresh);
    document.dispatchEvent(new CustomEvent("saarthi:init", { detail: fresh }));
  }
}

function shake(grid, code) {
  const bar = grid.querySelector(`.pcal-bar[data-code="${code}"]`) || document.querySelector(`.pcal-bar[data-code="${code}"]`);
  if (!bar) return;
  bar.classList.remove("is-shaking");
  void bar.offsetWidth;
  bar.classList.add("is-shaking");
  bar.addEventListener("animationend", () => bar.classList.remove("is-shaking"), { once: true });
}

async function block(grid, vehicle, start, end) {
  const res = await request("/p/blocks", { method: "POST", body: new URLSearchParams({ vehicle, start, end }), accept: "application/json" });
  const data = await res.json().catch(() => ({}));
  clearPick(grid);
  if (res.status === 409) {
    shake(grid, data.code);
    toast({ message: "Already booked", tone: "bad" });
    return;
  }
  if (!res.ok) { toast({ message: data.error || "Couldn't block those days.", tone: "bad" }); return; }
  await reload(grid);
  toast({ message: data.message, tone: "good", undo: { url: `/p/blocks/${data.id}`, method: "DELETE", reload: false } });
}

export function initProviderCalendar(root) {
  for (const grid of all(root, "[data-pcal]")) {
    if (!once(grid)) continue;
    grid.addEventListener("click", async (e) => {
      const blockBar = e.target.closest("[data-block-id]");
      if (blockBar) {
        const res = await request(`/p/blocks/${blockBar.dataset.blockId}`, { method: "DELETE", accept: "application/json" });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) { toast({ message: data.error || "Couldn't remove that block.", tone: "bad" }); return; }
        await reload(grid);
        toast({ message: data.message, undo: { ...data.undo, reload: false } });
        return;
      }
      const cell = e.target.closest(".pcal-cell");
      if (!cell) return;
      const { vehicle, day } = cell.dataset;
      if (!pick || pick.vehicle !== vehicle) {
        clearPick(grid);
        pick = { vehicle, day };
        cell.classList.add("is-pick");
        toast({ message: "Now tap the last day to block. Esc to cancel." });
        return;
      }
      const [start, end] = day < pick.day ? [day, pick.day] : [pick.day, day];
      grid.querySelectorAll(`.pcal-cell[data-vehicle="${vehicle}"]`).forEach((c) => {
        if (c.dataset.day >= start && c.dataset.day <= end) c.classList.add("is-range");
      });
      await block(grid, vehicle, start, end);
    });
  }
  if (!window.__pcalEsc) {
    window.__pcalEsc = true;
    document.addEventListener("keydown", (e) => {
      const grid = document.querySelector("[data-pcal]");
      if (e.key === "Escape" && grid && pick) clearPick(grid);
    });
    document.addEventListener("saarthi:undone", () => {
      const grid = document.querySelector("[data-pcal]");
      if (grid) reload(grid);
    });
  }
}
