// Saarthi service worker (hand-written, no library).
//  - caches the app shell (CSS, core JS, icons, fonts, /offline) at install
//  - caches every Pickup Pass page you open, so it opens with no network
//  - versioned /static files: cache first; other /static files: network
//    first with a cached fallback; other pages: network, else /offline
// Served from /sw.js by the Go server, which fills in the version and the
// versioned shell URLs.
const VERSION = "__VERSION__";
const SHELL = "__SHELL__";
const SHELL_CACHE = `saarthi-shell-${VERSION}`;
const ASSETS = "saarthi-assets";
const PASSES = "saarthi-passes";

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k.startsWith("saarthi-shell-") && k !== SHELL_CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

const isPass = (url) => /^\/trips\/SAR-[0-9A-Z]{5}\/pass$/i.test(url.pathname);

async function passPage(req, url) {
  try {
    const res = await fetch(req);
    // a redirect means "log in first": never store the login page as a pass
    if (res.ok && !res.redirected && res.headers.get("Content-Type")?.includes("text/html")) {
      const cache = await caches.open(PASSES);
      await cache.put(url.pathname, res.clone());
    }
    return res;
  } catch {
    const cache = await caches.open(PASSES);
    return (await cache.match(url.pathname)) || (await caches.match("/offline")) || Response.error();
  }
}

async function page(req) {
  try {
    return await fetch(req);
  } catch {
    return (await caches.match("/offline")) || Response.error();
  }
}

async function asset(req, url) {
  if (url.searchParams.has("v")) {
    const hit = await caches.match(req);
    if (hit) return hit;
    const res = await fetch(req);
    if (res.ok) (await caches.open(ASSETS)).put(req, res.clone());
    return res;
  }
  try {
    const res = await fetch(req);
    if (res.ok) (await caches.open(ASSETS)).put(req, res.clone());
    return res;
  } catch {
    return (await caches.match(req)) || Response.error();
  }
}

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (req.mode === "navigate") {
    event.respondWith(isPass(url) ? passPage(req, url) : page(req));
    return;
  }
  if (url.pathname.startsWith("/static/")) event.respondWith(asset(req, url));
});

// Logged out: forget saved passes (they belong to the account).
self.addEventListener("message", (event) => {
  if (event.data === "logged-out") caches.delete(PASSES);
});
