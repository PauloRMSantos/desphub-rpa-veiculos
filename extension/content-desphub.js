// Bridge between the DespHub web app (page context) and the extension.
//
// Runs only on the DespHub web origin (see manifest `content_scripts.matches`).
// The web app talks to the extension via window.postMessage; this script relays
// to the background service worker. Messages are validated by origin + a tag.
//
// Protocol (page → extension), all messages tagged `__desphub: true`:
//   { __desphub:true, id, type:"ARM_GOVBR_CAPTURE", pairingToken, backendUrl }
//   { __desphub:true, id, type:"GET_GOVBR_STATUS" }
//
// Replies (extension → page), tagged `__desphubExt: true`:
//   { __desphubExt:true, id, type:"ARM_RESULT",   payload }
//   { __desphubExt:true, id, type:"GOVBR_STATUS", payload }
//   { __desphubExt:true, type:"EXT_READY" }   // announced once on load

window.addEventListener("message", async (event) => {
  // Only accept messages from THIS page (not iframes / other origins).
  if (event.source !== window) return;
  const msg = event.data;
  if (!msg || msg.__desphub !== true || typeof msg.type !== "string") return;

  try {
    if (msg.type === "ARM_GOVBR_CAPTURE") {
      const payload = await chrome.runtime.sendMessage({
        type: "ARM",
        pairingToken: msg.pairingToken,
        backendUrl: msg.backendUrl,
      });
      reply(msg.id, "ARM_RESULT", payload);
    } else if (msg.type === "GET_GOVBR_STATUS") {
      const payload = await chrome.runtime.sendMessage({ type: "STATUS" });
      reply(msg.id, "GOVBR_STATUS", payload);
    }
  } catch (e) {
    reply(msg.id, "ERROR", { error: String(e) });
  }
});

function reply(id, type, payload) {
  window.postMessage(
    { __desphubExt: true, id, type, payload },
    window.location.origin
  );
}

// Let the app know the extension is installed (so it can enable the button).
window.postMessage({ __desphubExt: true, type: "EXT_READY" }, window.location.origin);
