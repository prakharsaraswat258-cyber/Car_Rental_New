// Provider calendar.
//  - mouse / pen: drag across day cells on a row; a ghost bar snaps to
//    whole days; release to block them (maintenance, hatched)
//  - touch: tap the first day, then the last
//  - keyboard: arrows move, Shift + arrows select along the row, Enter
//    blocks, Esc cancels
// A clash with a booking shakes that bar 3 times and toasts
// "Already booked". Tapping a block removes it, with Undo.
import { all, once, request } from "./motion.js";
import { toast } from "./toast.js";

let state = null; // { row, vehicle, a, b } — a/b are day strings

function rowCells(row) { return [...row.querySelectorAll(".pcal-cell")]; }

function clear(grid) {
  grid.querySelectorAll(".pcal-ghost").forEach((g) => g.remove());
  grid.querySelectorAll(".is-pick, .is-range").forEach((c) => c.classList.remove("is-pick", "is-range"));
  grid.querySelector(".pcal")?.classList.remove("is-dragging");
  state = null;
}

function paint(grid) {
  grid.querySelectorAll(".pcal-ghost").forEach((g) => g.remove());
  if (!state) return;
  const [s, e] = state.a <= state.b ? [state.a, state.b] : [state.b, state.a];
  const cells = rowCells(state.row).filter((c) => c.dataset.day >= s && c.dataset.day <= e);
  if (!cells.length) return;
  const first = cells[0], last = cells[cells.length - 1];
  const ghost = document.createElement("span");
  ghost.className = "pcal-ghost";
  ghost.style.left = `${first.offsetLeft + 4}px`;
  ghost.style.width = `${last.offsetLeft + last.offsetWidth - first.offsetLeft - 8}px`;
  ghost.textContent = cells.length === 1 ? "Block 1 day" : `Block ${cells.length} days`;
  state.row.appendChild(ghost);
}

async function reload(grid) {
  const res = await request(location.pathname + location.search, { method: "GET" });
  if (!res.ok) return;
  const doc = new DOMParser().parseFromString(await res.text(), "text/html");
  const fresh = doc.getElementById("pcal");
  if (fresh) {
    grid.replaceWith(fresh);
    document.dispatchEvent(new CustomEvent("saarthi:init", { detail: fresh }));
  }
}

function shake(code) {
  const bar = document.querySelector(`.pcal-bar[data-code="${code}"]`);
  if (!bar) return;
  bar.classList.remove("is-shaking");
  void bar.offsetWidth;
  bar.classList.add("is-shaking");
  bar.addEventListener("animationend", () => bar.classList.remove("is-shaking"), { once: true });
}

async function commit(grid) {
  if (!state) return;
  const [start, end] = state.a <= state.b ? [state.a, state.b] : [state.b, state.a];
  const vehicle = state.vehicle;
  const focusDay = state.b;
  const res = await request("/p/blocks", { method: "POST", body: new URLSearchParams({ vehicle, start, end, note: "Maintenance" }), accept: "application/json" });
  const data = await res.json().catch(() => ({}));
  if (res.status === 409) {
    clear(grid);
    shake(data.code);
    toast({ message: "Already booked", tone: "bad" });
    return;
  }
  clear(grid);
  if (!res.ok) { toast({ message: data.error || "Couldn't block those days.", tone: "bad" }); return; }
  await reload(grid);
  document.querySelector(`.pcal-cell[data-vehicle="${vehicle}"][data-day="${focusDay}"]`)?.focus();
  toast({ message: data.message, tone: "good", undo: { url: `/p/blocks/${data.id}`, method: "DELETE", reload: false } });
}

function setup(grid) {
  const cells = [...grid.querySelectorAll(".pcal-cell")];
  cells.forEach((c, i) => { c.tabIndex = i === 0 ? 0 : -1; });

  // remove a block
  grid.addEventListener("click", async (e) => {
    const block = e.target.closest("[data-block-id]");
    if (!block) return;
    const res = await request(`/p/blocks/${block.dataset.blockId}`, { method: "DELETE", accept: "application/json" });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) { toast({ message: data.error || "Couldn't remove that block.", tone: "bad" }); return; }
    await reload(grid);
    toast({ message: data.message, undo: { ...data.undo, reload: false } });
  });

  // mouse / pen drag
  grid.addEventListener("pointerdown", (e) => {
    const cell = e.target.closest(".pcal-cell");
    if (!cell || e.pointerType === "touch" || e.button !== 0) return;
    e.preventDefault();
    clear(grid);
    state = { row: cell.closest(".pcal-row"), vehicle: cell.dataset.vehicle, a: cell.dataset.day, b: cell.dataset.day, drag: true };
    grid.querySelector(".pcal").classList.add("is-dragging");
    paint(grid);
  });
  grid.addEventListener("pointermove", (e) => {
    if (!state?.drag) return;
    const el = document.elementFromPoint(e.clientX, e.clientY)?.closest(".pcal-cell");
    if (el && el.dataset.vehicle === state.vehicle && el.dataset.day !== state.b) {
      state.b = el.dataset.day;
      paint(grid);
    }
  });
  // touch: tap first day, then last
  grid.addEventListener("click", (e) => {
    const cell = e.target.closest(".pcal-cell");
    if (!cell || e.pointerType === "mouse" || state?.drag) return;
    if (!state || state.vehicle !== cell.dataset.vehicle) {
      clear(grid);
      state = { row: cell.closest(".pcal-row"), vehicle: cell.dataset.vehicle, a: cell.dataset.day, b: cell.dataset.day };
      cell.classList.add("is-pick");
      paint(grid);
      toast({ message: "Now tap the last day to block." });
      return;
    }
    state.b = cell.dataset.day;
    paint(grid);
    commit(grid);
  });

  // keyboard
  grid.addEventListener("keydown", (e) => {
    const cell = e.target.closest(".pcal-cell");
    if (!cell) return;
    const row = cell.closest(".pcal-row");
    const rows = [...grid.querySelectorAll(".pcal-row")];
    const rc = rowCells(row);
    const col = rc.indexOf(cell);
    let next = null;
    switch (e.key) {
      case "ArrowRight": next = rc[col + 1]; break;
      case "ArrowLeft": next = rc[col - 1]; break;
      case "ArrowDown": next = rows[rows.indexOf(row) + 1] && rowCells(rows[rows.indexOf(row) + 1])[col]; break;
      case "ArrowUp": next = rows[rows.indexOf(row) - 1] && rowCells(rows[rows.indexOf(row) - 1])[col]; break;
      case "Enter":
      case " ":
        e.preventDefault();
        if (!state || state.vehicle !== cell.dataset.vehicle) state = { row, vehicle: cell.dataset.vehicle, a: cell.dataset.day, b: cell.dataset.day };
        commit(grid);
        return;
      case "Escape":
        clear(grid);
        return;
      default:
        return;
    }
    if (!next) return;
    e.preventDefault();
    cell.tabIndex = -1;
    next.tabIndex = 0;
    next.focus();
    const sameRow = next.closest(".pcal-row") === row;
    if (e.shiftKey && sameRow) {
      if (!state || state.vehicle !== cell.dataset.vehicle) state = { row, vehicle: cell.dataset.vehicle, a: cell.dataset.day, b: cell.dataset.day };
      state.b = next.dataset.day;
      paint(grid);
    } else if (!e.shiftKey) {
      clear(grid);
    }
  });
}

export function initProviderCalendar(root) {
  for (const grid of all(root, "[data-pcal]")) {
    if (!once(grid)) continue;
    setup(grid);
    if (location.hash === "#block") grid.querySelector(".pcal-cell")?.focus();
  }
  if (!window.__pcalWired) {
    window.__pcalWired = true;
    window.addEventListener("pointerup", () => {
      const grid = document.querySelector("[data-pcal]");
      if (grid && state?.drag) commit(grid);
    });
    document.addEventListener("keydown", (e) => {
      const grid = document.querySelector("[data-pcal]");
      if (e.key === "Escape" && grid && state) clear(grid);
    });
    document.addEventListener("saarthi:undone", () => {
      const grid = document.querySelector("[data-pcal]");
      if (grid) reload(grid);
    });
  }
}
