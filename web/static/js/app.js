// Saarthi front-end entry. Each component is a small module wired through
// data-* attributes. init() runs on load, after every htmx swap, and after
// our own fetch swaps, so server-rendered fragments come alive the same way.
import { initThemeSwitch } from "./theme.js";
import { initToasts } from "./toast.js";
import { initLoader } from "./loader.js";
import { initSheets } from "./sheet.js";
import { initOdometers } from "./odometer.js";
import { initComposer } from "./composer.js";
import { initDatePickers } from "./daterange.js";
import { initCompare } from "./compare.js";
import { initMaps } from "./map.js";
import { initStatusLines } from "./statusline.js";
import { initDeposits } from "./deposit.js";
import { initUploads } from "./upload.js";
import { initCamera } from "./camera.js";
import { initSwipes } from "./swipe.js";
import { initPass } from "./pass.js";
import { initProviderCalendar } from "./pcal.js";
import { initEditor } from "./editor.js";
import { initInspect } from "./inspect.js";
import { initGallery } from "./gallery.js";
import { initWipes } from "./wipe.js";
import { initHelp } from "./help.js";
import { initCharts } from "./chart.js";
import { initPalette } from "./palette.js";
import { initForms } from "./forms.js";
import { initPWA } from "./pwa.js";
import { initMisc } from "./misc.js";
import { initStyleguide } from "./styleguide.js";

const components = [
  initThemeSwitch, initToasts, initLoader, initSheets, initOdometers, initComposer, initDatePickers,
  initCompare, initMaps, initStatusLines, initDeposits, initUploads, initCamera, initSwipes, initPass,
  initProviderCalendar, initEditor, initInspect, initGallery, initWipes, initHelp, initCharts,
  initPalette, initForms, initPWA, initMisc, initStyleguide,
];

function init(root) {
  if (!root || !root.querySelectorAll) return;
  for (const c of components) {
    try {
      c(root);
    } catch (err) {
      console.error("[saarthi]", c.name, err);
    }
  }
}

function start() {
  init(document);
  document.addEventListener("htmx:load", (e) => init(e.target));
  document.addEventListener("saarthi:init", (e) => init(e.detail));
}

// htmx is a classic deferred script placed before this module, so it has
// run by now. Motion and Leaflet load on demand (see motion.js / map.js).
start();
