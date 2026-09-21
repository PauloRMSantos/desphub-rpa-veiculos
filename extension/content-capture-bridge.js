// ISOLATED-world bridge on the DETRAN SPA page: receives the captured session
// from the MAIN-world hook (via window.postMessage) and forwards it to the
// background service worker, which relays it to the backend when armed.
window.addEventListener("message", (event) => {
  if (event.source !== window) return;
  const msg = event.data;
  if (!msg || msg.__desphubCapture !== true) return;
  if (!msg.bearer || !msg.userId) return;
  try {
    chrome.runtime.sendMessage({ type: "CAPTURED", bearer: msg.bearer, userId: msg.userId });
  } catch (_) {
    // extension context invalidated (e.g. after a reload) — the page reload that
    // follows re-injects a fresh bridge.
  }
});
