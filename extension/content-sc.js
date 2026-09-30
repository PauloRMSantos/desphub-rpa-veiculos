const DEBUG = true;
const log = (...a) => DEBUG && console.log("[desphub-sc]", ...a);

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg && msg.type === "SC_RUN") {
    runQuery(msg.plate, msg.renavam).then(sendResponse);
    return true; // async response
  }
});

async function runQuery(plate, renavam) {
  try {
    if (!onConsultaPage()) {
      return { ok: false, error: "not-on-consulta-page" };
    }
    const plateInput = findInput("placa");
    const renavamInput = findInput("renavam");
    const submit = findSubmit();
    if (!plateInput || !renavamInput || !submit) {
      log("form not found — is the broker logged in?", { plateInput, renavamInput, submit });
      return { ok: false, error: "form-not-found (login to DETRAN-SC first?)" };
    }

    const captured = waitForCapture(30000);

    setValue(plateInput, plate);
    setValue(renavamInput, (renavam || "").trim());
    log("filled form; submitting", { plate });
    submit.click();

    const payload = await captured;
    log("captured dossiê for", payload && payload.placa);
    return { ok: true, payload };
  } catch (e) {
    return { ok: false, error: String(e && e.message ? e.message : e) };
  }
}

function onConsultaPage() {
  return location.pathname.includes("consulta-dossie-veiculo");
}

function waitForCapture(timeoutMs) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      window.removeEventListener("message", onMsg);
      reject(new Error("timeout waiting for DETRAN-SC response"));
    }, timeoutMs);
    function onMsg(e) {
      if (e.source !== window) return;
      const d = e.data;
      if (!d || d.__desphubScCapture !== true) return;
      clearTimeout(timer);
      window.removeEventListener("message", onMsg);
      resolve(d.payload);
    }
    window.addEventListener("message", onMsg);
  });
}

function findInput(labelText) {
  labelText = labelText.toLowerCase();
  const inputs = [...document.querySelectorAll("input")];
  let el = inputs.find((i) => (i.placeholder || "").toLowerCase().includes(labelText));
  if (el) return el;
  el = inputs.find((i) => (i.getAttribute("formcontrolname") || "").toLowerCase().includes(labelText));
  if (el) return el;
  el = inputs.find((i) => (i.getAttribute("name") || "").toLowerCase().includes(labelText));
  if (el) return el;
  const label = [...document.querySelectorAll("label")].find((l) =>
    l.textContent.toLowerCase().includes(labelText)
  );
  if (label) {
    const id = label.getAttribute("for");
    if (id) {
      const byId = document.getElementById(id);
      if (byId) return byId;
    }
    const near = label.parentElement && label.parentElement.querySelector("input");
    if (near) return near;
  }
  return null;
}

function findSubmit() {
  const candidates = [...document.querySelectorAll("button, input[type=submit]")];
  return candidates.find((b) => (b.textContent || b.value || "").toLowerCase().includes("consultar")) || null;
}

function setValue(el, value) {
  const proto = el instanceof HTMLTextAreaElement
    ? window.HTMLTextAreaElement.prototype
    : window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value").set;
  el.focus();
  setter.call(el, value);
  el.dispatchEvent(new Event("input", { bubbles: true }));
  el.dispatchEvent(new Event("change", { bubbles: true }));
  el.dispatchEvent(new Event("blur", { bubbles: true }));
}
