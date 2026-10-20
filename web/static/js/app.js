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
import { initSwipes } from "./swipe.js";
import { initPass } from "./pass.js";
import { initProviderCalendar } from "./pcal.js";
import { initEditor } from "./editor.js";
import { initInspect } from "./inspect.js";
import { initMisc } from "./misc.js";
import { initStyleguide } from "./styleguide.js";

const components = [
  initThemeSwitch, initToasts, initLoader, initSheets, initOdometers, initComposer, initDatePickers,
  initCompare, initMaps, initStatusLines, initDeposits, initUploads, initSwipes, initPass,
  initProviderCalendar, initEditor, initInspect, initMisc, initStyleguide,
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

// htmx, Motion and Leaflet are classic deferred scripts placed before this
// module, so they have run by now.
start();
