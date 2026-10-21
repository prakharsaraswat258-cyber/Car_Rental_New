// Offline Pickup Pass: register the service worker, show "saved on this
// phone" on passes once it's in control, and forget saved passes after
// logging out.
export function initPWA() {
  if (window.__pwa || !("serviceWorker" in navigator)) return;
  window.__pwa = true;
  navigator.serviceWorker.register("/sw.js", { scope: "/" }).then(() => navigator.serviceWorker.ready).then((reg) => {
    if (!document.body.dataset.auth) reg.active?.postMessage("logged-out");
    const note = document.querySelector("[data-offline-note]");
    if (note && navigator.serviceWorker.controller) note.hidden = false;
  }).catch(() => { /* offline support is a bonus, never a blocker */ });
}
