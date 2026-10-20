// Proof photo uploads (check-in / return) and vehicle photo uploads.
// Each uploaded angle fills its part of the silhouette in teal; when all
// four required angles (and the odometer reading) are in, the swipe unlocks.
import { all, once, play, request, swapHTML, DUR, ENTER } from "./motion.js";
import { toast } from "./toast.js";

const MAX = 8 * 1024 * 1024;

function checkin() { return document.querySelector("[data-checkin]"); }

export function refreshCheckin() {
  const box = checkin();
  if (!box) return;
  const tiles = [...box.querySelectorAll(".angle")];
  for (const t of tiles) {
    const zone = document.querySelector(`[data-silhouette] [data-zone="${t.dataset.angle}"]`);
    zone?.classList.toggle("is-filled", t.classList.contains("is-done"));
  }
  const missing = tiles.find((t) => t.hasAttribute("data-required") && !t.classList.contains("is-done"));
  const odo = box.querySelector("[data-odometer]");
  const odoOK = !odo || (odo.value !== "" && odo.checkValidity());
  const swipe = box.querySelector("[data-swipe]");
  const hint = box.querySelector("[data-checkin-hint]");
  const done = tiles.filter((t) => t.hasAttribute("data-required") && t.classList.contains("is-done")).length;
  if (swipe) {
    swipe.setAttribute("aria-disabled", String(!!missing || !odoOK));
    swipe.dataset.hint = missing ? missing.dataset.hint : (!odoOK ? "Add the odometer reading first." : "");
  }
  if (hint) {
    if (missing && done > 0) {
      hint.hidden = false;
      hint.textContent = missing.dataset.hint;
    } else hint.hidden = true;
  }
}

async function uploadAngle(input) {
  const file = input.files?.[0];
  if (!file) return;
  const li = input.closest(".angle");
  if (file.size > MAX) { toast({ message: "That photo is over 8 MB. Try again with a smaller one.", tone: "bad" }); input.value = ""; return; }
  li.classList.add("is-uploading");
  li.classList.remove("is-error");
  const thumb = li.querySelector(".angle-thumb");
  const preview = URL.createObjectURL(file);
  thumb.innerHTML = `<img src="${preview}" alt="">`;
  const fd = new FormData();
  fd.append("phase", input.dataset.phase);
  fd.append("angle", input.dataset.angle);
  fd.append("photo", file);
  try {
    const res = await request(input.dataset.upload, { method: "POST", body: fd });
    const text = await res.text();
    if (!res.ok) throw new Error(text || "Upload failed. Try again.");
    const fresh = swapHTML(li, text);
    if (fresh) {
      play(fresh.querySelector(".angle-check"), [{ transform: "scale(0.4)", opacity: 0 }, { transform: "scale(1)", opacity: 1 }], { duration: DUR.slow, easing: ENTER });
    }
  } catch (err) {
    li.classList.remove("is-uploading");
    li.classList.add("is-error");
    toast({ message: String(err.message || err).slice(0, 160), tone: "bad" });
  } finally {
    URL.revokeObjectURL(preview);
    refreshCheckin();
  }
}

async function uploadVehiclePhotos(input) {
  const files = [...(input.files || [])];
  const grid = input.closest("#photo-manage");
  for (const f of files) {
    if (f.size > MAX) { toast({ message: `${f.name} is over 8 MB.`, tone: "bad" }); continue; }
    const fd = new FormData();
    fd.append("photo", f);
    const res = await request(input.dataset.photoUpload, { method: "POST", body: fd });
    const text = await res.text();
    if (!res.ok) { toast({ message: text.slice(0, 160), tone: "bad" }); continue; }
    const current = document.getElementById("photo-manage");
    swapHTML(current, text);
  }
  if (grid) document.dispatchEvent(new CustomEvent("saarthi:photos"));
  toast({ message: files.length > 1 ? "Photos added." : "Photo added.", tone: "good" });
}

export function initUploads(root) {
  for (const input of all(root, "input[type=file][data-upload]")) {
    if (!once(input)) continue;
    input.addEventListener("change", () => uploadAngle(input));
  }
  for (const input of all(root, "input[type=file][data-photo-upload]")) {
    if (!once(input)) continue;
    input.addEventListener("change", () => uploadVehiclePhotos(input));
  }
  for (const form of all(root, "form[data-photo-delete]")) {
    if (!once(form)) continue;
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const res = await request(form.action, { method: "POST" });
      if (res.ok) {
        swapHTML(document.getElementById("photo-manage"), await res.text());
        document.dispatchEvent(new CustomEvent("saarthi:photos"));
        toast({ message: "Photo removed." });
      }
    });
  }
  const box = checkin();
  if (box && once(box, "checkinReady")) {
    box.querySelector("[data-odometer]")?.addEventListener("input", refreshCheckin);
  }
  if (box) refreshCheckin();
}
