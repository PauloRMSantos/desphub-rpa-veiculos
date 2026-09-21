# DespHub — gov.br Session Courier (browser extension)

Captures the gov.br/DETRAN-RS session **in the broker's own browser** and hands
it to the DespHub backend, which relays it to the RPA. See the full design in
[`../docs/COURIER.md`](../docs/COURIER.md).

> Status: **scaffold (v0.1.0)**. The capture + relay flow works; it needs the two
> backend endpoints (below) and the DespHub web app trigger to be end-to-end.

## Files

| File | Role |
|------|------|
| `manifest.json` | MV3 manifest. **Edit `host_permissions` + content-script `matches`** to your real DespHub origins. |
| `background.js` | Service worker: arm → capture (`webRequest`) → relay to backend. |
| `content-desphub.js` | Bridge on the DespHub web origin (pairing handshake via `postMessage`). |
| `content-govbr.js` | Fallback `fetch`-hook capture (documented, not registered). |
| `popup.html` / `popup.js` | Status indicator (idle / waiting / connected / error). |

## Configure

Replace the placeholder origins in `manifest.json` with your real ones (both dev
and prod), in `host_permissions` **and** in `content_scripts[0].matches`:

- DespHub web app origin — where the "Connect gov.br" button lives (e.g.
  `http://localhost:5173/*`, `https://app.desphub.com/*`).
- DespHub backend origin — where `POST /api/rpa/govbr-session` lives (e.g.
  `https://api.desphub.com/*`). Needed so the worker's `fetch` is allowed.

The backend URL itself is **not** hardcoded — the web app passes it when arming,
so the same build works for dev and prod.

## Load it (unpacked, for development)

1. Chrome → `chrome://extensions`
2. Enable **Developer mode** (top right)
3. **Load unpacked** → select this `extension/` folder
4. Pin it; the popup shows the current state.

## How the DespHub web app drives it

The frontend needs the extension present, then arms it with a **pairing token**
(issued by the backend, see below). Minimal integration:

```js
// 1. Ask the backend for a short-lived pairing token (bound to this broker).
const { pairingToken } = await fetch("/api/rpa/pairing-token", { method: "POST" })
  .then((r) => r.json());

// 2. Arm the extension and open the DETRAN login.
const id = crypto.randomUUID();
window.postMessage(
  {
    __desphub: true,
    id,
    type: "ARM_GOVBR_CAPTURE",
    pairingToken,
    backendUrl: "https://api.desphub.com", // your backend origin
  },
  window.location.origin
);
window.open("https://pcsdetran.rs.gov.br/consulta-veiculo?contabiliza=true", "_blank");

// 3. (optional) Poll status to drive the "Connected" badge.
window.addEventListener("message", (e) => {
  if (e.source !== window || !e.data?.__desphubExt) return;
  if (e.data.type === "GOVBR_STATUS") console.log("status:", e.data.payload);
  if (e.data.type === "EXT_READY") console.log("extension installed");
});
function pollStatus() {
  window.postMessage({ __desphub: true, id: crypto.randomUUID(), type: "GET_GOVBR_STATUS" }, window.location.origin);
}
```

If `EXT_READY` never arrives, the extension isn't installed — show the install
prompt (or fall back to the `cmd/desphub-login` helper).

## Backend endpoints this depends on (to be built)

1. `POST /api/rpa/pairing-token` — authenticated as the logged-in broker, returns
   a short-lived (~5 min) pairing token bound to that broker.
2. `POST /api/rpa/govbr-session` — receives `{ bearer, userId }` with the pairing
   token as `Authorization`, validates it, and relays to the RPA
   (`POST /api/detran/session/token` with the internal `X-Api-Key`). Returns
   `{ status, expiresAt }`.

The RPA's `SESSION_TOKEN_APIKEY` stays **only on the backend** — never in this
extension.

## Security notes

- Non-blocking `webRequest`: the courier only **observes** DETRAN requests, never
  modifies them.
- Capture is armed one-shot and time-boxed (5 min); outside that window the
  worker ignores everything.
- The gov.br token travels only over HTTPS to the DespHub backend.
