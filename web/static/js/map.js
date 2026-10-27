// Leaflet map with price pins. Tiles follow the theme (CARTO light/dark).
// Pins and cards highlight each other on hover. Search results swapped by
// htmx bring fresh pins.
import { all, once, esc, loadScript, loadCSS } from "./motion.js";
import { isDark } from "./theme.js";

const TILES = {
  light: "https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png",
  dark: "https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png",
};
const ATTR = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>';

function readPins(el) {
  const src = el.dataset.pinsFrom ? document.querySelector(`${el.dataset.pinsFrom} [data-pin-list]`) : el;
  const raw = src?.dataset.pinList ?? src?.dataset.pins ?? "[]";
  try { return JSON.parse(raw); } catch { return []; }
}

function setup(el) {
  const L = window.L;
  const isStatic = el.hasAttribute("data-static");
  const map = L.map(el, {
    zoomControl: !isStatic, scrollWheelZoom: false, attributionControl: true,
    dragging: !isStatic || !matchMedia("(pointer: coarse)").matches, tap: false,
  });
  let tiles;
  const setTiles = () => {
    tiles?.remove();
    tiles = L.tileLayer(isDark() ? TILES.dark : TILES.light, { attribution: ATTR, subdomains: "abcd", maxZoom: 19 }).addTo(map);
  };
  setTiles();
  document.addEventListener("saarthi:theme", setTiles);

  const layer = L.layerGroup().addTo(map);
  const draw = () => {
    layer.clearLayers();
    const pins = readPins(el);
    const pts = [];
    for (const p of pins) {
      const icon = L.divIcon({ className: "", iconSize: null, html: `<span class="pin" data-pin="${esc(p.id)}">${esc(p.label)}</span>` });
      const m = L.marker([p.lat, p.lng], { icon, keyboard: !isStatic, title: p.label }).addTo(layer);
      if (p.href && !isStatic) m.on("click", () => { location.href = p.href; });
      pts.push([p.lat, p.lng]);
    }
    if (pts.length === 1) map.setView(pts[0], isStatic ? 15 : 14);
    else if (pts.length) map.fitBounds(pts, { padding: [40, 40], maxZoom: 15 });
    else map.setView([20.5, 78.9], 4);
  };
  draw();
  el._redraw = draw;
  el._map = map;

  // card ⇄ pin hover
  el.addEventListener("mouseover", (e) => {
    const pin = e.target.closest("[data-pin]");
    if (!pin) return;
    document.querySelector(`.vcard[data-vehicle="${pin.dataset.pin}"]`)?.classList.add("is-hot");
  });
  el.addEventListener("mouseout", (e) => {
    const pin = e.target.closest("[data-pin]");
    if (pin) document.querySelector(`.vcard[data-vehicle="${pin.dataset.pin}"]`)?.classList.remove("is-hot");
  });
}

let wired = false;

// Leaflet is only fetched on pages that have a map (performance budget).
function leaflet() {
  if (window.L) return Promise.resolve(window.L);
  const b = document.body.dataset;
  return Promise.all([loadCSS(b.leafletCss), loadScript(b.leafletJs)]).then(() => window.L);
}

export function initMaps(root) {
  const maps = all(root, "[data-map]").filter((el) => !el.dataset.mapReady);
  if (!maps.length) return;
  leaflet().then(() => {
    for (const el of maps) {
      if (!once(el, "mapReady")) continue;
      // maps inside hidden containers wait until shown
      if (el.offsetParent === null && !el.closest(".map-wrap")) { delete el.dataset.mapReady; continue; }
      setup(el);
    }
  }).catch(() => maps.forEach((el) => { el.textContent = "The map couldn't load. The list still works."; }));
  if (!wired) {
    wired = true;
    document.addEventListener("htmx:afterSwap", (e) => {
      if (e.target.id !== "results") return;
      document.querySelectorAll("[data-map][data-pins-from]").forEach((m) => m._redraw?.());
    });
    document.addEventListener("mouseover", (e) => {
      const card = e.target.closest?.(".vcard[data-vehicle]");
      document.querySelectorAll(".pin.is-hot").forEach((p) => { if (!card || p.dataset.pin !== card.dataset.vehicle) p.classList.remove("is-hot"); });
      if (card) document.querySelector(`.pin[data-pin="${card.dataset.vehicle}"]`)?.classList.add("is-hot");
    });
  }
}

export function refreshMaps() {
  document.querySelectorAll("[data-map]").forEach((m) => m._map?.invalidateSize());
}
