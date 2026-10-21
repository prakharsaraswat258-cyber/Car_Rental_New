// Guided check-in camera: full-screen rear camera (getUserMedia) with an
// original silhouette per angle. Each frame is checked on a small canvas:
// too dark (mean < 60/255) or blurry (low Laplacian variance) asks for a
// retake. Without camera access, each tile opens the photo picker instead.
import { all, once, play, vibrate, DUR, ENTER, reduced } from "./motion.js";
import { toast } from "./toast.js";
import { uploadShot, refreshCheckin } from "./upload.js";

const DARK = 60;
const BLUR = 35;

// quality samples a frame at 160 px wide: mean brightness and the
// variance of the Laplacian (low variance = soft, blurry image).
export function quality(source, w, h) {
  const sw = 160, sh = Math.max(1, Math.round((h / w) * 160));
  const c = document.createElement("canvas");
  c.width = sw; c.height = sh;
  const ctx = c.getContext("2d", { willReadFrequently: true });
  ctx.drawImage(source, 0, 0, sw, sh);
  const px = ctx.getImageData(0, 0, sw, sh).data;
  const g = new Float32Array(sw * sh);
  let sum = 0;
  for (let i = 0, j = 0; i < px.length; i += 4, j++) {
    g[j] = 0.299 * px[i] + 0.587 * px[i + 1] + 0.114 * px[i + 2];
    sum += g[j];
  }
  const mean = sum / g.length;
  let n = 0, m = 0, m2 = 0;
  for (let y = 1; y < sh - 1; y++) {
    for (let x = 1; x < sw - 1; x++) {
      const k = y * sw + x;
      const lap = 4 * g[k] - g[k - 1] - g[k + 1] - g[k - sw] - g[k + sw];
      n++;
      const d = lap - m;
      m += d / n;
      m2 += d * (lap - m);
    }
  }
  const variance = n > 1 ? m2 / (n - 1) : 0;
  return { dark: mean < DARK, blurry: variance < BLUR, mean, variance };
}

export function qualityMessage(q) {
  if (q.dark) return "Too dark to see scratches. Try again near light.";
  if (q.blurry) return "A little blurry. Hold still and try again.";
  return "";
}

class Camera {
  constructor(box, cam) {
    this.box = box;
    this.cam = cam;
    this.video = cam.querySelector("[data-cam-video]");
    this.review = cam.querySelector("[data-cam-review]");
    this.guide = cam.querySelector("[data-cam-guide]");
    this.tip = cam.querySelector("[data-cam-tip]");
    this.confirm = cam.querySelector("[data-cam-confirm]");
    this.shootBar = cam.querySelector("[data-cam-shoot]").parentElement;
    this.anyway = cam.querySelector("[data-cam-anyway]");
    this.stream = null;
    this.blob = null;
    this.i = 0;
    cam.querySelector("[data-cam-close]").addEventListener("click", () => this.close());
    cam.querySelector("[data-cam-shoot]").addEventListener("click", () => this.shoot());
    cam.querySelector("[data-cam-retake]").addEventListener("click", () => this.retake());
    cam.querySelector("[data-cam-use]").addEventListener("click", () => this.use());
    this.anyway.addEventListener("click", () => this.use());
    cam.querySelector("[data-cam-skip]").addEventListener("click", () => this.next(1));
    cam.addEventListener("keydown", (e) => this.keys(e));
  }

  tiles() { return [...this.box.querySelectorAll(".angle")]; }

  async open() {
    this.trigger = document.activeElement;
    const tiles = this.tiles();
    const missing = tiles.findIndex((t) => !t.classList.contains("is-done"));
    this.i = missing < 0 ? 0 : missing;
    if (!navigator.mediaDevices?.getUserMedia) return this.fallback();
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: { ideal: "environment" }, width: { ideal: 1920 }, height: { ideal: 1440 } }, audio: false,
      });
    } catch {
      return this.fallback();
    }
    this.video.srcObject = this.stream;
    this.cam.hidden = false;
    document.body.classList.add("has-modal");
    play(this.cam, [{ opacity: 0 }, { opacity: 1 }], { duration: DUR.base });
    this.show();
    this.cam.querySelector("[data-cam-shoot]").focus();
  }

  fallback() {
    this.box.querySelector("[data-camera-fallback]").hidden = false;
    toast({ message: "The camera isn't available here, so pick photos instead.", tone: "warn" });
    this.tiles()[this.i]?.querySelector("input[type=file]")?.click();
  }

  show() {
    const t = this.tiles()[this.i];
    if (!t) return;
    const total = this.tiles().length;
    this.cam.querySelector("[data-cam-angle]").textContent = t.dataset.label;
    this.cam.querySelector("[data-cam-progress]").textContent = `${this.i + 1} of ${total}`;
    const tpl = document.querySelector(`template[data-guide-art="${t.dataset.guide}"]`);
    this.guide.replaceChildren(tpl ? tpl.content.cloneNode(true) : "");
    this.guide.classList.toggle("is-mirror", t.hasAttribute("data-mirror"));
    this.guide.classList.toggle("is-turn", t.hasAttribute("data-turn"));
    const gauge = t.dataset.guide === "gauge";
    this.say(gauge ? `Fill the frame with the ${t.dataset.label.toLowerCase()}.` : `Step back until the ${t.dataset.label.toLowerCase()} fits the outline.`);
    this.mode("shoot");
    if (!reduced()) play(this.guide, [{ opacity: 0, transform: "scale(0.96)" }, { opacity: 1, transform: "none" }], { duration: DUR.slow, easing: ENTER });
  }

  say(text, warn = false) {
    this.tip.textContent = text;
    this.tip.classList.toggle("is-warn", warn);
  }

  mode(m) {
    const shooting = m === "shoot";
    this.shootBar.hidden = !shooting;
    this.confirm.hidden = shooting;
    this.review.hidden = shooting;
    this.video.hidden = !shooting;
    this.guide.hidden = !shooting;
  }

  shoot() {
    const v = this.video;
    if (!v.videoWidth) return;
    const scale = Math.min(1, 1600 / Math.max(v.videoWidth, v.videoHeight));
    const c = this.cam.querySelector("[data-cam-canvas]");
    c.width = Math.round(v.videoWidth * scale);
    c.height = Math.round(v.videoHeight * scale);
    c.getContext("2d").drawImage(v, 0, 0, c.width, c.height);
    vibrate(10);
    play(this.cam, [{ opacity: 0.4 }, { opacity: 1 }], { duration: 160 });
    const q = quality(c, c.width, c.height);
    c.toBlob((blob) => {
      this.blob = blob;
      if (this.reviewURL) URL.revokeObjectURL(this.reviewURL);
      this.reviewURL = URL.createObjectURL(blob);
      this.review.src = this.reviewURL;
      this.mode("review");
      const msg = qualityMessage(q);
      this.anyway.hidden = !msg;
      if (msg) {
        this.say(msg, true);
        this.cam.querySelector("[data-cam-retake]").focus();
      } else {
        this.say("Looks clear. Use it?");
        this.cam.querySelector("[data-cam-use]").focus();
      }
    }, "image/jpeg", 0.88);
  }

  retake() {
    this.blob = null;
    this.show();
    this.cam.querySelector("[data-cam-shoot]").focus();
  }

  async use() {
    if (!this.blob) return;
    const btn = this.cam.querySelector("[data-cam-use]");
    btn.classList.add("is-loading");
    const ok = await uploadShot(this.tiles()[this.i], this.blob);
    btn.classList.remove("is-loading");
    if (!ok) { this.retake(); return; }
    this.next(1);
  }

  next(step) {
    const tiles = this.tiles();
    for (let k = 1; k <= tiles.length; k++) {
      const j = (this.i + step * k + tiles.length) % tiles.length;
      if (!tiles[j].classList.contains("is-done")) {
        this.i = j;
        this.show();
        return;
      }
    }
    this.close();
    toast({ message: "All photos are in. Add the odometer reading and swipe.", tone: "good" });
    refreshCheckin();
  }

  close() {
    this.stream?.getTracks().forEach((t) => t.stop());
    this.stream = null;
    this.cam.hidden = true;
    document.body.classList.remove("has-modal");
    const done = this.tiles().every((t) => t.classList.contains("is-done"));
    this.box.querySelector("[data-camera-label]").textContent = done ? "Retake a photo" : "Continue the walk-around";
    (this.trigger || this.box.querySelector("[data-camera-open]"))?.focus();
  }

  keys(e) {
    if (e.key === "Escape") { e.preventDefault(); this.close(); return; }
    if (e.key !== "Tab") return;
    const f = [...this.cam.querySelectorAll("button:not([hidden])")].filter((b) => b.offsetParent !== null);
    if (!f.length) return;
    const first = f[0], last = f[f.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  }
}

export function initCamera(root) {
  for (const box of all(root, "[data-checkin]")) {
    if (!once(box, "cameraReady")) continue;
    const cam = document.querySelector("[data-camera]");
    const btn = box.querySelector("[data-camera-open]");
    if (!cam || !btn) continue;
    const camera = new Camera(box, cam);
    if (!navigator.mediaDevices?.getUserMedia || !window.isSecureContext) {
      box.querySelector("[data-camera-fallback]").hidden = false;
    }
    btn.addEventListener("click", () => camera.open());
  }
}
