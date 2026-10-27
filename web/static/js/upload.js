// Proof photo uploads (check-in / return) and vehicle photo uploads.
// Each uploaded angle fills its zone of the silhouette in teal; when every
// required shot (and the odometer reading) is in, the swipe unlocks.
import { all, once, play, request, swapHTML, esc, DUR, ENTER } from "./motion.js";
import { toast } from "./toast.js";
import { quality, qualityMessage } from "./camera.js";

const MAX = 8 * 1024 * 1024;
const SHRINK_OVER = 3.5 * 1024 * 1024; // hosted demo (Vercel) refuses bodies over 4.5 MB

function checkin() { return document.querySelector("[data-checkin]"); }

// shrink re-encodes a big photo to 1600 px JPEG before upload; the server
// resizes to that size anyway, so nothing is lost.
async function shrink(file) {
  if (file.size <= SHRINK_OVER || !window.createImageBitmap) return file;
  try {
    const bmp = await createImageBitmap(file);
    const s = Math.min(1, 1600 / Math.max(bmp.width, bmp.height));
    const c = document.createElement("canvas");
    c.width = Math.round(bmp.width * s);
    c.height = Math.round(bmp.height * s);
    c.getContext("2d").drawImage(bmp, 0, 0, c.width, c.height);
    bmp.close?.();
    const blob = await new Promise((resolve) => c.toBlob(resolve, "image/jpeg", 0.88));
    return blob || file;
  } catch {
    return file;
  }
}

export function refreshCheckin() {
  const box = checkin();
  if (!box) return;
  const tiles = [...box.querySelectorAll(".angle")];
  for (const t of tiles) {
    document.querySelector(`[data-silhouette] [data-zone="${t.dataset.angle}"]`)?.classList.toggle("is-filled", t.classList.contains("is-done"));
  }
  const missing = tiles.find((t) => t.hasAttribute("data-required") && !t.classList.contains("is-done"));
  const odo = box.querySelector("[data-odometer]");
  const odoOK = !odo || (odo.value !== "" && odo.checkValidity());
  const swipe = box.querySelector("[data-swipe]");
  const hint = box.querySelector("[data-checkin-hint]");
  const done = tiles.filter((t) => t.classList.contains("is-done")).length;
  if (swipe) {
    swipe.setAttribute("aria-disabled", String(!!missing || !odoOK));
    swipe.dataset.hint = missing ? missing.dataset.hint : (!odoOK ? "Add the odometer reading first." : "");
  }
  if (hint && !hint.dataset.sticky) {
    hint.hidden = !(missing && done > 0);
    if (missing) hint.textContent = missing.dataset.hint;
  }
  const label = box.querySelector("[data-camera-label]");
  if (label && done > 0) label.textContent = missing ? `Continue the walk-around (${done} of ${tiles.length})` : "Retake a photo";
}

// uploadShot sends one photo (from the camera or the picker) and swaps in
// the server's tile. Resolves true on success.
export async function uploadShot(li, blob) {
  const box = checkin();
  li.classList.add("is-uploading");
  li.classList.remove("is-error");
  const fd = new FormData();
  fd.append("phase", box.dataset.phase);
  fd.append("angle", li.dataset.angle);
  fd.append("photo", blob, `${li.dataset.angle}.jpg`);
  try {
    const res = await request(box.dataset.upload, { method: "POST", body: fd });
    const text = await res.text();
    if (!res.ok) throw new Error(text || "Upload failed. Try again.");
    const fresh = swapHTML(li, text);
    if (fresh) play(fresh.querySelector(".angle-check"), [{ transform: "scale(0.4)", opacity: 0 }, { transform: "scale(1)", opacity: 1 }], { duration: DUR.slow, easing: ENTER });
    return true;
  } catch (err) {
    li.classList.remove("is-uploading");
    li.classList.add("is-error");
    toast({ message: String(err.message || err).slice(0, 160), tone: "bad" });
    return false;
  } finally {
    refreshCheckin();
  }
}

// A picked file gets the same too-dark / blurry check as camera frames.
async function checkFile(file) {
  try {
    const bmp = await createImageBitmap(file);
    const q = quality(bmp, bmp.width, bmp.height);
    bmp.close?.();
    return qualityMessage(q);
  } catch {
    return "";
  }
}

async function pickAngle(input) {
  const file = input.files?.[0];
  if (!file) return;
  const li = input.closest(".angle");
  const thumb = li.querySelector(".angle-thumb");
  const preview = URL.createObjectURL(file);
  const before = thumb.innerHTML;
  thumb.innerHTML = `<img src="${preview}" alt="">`;
  const msg = await checkFile(file);
  if (msg) {
    const hint = checkin().querySelector("[data-checkin-hint]");
    hint.hidden = false;
    hint.dataset.sticky = "1";
    hint.innerHTML = `${esc(msg)} <span class="row-wrap" style="margin-top:8px"><button class="btn btn-secondary btn-sm" type="button" data-retake>Retake</button><button class="btn btn-ghost btn-sm" type="button" data-anyway>Use anyway</button></span>`;
    const choice = await new Promise((resolve) => {
      hint.querySelector("[data-retake]").addEventListener("click", () => resolve("retake"), { once: true });
      hint.querySelector("[data-anyway]").addEventListener("click", () => resolve("use"), { once: true });
    });
    delete hint.dataset.sticky;
    hint.hidden = true;
    if (choice === "retake") {
      thumb.innerHTML = before;
      URL.revokeObjectURL(preview);
      input.value = "";
      input.click();
      return;
    }
  }
  const body = await shrink(file);
  if (body.size > MAX) {
    thumb.innerHTML = before;
    URL.revokeObjectURL(preview);
    input.value = "";
    toast({ message: "That photo is over 8 MB. Try again with a smaller one.", tone: "bad" });
    return;
  }
  await uploadShot(li, body);
  URL.revokeObjectURL(preview);
}

async function uploadVehiclePhotos(input) {
  const files = [...(input.files || [])];
  let added = 0;
  for (const f of files) {
    const body = await shrink(f);
    if (body.size > MAX) { toast({ message: `${f.name} is over 8 MB.`, tone: "bad" }); continue; }
    const fd = new FormData();
    fd.append("photo", body, f.name);
    const res = await request(input.dataset.photoUpload, { method: "POST", body: fd });
    const text = await res.text();
    if (!res.ok) { toast({ message: text.slice(0, 160), tone: "bad" }); continue; }
    swapHTML(document.getElementById("photo-manage"), text);
    added++;
  }
  document.dispatchEvent(new CustomEvent("saarthi:photos"));
  if (added) toast({ message: added > 1 ? `${added} photos added.` : "Photo added.", tone: "good" });
}

export function initUploads(root) {
  for (const input of all(root, "input[type=file][data-upload]")) {
    if (!once(input)) continue;
    input.addEventListener("change", () => pickAngle(input));
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
