// Forms: inline validation on blur with friendly copy under the field,
// focus the server's error summary, and stop double submits.
import { all, once } from "./motion.js";

function message(el) {
  const v = el.validity;
  const d = el.dataset;
  if (v.valueMissing) return d.msgRequired || "This one is needed.";
  if (v.typeMismatch) return d.msgType || (el.type === "email" ? "That email doesn't look right." : "That doesn't look right.");
  if (v.tooShort) return d.msgShort || `Use at least ${el.minLength} characters.`;
  if (v.rangeUnderflow) return d.msgMin || `It can't be below ${el.min}.`;
  if (v.rangeOverflow) return d.msgMax || `It can't be above ${el.max}.`;
  if (v.badInput) return "Use numbers only.";
  return el.validationMessage;
}

function errorEl(el) {
  const field = el.closest(".field") || el.parentElement;
  let err = field.querySelector(".field-error[data-live]");
  if (!err) {
    err = document.createElement("span");
    err.className = "field-error";
    err.dataset.live = "1";
    err.id = `${el.id || el.name}-error`;
    field.appendChild(err);
  }
  return err;
}

function check(el) {
  if (!el.willValidate) return true;
  const ok = el.checkValidity();
  const server = el.closest(".field")?.querySelector(".field-error:not([data-live])");
  if (ok) {
    el.removeAttribute("aria-invalid");
    errorEl(el).remove();
    server?.remove();
    return true;
  }
  server?.remove();
  const err = errorEl(el);
  err.textContent = message(el);
  el.setAttribute("aria-invalid", "true");
  el.setAttribute("aria-describedby", err.id);
  return false;
}

export function initForms(root) {
  for (const form of all(root, "form[data-validate]")) {
    if (!once(form, "validateReady")) continue;
    form.addEventListener("blur", (e) => {
      const el = e.target;
      if (el.matches("input, select, textarea") && (el.value !== "" || el.dataset.touched)) check(el);
    }, true);
    form.addEventListener("input", (e) => {
      const el = e.target;
      el.dataset.touched = "1";
      if (el.getAttribute("aria-invalid") === "true") check(el);
    });
    form.addEventListener("submit", (e) => {
      const fields = [...form.querySelectorAll("input, select, textarea")].filter((el) => el.willValidate && el.type !== "hidden");
      const bad = fields.filter((el) => !check(el));
      if (bad.length) {
        e.preventDefault();
        e.stopImmediatePropagation();
        bad[0].focus();
      }
    }, true);
  }
  for (const s of all(root, "[data-error-summary]")) {
    if (once(s, "summaryReady")) s.focus();
  }
  // one submit per form: buttons show the wheel and stop answering
  for (const form of all(root, "form[data-submit-loading]")) {
    if (!once(form, "onceSubmit")) continue;
    form.addEventListener("submit", (e) => {
      if (form.dataset.submitting) { e.preventDefault(); return; }
      if (!e.defaultPrevented) form.dataset.submitting = "1";
    });
  }
  if (!window.__formsWired) {
    window.__formsWired = true;
    window.addEventListener("pageshow", () => document.querySelectorAll("form[data-submitting]").forEach((f) => delete f.dataset.submitting));
  }
}
