// Compare tray: holds up to 3. Adding one shrinks the card's picture and
// arcs it into the tray; the counter bumps on a spring. A 4th asks which
// one to replace.
import { all, once, play, bump, request, swapHTML, DUR, ENTER } from "./motion.js";
import { toast } from "./toast.js";

function setPressed(ids) {
  for (const b of document.querySelectorAll("[data-compare]")) {
    b.setAttribute("aria-pressed", String(ids.includes(b.dataset.compare)));
  }
}

async function flyToTray(fromEl, id) {
  const tray = document.getElementById("compare-tray");
  const target = tray?.querySelector(`[data-tray-id="${id}"]`);
  if (!fromEl || !target) return;
  const a = fromEl.getBoundingClientRect(), b = target.getBoundingClientRect();
  const ghost = fromEl.cloneNode(true);
  ghost.removeAttribute("style");
  ghost.className = `fly-thumb ${[...fromEl.classList].filter((c) => c.startsWith("tint-")).join(" ")}`;
  Object.assign(ghost.style, { left: `${a.left}px`, top: `${a.top}px`, width: `${a.width}px`, height: `${a.height}px` });
  ghost.querySelectorAll("form, .vcard-badge").forEach((n) => n.remove());
  document.body.appendChild(ghost);
  target.style.opacity = "0";
  const s = b.width / a.width;
  const dx = b.left - a.left - (a.width - b.width) / 2, dy = b.top - a.top - (a.height - b.height) / 2;
  await play(ghost, [
    { transform: "translate(0,0) scale(1)", opacity: 1, borderRadius: "16px" },
    { transform: `translate(${dx * 0.55}px, ${dy * 0.35 - 90}px) scale(${(1 + s) / 2.4})`, opacity: 1, offset: 0.5 },
    { transform: `translate(${dx}px, ${dy}px) scale(${s})`, opacity: 0.6, borderRadius: "50%" },
  ], { duration: DUR.hero, easing: ENTER });
  ghost.remove();
  target.style.opacity = "";
  bump(tray.querySelector("[data-tray-count]"));
}

async function submit(form) {
  const btn = form.querySelector("button");
  const id = form.action.match(/\/compare\/([0-9a-f]{32})/)?.[1];
  const removing = form.matches("[data-compare-remove]") || btn?.getAttribute("aria-pressed") === "true";
  const url = removing ? `/compare/${id}` : form.action;
  const res = await request(url, { method: removing ? "DELETE" : "POST" });
  const html = await res.text();
  if (res.status === 409) {
    const root = document.getElementById("sheet-root");
    root.innerHTML = html;
    document.dispatchEvent(new CustomEvent("saarthi:init", { detail: root }));
    return;
  }
  if (!res.ok) { toast({ message: "Couldn't update compare. Try again.", tone: "bad" }); return; }
  const ids = JSON.parse(res.headers.get("X-Compare-IDs") || "[]");
  const old = document.getElementById("compare-tray");
  if (old) swapHTML(old, html);
  setPressed(ids);
  // close the replace dialog if it was open
  document.querySelector("#compare-full[open] [data-sheet-close]")?.click();
  if (form.matches("[data-compare-remove]")) {
    const col = form.closest("td");
    if (col && ids.length) {
      // drop that column from the open compare table
      const idx = [...col.parentElement.children].indexOf(col);
      for (const tr of col.closest("table").rows) tr.cells[idx]?.remove();
    } else {
      document.querySelector("#compare-sheet[open] [data-sheet-close]")?.click();
      if (location.pathname === "/compare") location.reload();
    }
    return;
  }
  if (!removing) {
    const media = form.closest(".vcard")?.querySelector(".vcard-media") || form.closest(".gallery-hero");
    await flyToTray(media, id);
  }
}

export function initCompare(root) {
  for (const form of all(root, "form[data-compare-form], form[data-compare-remove]")) {
    if (!once(form, "cmpReady")) continue;
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      submit(form).catch(() => form.submit());
    });
  }
  const tray = document.getElementById("compare-tray");
  if (tray && once(tray, "trayReady") && !tray.hidden && root === document) {
    play(tray, [{ transform: "translate(-50%, 20px)", opacity: 0 }, { transform: "translate(-50%, 0)", opacity: 1 }], { duration: DUR.slow, easing: ENTER });
  }
}
