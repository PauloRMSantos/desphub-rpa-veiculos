const DETRAN_API = "https://pcsdetran.procergs.com.br/*";
const BACKEND_SESSION_PATH = "/api/rpa/govbr-session";
const ARM_TTL_MS = 5 * 60 * 1000; // pairing window


const DEBUG = true;
const log = (...a) => DEBUG && console.log("[desphub-courier]", ...a);
let relayedBearer = null;

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
    case "RUN_SC_QUERY":
      handleRunScQuery(msg.plate, msg.renavam).then((r) => sendResponse(r));
      return true;
    case "CONNECT_SC":
      connectSc().then((r) => sendResponse(r));
      return true;
    case "SC_STATUS":
      scStatus().then((r) => sendResponse(r));
      return true;
  }
});

const SC_CONSULTA_URL = "https://servicos.detran.sc.gov.br/consulta-dossie-veiculo";
const SC_LOGIN_URL = "https://servicos.detran.sc.gov.br/login";
const SC_ORIGIN = "https://servicos.detran.sc.gov.br/*";

// Auto-minimize the SC worker window once login completes. Registered at top
// level so it survives service-worker restarts; the intent lives in
// storage.session (scAutoMinimize), set when the connect window is opened.
chrome.tabs.onUpdated.addListener(async (tabId, info, tab) => {
  if (info.status !== "complete") return;
  const { scWorkerTabId, scAutoMinimize } = await chrome.storage.session.get([
    "scWorkerTabId",
    "scAutoMinimize",
  ]);
  if (!scAutoMinimize || tabId !== scWorkerTabId) return;
  const url = (tab && tab.url) || "";
  // Logged in = on a DETRAN-SC page that is NOT the login screen.
  if (url.includes("servicos.detran.sc.gov.br") && !url.includes("/login")) {
    await chrome.storage.session.set({ scAutoMinimize: false });
    try {
      await chrome.windows.update(tab.windowId, { state: "minimized" });
      log("SC login detected → worker window minimized");
    } catch (_) {
      /* window gone */
    }
  }
});

// Connect DETRAN-SC once (analogous to the gov.br connection, but SC is a
// SEPARATE portal with its own login). Opens a dedicated window for the broker
// to log in via gov.br; that window becomes the reusable, out-of-the-way worker
// for SC queries. The broker minimizes it and forgets about it.
async function connectSc() {
  const existing = await getScWorkerTab();
  if (existing) {
    // Re-open focused for (re)login; it minimizes again once login completes.
    await chrome.storage.session.set({ scAutoMinimize: true });
    try {
      await chrome.windows.update(existing.windowId, { focused: true, state: "normal" });
    } catch (_) {}
    return { ok: true, reused: true };
  }
  const win = await chrome.windows.create({ url: SC_LOGIN_URL, focused: true, width: 980, height: 760 });
  const tab = win.tabs && win.tabs[0];
  if (tab) await chrome.storage.session.set({ scWorkerTabId: tab.id, scAutoMinimize: true });
  log("SC connect window opened; auto-minimizes once login completes");
  return { ok: true };
}

// The tracked DETRAN-SC worker tab, or null if not connected.
async function getScWorkerTab() {
  const { scWorkerTabId } = await chrome.storage.session.get("scWorkerTabId");
  if (scWorkerTabId != null) {
    try {
      const t = await chrome.tabs.get(scWorkerTabId);
      if (t && String(t.url || "").includes("servicos.detran.sc.gov.br")) return t;
    } catch (_) {
      /* tab gone */
    }
  }
  const tabs = await chrome.tabs.query({ url: SC_ORIGIN });
  if (tabs[0]) {
    await chrome.storage.session.set({ scWorkerTabId: tabs[0].id });
    return tabs[0];
  }
  return null;
}

// Logged in = a DETRAN-SC worker tab that is NOT on the login screen.
function isLoggedInScUrl(url) {
  return (
    typeof url === "string" &&
    url.includes("servicos.detran.sc.gov.br") &&
    !url.includes("/login")
  );
}

async function scStatus() {
  const tab = await getScWorkerTab();
  return { connected: !!(tab && isLoggedInScUrl(tab.url)) };
}

// Runs an SC query in the EXISTING worker tab (never opens a new visible tab).
// Returns { error: "sc-not-connected" } when there is no logged-in SC worker.
async function handleRunScQuery(plate, renavam) {
  if (!plate) return { ok: false, error: "missing plate" };
  try {
    const tab = await getScWorkerTab();
    if (!tab) return { ok: false, error: "sc-not-connected" };

    // Reset the worker to the consulta form (it may be on a result view).
    if (!String(tab.url || "").includes("consulta-dossie-veiculo")) {
      await chrome.tabs.update(tab.id, { url: SC_CONSULTA_URL });
      await waitTabComplete(tab.id, 20000);
    }
    // Do NOT focus the worker window — keep it out of the broker's way.
    const result = await chrome.tabs.sendMessage(tab.id, { type: "SC_RUN", plate, renavam });
    if (result && result.error === "not-on-consulta-page") {
      return { ok: false, error: "sc-not-connected" }; // redirected to login
    }
    log("SC query result:", result && result.ok ? "ok" : result);
    return result || { ok: false, error: "no response from SC tab" };
  } catch (e) {
    return { ok: false, error: String(e && e.message ? e.message : e) };
  }
}

function waitTabComplete(tabId, timeoutMs) {
  return new Promise((resolve, reject) => {
    let done = false;
    const finish = (fn, arg) => {
      if (done) return;
      done = true;
      chrome.tabs.onUpdated.removeListener(onUpd);
      clearTimeout(timer);
      fn(arg);
    };
    const timer = setTimeout(() => finish(reject, new Error("SC tab load timeout")), timeoutMs);
    const onUpd = (id, info) => {
      if (id === tabId && info.status === "complete") finish(resolve);
    };
    chrome.tabs.onUpdated.addListener(onUpd);
    chrome.tabs.get(tabId).then((t) => {
      if (t && t.status === "complete") finish(resolve);
    });
  });
}

async function handleCaptured(bearer, userId) {
  if (!bearer || !userId) return { ok: false };
  if (bearer === relayedBearer) return { ok: false, reason: "dup" };
  relayedBearer = bearer;

  const { arm } = await chrome.storage.session.get("arm");
  if (!arm || arm.expiresAt < Date.now()) {
    log("captured a session but NOT armed (ignoring)");
    relayedBearer = null; 
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
  relayedBearer = null; 
  setBadge("•", "#f59e0b"); 
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
  if (!authz || !userId) return;

  const bearer = authz.replace(/^Bearer\s+/i, "").trim();

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
