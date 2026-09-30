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
    } else if (msg.type === "RUN_SC_QUERY") {
      const payload = await chrome.runtime.sendMessage({
        type: "RUN_SC_QUERY",
        plate: msg.plate,
        renavam: msg.renavam,
      });
      reply(msg.id, "SC_RESULT", payload);
    } else if (msg.type === "CONNECT_SC") {
      const payload = await chrome.runtime.sendMessage({ type: "CONNECT_SC" });
      reply(msg.id, "SC_CONNECT_RESULT", payload);
    } else if (msg.type === "SC_STATUS") {
      const payload = await chrome.runtime.sendMessage({ type: "SC_STATUS" });
      reply(msg.id, "SC_STATUS", payload);
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
