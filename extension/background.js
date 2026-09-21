// DespHub gov.br courier — service worker (MV3).
//
// Responsibilities:
//   1. When the DespHub web app "arms" it (with a pairing token + backend URL),
//      start watching the DETRAN-RS API for the session headers.
//   2. On the first request that carries them, extract Bearer + X-User-Id.
//   3. Relay them to the DespHub backend (authenticated with the pairing token),
//      which forwards to the RPA. The RPA's own X-Api-Key never lives here.
//
// The capture is ONE-SHOT and time-boxed: no arm, no listening.

const DETRAN_API = "https://pcsdetran.procergs.com.br/*";
const BACKEND_SESSION_PATH = "/api/rpa/govbr-session";
const ARM_TTL_MS = 5 * 60 * 1000; // pairing window

// Flip to false to silence the console trace.
const DEBUG = true;
const log = (...a) => DEBUG && console.log("[desphub-courier]", ...a);

// Dedupe: the DETRAN SPA fires many procergs calls in quick succession, all
// carrying the SAME session token. Once we relay a token we ignore repeats,
// so a single "Connect" produces a single relay. Reset on each new arm.
let relayedBearer = null;

// Listener registered at top level so it survives service-worker restarts.
// Non-blocking (no "blocking" in the spec): we only observe, never modify.
chrome.webRequest.onBeforeSendHeaders.addListener(
  handleHeaders,
  { urls: [DETRAN_API] },
  ["requestHeaders", "extraHeaders"] // extraHeaders exposes Authorization
);
log("service worker loaded; watching", DETRAN_API);

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || typeof msg.type !== "string") return;
  switch (msg.type) {
    case "ARM":
      arm(msg.pairingToken, msg.backendUrl).then((r) => sendResponse(r));
      return true; // async response
    case "DISARM":
      disarm().then(() => sendResponse({ ok: true }));
      return true;
    case "STATUS":
      getState().then(sendResponse);
      return true;
    case "CAPTURED":
      handleCaptured(msg.bearer, msg.userId).then((r) => sendResponse(r));
      return true;
  }
});

// Captured by the content-script hook on the DETRAN page.
async function handleCaptured(bearer, userId) {
  if (!bearer || !userId) return { ok: false };
  // Synchronous dedupe BEFORE any await: concurrent captures of the same token
  // collapse into a single relay.
  if (bearer === relayedBearer) return { ok: false, reason: "dup" };
  relayedBearer = bearer;

  const { arm } = await chrome.storage.session.get("arm");
  if (!arm || arm.expiresAt < Date.now()) {
    log("captured a session but NOT armed (ignoring)");
    relayedBearer = null; // wasn't consumed; allow a later armed capture
    return { ok: false, reason: "not-armed" };
  }
  log("CAPTURED → relaying to backend");
  await chrome.storage.session.remove("arm"); // one-shot
  await relay(arm, bearer, userId);
  return { ok: true };
}

async function arm(pairingToken, backendUrl) {
  if (!pairingToken || !backendUrl) {
    return { ok: false, error: "missing pairingToken or backendUrl" };
  }
  await chrome.storage.session.set({
    arm: { pairingToken, backendUrl, expiresAt: Date.now() + ARM_TTL_MS },
  });
  relayedBearer = null; // fresh connection: allow relaying the next captured token
  setBadge("•", "#f59e0b"); // amber: armed / waiting
  log("ARMED — backendUrl:", backendUrl, "| now log in at the DETRAN consulta page");
  return { ok: true };
}

async function disarm() {
  await chrome.storage.session.remove("arm");
  setBadge("", "#000000");
}

async function getState() {
  const { arm, lastConnect } = await chrome.storage.session.get(["arm", "lastConnect"]);
  const armed = !!arm && arm.expiresAt > Date.now();
  return { armed, lastConnect: lastConnect || null };
}

async function handleHeaders(details) {
  const headers = details.requestHeaders || [];
  const pick = (name) =>
    headers.find((h) => h.name.toLowerCase() === name)?.value;

  const authz = pick("authorization");
  const userId = pick("x-user-id");
  if (!authz || !userId) return; // preflight / unauthenticated call

  const bearer = authz.replace(/^Bearer\s+/i, "").trim();
  // Same deduped path as the content-script capture (arm-check + one-shot relay).
  await handleCaptured(bearer, userId);
}

async function relay(arm, bearer, userId) {
  const url = arm.backendUrl.replace(/\/+$/, "") + BACKEND_SESSION_PATH;
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + arm.pairingToken,
      },
      body: JSON.stringify({ bearer, userId }),
    });
    const data = await res.json().catch(() => ({}));
    const record = {
      ok: res.ok,
      status: res.status,
      at: Date.now(),
      expiresAt: data.expiresAt || null,
    };
    await chrome.storage.session.set({ lastConnect: record });
    setBadge(res.ok ? "✓" : "!", res.ok ? "#16a34a" : "#dc2626");
    log("RELAY", res.ok ? "OK" : "FAILED", "→ POST", url, "→ HTTP", res.status, data);
  } catch (e) {
    await chrome.storage.session.set({
      lastConnect: { ok: false, at: Date.now(), error: String(e) },
    });
    setBadge("!", "#dc2626");
    log("RELAY ERROR → POST", url, "→", String(e));
  }
}

function setBadge(text, color) {
  try {
    chrome.action.setBadgeText({ text });
    if (color) chrome.action.setBadgeBackgroundColor({ color });
  } catch (_) {
    /* action API may be unavailable in some contexts */
  }
}
