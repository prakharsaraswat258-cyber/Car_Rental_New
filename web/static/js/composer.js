// Ask Saarthi composer.
//  - placeholder cycles through examples every 4 s
//  - on send, POST /ask (JSON); the matched words lift out of the text and
//    fly (FLIP) into the chips row, then we navigate to the results, where
//    the composer + chips morph via cross-document View Transitions
//  - missing place/dates → ask with quick chips instead of navigating
import { all, once, reduced, play, esc, icon, request, DUR, ENTER, STAGGER } from "./motion.js";

const KIND_ICON = { place: "map-pin", dates: "calendar", type: "gauge", budget: "wallet", sort: "arrow-up-down" };

function cycle(form, input, ghost, examples) {
  if (examples.length < 2) return;
  let i = 0;
  setInterval(() => {
    if (input.value || document.hidden) return;
    i = (i + 1) % examples.length;
    if (reduced()) { ghost.textContent = examples[i]; return; }
    ghost.classList.add("is-out");
    setTimeout(() => {
      ghost.textContent = examples[i];
      ghost.classList.remove("is-out");
      ghost.classList.add("is-in");
      requestAnimationFrame(() => requestAnimationFrame(() => ghost.classList.remove("is-in")));
    }, 400);
  }, 4000);
}

// measure returns the viewport box of text[start:end] inside the input.
function measure(input, start, end) {
  const cs = getComputedStyle(input);
  const probe = document.createElement("span");
  probe.style.cssText = `position:fixed;visibility:hidden;white-space:pre;font:${cs.font};letter-spacing:${cs.letterSpacing}`;
  document.body.appendChild(probe);
  probe.textContent = input.value.slice(0, start);
  const x0 = probe.getBoundingClientRect().width;
  probe.textContent = input.value.slice(start, end);
  const w = probe.getBoundingClientRect().width;
  probe.remove();
  const r = input.getBoundingClientRect();
  const left = r.left + (parseFloat(cs.paddingLeft) || 0) + x0 - input.scrollLeft;
  const lh = parseFloat(cs.lineHeight);
  const lineH = Number.isFinite(lh) ? lh : (parseFloat(cs.fontSize) || 16) * 1.25;
  return { left, top: r.top + (r.height - lineH) / 2, width: w, visible: left >= r.left - 4 && left + w <= r.right + 4 };
}

function chipHTML(c) {
  return `<span class="chip chip-ask" data-kind="${esc(c.kind)}">${icon(KIND_ICON[c.kind] || "check")}${esc(c.label)}` +
    (c.assumed ? `<span class="chip-assumed">1 day, assumed</span>` : "") + `</span>`;
}

async function fly(input, row, chips) {
  row.innerHTML = chips.map(chipHTML).join("");
  const chipEls = [...row.children];
  if (reduced()) {
    await Promise.all(chipEls.map((c) => play(c, [{ opacity: 0 }, { opacity: 1 }])));
    return;
  }
  chipEls.forEach((c) => { c.style.opacity = "0"; });
  const flights = [];
  let n = 0;
  chips.forEach((c, ci) => {
    const target = chipEls[ci].getBoundingClientRect();
    for (const span of c.spans || []) {
      const box = measure(input, span.start, span.end);
      if (!box.visible || box.width < 2) continue;
      const word = document.createElement("span");
      word.className = "fly-word";
      word.textContent = input.value.slice(span.start, span.end);
      word.style.left = `${box.left}px`;
      word.style.top = `${box.top}px`;
      document.body.appendChild(word);
      const dx = target.left + 14 - box.left, dy = target.top + 6 - box.top;
      const delay = Math.min(n++, 8) * STAGGER;
      flights.push(play(word, [
        { transform: "translate(0,0) scale(1)", opacity: 1 },
        { transform: `translate(${dx * 0.5}px, ${dy * 0.5 - 28}px) scale(1.06)`, opacity: 1, offset: 0.45 },
        { transform: `translate(${dx}px, ${dy}px) scale(0.9)`, opacity: 0 },
      ], { duration: DUR.hero, easing: ENTER, delay, fill: "forwards" }).then(() => word.remove()));
    }
    const delay = Math.min(ci, 8) * STAGGER + DUR.hero * 0.55;
    flights.push(play(chipEls[ci], [
      { opacity: 0, transform: "translateY(6px) scale(0.92)" },
      { opacity: 1, transform: "none" },
    ], { duration: DUR.slow, easing: ENTER, delay, fill: "forwards" }).then(() => { chipEls[ci].style.opacity = ""; }));
  });
  input.animate?.([{ opacity: 1 }, { opacity: 0.35, offset: 0.3 }, { opacity: 1 }], { duration: DUR.hero + 200 });
  await Promise.all(flights);
}

function showMissing(box, reply, input, form) {
  if (!box || !reply.prompt) return;
  box.hidden = false;
  box.querySelector("[data-ask-title]").textContent = reply.prompt.title;
  const opts = box.querySelector("[data-ask-options]");
  opts.innerHTML = "";
  for (const o of reply.prompt.options) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "chip";
    b.textContent = o.label;
    b.addEventListener("click", () => {
      input.value = `${input.value.trim()} ${o.append}`.trim();
      form.requestSubmit();
    });
    opts.appendChild(b);
  }
  const any = document.createElement("a");
  any.className = "btn btn-ghost btn-sm";
  any.href = reply.url;
  any.textContent = "Show me everything";
  opts.appendChild(any);
  play(box, [{ opacity: 0, transform: "translateY(8px)" }, { opacity: 1, transform: "none" }], { duration: DUR.slow, easing: ENTER });
}

export function initComposer(root) {
  for (const form of all(root, "[data-composer]")) {
    if (!once(form)) continue;
    const input = form.querySelector("input[name=q]");
    const ghost = form.querySelector(".composer-ghost");
    let examples = [];
    try { examples = JSON.parse(form.dataset.examples || "[]"); } catch { /* none */ }
    if (ghost) cycle(form, input, ghost, examples);
    const scope = form.parentElement;
    const row = scope.querySelector("[data-ask-chips]");
    const missing = scope.querySelector("[data-ask-missing]");
    const btn = form.querySelector("button[type=submit]");

    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const text = input.value.trim();
      if (!text) {
        input.focus();
        play(form, [{ transform: "translateX(0)" }, { transform: "translateX(-6px)" }, { transform: "translateX(6px)" }, { transform: "translateX(0)" }], { duration: 300 });
        return;
      }
      btn?.classList.add("is-loading");
      let reply;
      try {
        const res = await request("/ask", { method: "POST", body: new URLSearchParams({ q: text }), accept: "application/json" });
        reply = await res.json();
      } catch {
        btn?.classList.remove("is-loading");
        form.submit(); // no-JS path still works
        return;
      }
      btn?.classList.remove("is-loading");
      if (missing) missing.hidden = true;
      if (row) await fly(input, row, reply.chips);
      if (reply.prompt) {
        showMissing(missing, reply, input, form);
        return;
      }
      if (form.hasAttribute("data-demo-only")) return;
      location.assign(reply.url);
    });
  }
  for (const b of all(root, "[data-ask-insert]")) {
    if (!once(b)) continue;
    b.addEventListener("click", () => {
      const input = document.querySelector("[data-composer] input[name=q]");
      if (!input) return;
      const word = b.dataset.askInsert;
      if (!input.value.toLowerCase().includes(word.toLowerCase())) input.value = `${word} ${input.value}`.trim() + " ";
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    });
  }
}
