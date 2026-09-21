# Design — gov.br Session Courier (Browser Extension)

How the DespHub broker connects their gov.br account so the RPA can query
DETRAN-RS, **without ever handling a "token" by hand** and **without the RPA
touching the gov.br login** (which fails on a datacenter IP — see below).

> Status: **design** (not yet implemented). This document is the blueprint for
> the extension and the one new backend endpoint it needs.

---

## 1. Why an extension at all

The gov.br session token (a JWT `Bearer` + an `X-User-Id`) lives **in the memory
of the DETRAN SPA** (`pcsdetran.rs.gov.br`), sent as a header on its API calls.
Three hard constraints shape the whole design:

1. **Bot detection.** The gov.br login is behind Cloudflare Turnstile. From a
   datacenter IP (our Hostinger VPS) the captcha rejects even correct answers,
   so **the login cannot happen on the server**. It must happen in the broker's
   own residential browser, where the captcha behaves. *(Validated 2026-09-19:
   the DETRAN API itself returns 200 from the VPS — only the login is blocked.)*
2. **Same-Origin Policy.** The DespHub web app (its own origin) **cannot read**
   anything inside the DETRAN SPA tab (another origin). So the DespHub page alone
   cannot grab the token. Only a browser extension (or a native helper) can see
   across origins.
3. **The RPA is internal-only.** In production the RPA is not exposed to the
   internet (Docker network `desphub-net`, no published port). The extension,
   living on the public internet, **cannot reach the RPA directly**. It must go
   through the public DespHub backend.

The extension is the smallest thing that satisfies all three: it runs in the
broker's real browser, reads across origins, and hands the token to the backend
over HTTPS.

---

## 2. Architecture

```
┌──────────────── Broker's own browser (residential IP) ─────────────────┐
│                                                                         │
│  DespHub web app tab            DETRAN SPA tab (gov.br login)           │
│  ┌───────────────────┐          ┌───────────────────────────┐          │
│  │ "Connect gov.br"  │          │ broker logs in (captcha OK)│          │
│  │  button           │          │ SPA calls procergs w/ token│          │
│  └─────────┬─────────┘          └──────────────┬────────────┘          │
│            │ (1) pairing token                 │ (3) capture            │
│            ▼          ┌────────────────────────▼──────────┐             │
│         ┌──────────── │        EXTENSION (courier)         │             │
│         │  content    │  - arms capture on "Connect"       │             │
│         │  script  ◀──┤  - reads Authorization + X-User-Id │             │
│         └──────────── │  - POSTs to backend (HTTPS)        │             │
│                       └──────────────┬─────────────────────┘             │
└──────────────────────────────────────┼──────────────────────────────────┘
                                        │ (4) POST /api/rpa/govbr-session
                                        │     Authorization: <broker's DespHub JWT>
                                        ▼
                        ┌──────────────────────────────┐
                        │   DespHub Backend (public)    │
                        │  - authenticates the broker   │
                        │  - relays to RPA (internal)   │
                        └───────────────┬───────────────┘
                                        │ (5) POST /api/detran/session/token
                                        │     X-Api-Key: <server-side secret>
                                        ▼
                        ┌──────────────────────────────┐
                        │   RPA (desphub-net, internal) │
                        │   stores token, ready to query│
                        └──────────────────────────────┘
```

**Key point:** the RPA's `SESSION_TOKEN_APIKEY` stays **only on the backend**.
The extension authenticates with the broker's normal DespHub session and never
sees that key. (Extension source is world-readable — a secret there = leaked.)

---

## 3. The capture mechanism

Two viable strategies; primary + fallback.

### Primary — observe the request headers (`webRequest`)
A background service worker listens (non-blocking) on requests to the DETRAN API
and reads the two headers the SPA attaches:

```js
chrome.webRequest.onBeforeSendHeaders.addListener(
  onHeaders,
  { urls: ["https://pcsdetran.procergs.com.br/*"] },
  ["requestHeaders", "extraHeaders"]   // extraHeaders needed to see Authorization
);
```
From the matched request it pulls `authorization` (strip `Bearer `) and
`x-user-id`. This is exactly what the Go RPA does today via CDP, moved into the
broker's browser.

### Fallback — hook `fetch`/`XHR` in the page (`world: "MAIN"` content script)
If `extraHeaders` ever stops exposing `Authorization`, a `document_start`
content script injected into the DETRAN page's MAIN world wraps `fetch` and
`XMLHttpRequest.setRequestHeader` to read the header the app itself sets. More
invasive, but immune to header-visibility rules.

> Decision: ship **Primary**; keep the fallback documented for resilience.

---

## 4. Arming & pairing (which broker is this token for?)

> **Decision (locked):** the extension authenticates to the backend with a
> **dedicated pairing token**, not the broker's full DespHub session. This keeps
> the extension's power scoped to a single, time-boxed connection.

The token must be bound to the broker who clicked "Connect", not whoever happens
to be logged into gov.br. Flow:

1. Broker clicks **"Connect gov.br"** in the DespHub web app.
2. The web app asks its backend for a **short-lived pairing token** (JWT, ~5 min,
   bound to this broker) and passes it to the extension via the content script
   (`window.postMessage` → content script → `chrome.runtime.sendMessage`).
3. The extension **arms** capture and opens the DETRAN login tab.
4. On the first captured request, the extension POSTs `{bearer, userId}` to the
   backend **with the pairing token** as its credential.
5. Backend validates the pairing token → knows the broker → relays to the RPA →
   marks the broker "connected".

Arming is one-shot and time-boxed: no click, no capture. The extension only ever
listens on the DETRAN origin, only while armed.

---

## 5. New backend endpoints (what the back team must build)

**5a. Issue a pairing token** (called by the DespHub web app on "Connect"):
```
POST /api/rpa/pairing-token          (authenticated as the logged-in broker)
→ { "pairingToken": "<short-lived JWT, ~5 min, bound to brokerId>" }
```

**5b. Receive the captured gov.br session** (called by the extension):
```
POST /api/rpa/govbr-session
Authorization: Bearer <pairing token from 5a>
Content-Type: application/json

{ "bearer": "<gov.br JWT>", "userId": "<X-User-Id, base64 CPF>" }
```

Backend responsibilities:
- Validate the pairing token → resolve the broker it was issued to.
- Relay to the RPA: `POST /api/detran/session/token` with header
  `X-Api-Key: <SESSION_TOKEN_APIKEY>` and the same body.
- Persist per-broker connection state (connected, `expiresAt` from the JWT `exp`).
- Return `{ "status": "connected", "expiresAt": "..." }` to the extension.

Nothing else in the current RPA contract changes — `/session/token` already
accepts exactly `{bearer, userId}`.

---

## 6. UX states (in the DespHub web app)

| State | Trigger | What the broker sees |
|-------|---------|----------------------|
| **Disconnected** | no token / never connected | Button **"Connect gov.br"** |
| **Connecting** | armed, DETRAN tab open | "Waiting for gov.br login…" |
| **Connected** | token relayed OK | Green badge **"Connected until HH:MM"** |
| **Expiring/expired** | `GET /api/detran/session` says expired, or a query returns step `reconnect` | Amber **"Reconnect gov.br"** |

The web app polls `GET /api/detran/session` (through the backend) to drive the
badge. Thanks to gov.br **trusted-device**, reconns rarely re-prompt a captcha.

---

## 7. Extension skeleton (MV3)

```
extension/
  manifest.json        # MV3; permissions: webRequest; host_permissions:
                       #   https://pcsdetran.procergs.com.br/*, DespHub web origin
  background.js        # service worker: arm/disarm, capture, POST to backend
  content-govbr.js     # (fallback) MAIN-world fetch/XHR hook on DETRAN page
  content-desphub.js   # bridge on the DespHub web origin (pairing handshake)
  popup.html/js        # optional: manual "Connect" + status, for debugging
```

Permissions are deliberately minimal: it can observe the DETRAN API origin and
talk to the DespHub origin — nothing else.

---

## 8. Security & LGPD notes

- The RPA API key never leaves the backend.
- Pairing tokens are short-lived and single-purpose.
- The gov.br token/`X-User-Id` travel only over HTTPS to the DespHub backend
  (the same trust boundary the broker already uses).
- The extension captures **only** on the DETRAN origin and **only** while armed.
- CPF masking (`ANONYMIZE_PII`) still applies on query output, unchanged.

---

## 9. Non-Chrome / no-extension fallback

Brokers not on a Chromium browser use the packaged `cmd/desphub-login` helper
(attach to their real Chrome via remote debugging, capture, POST to backend).
Same relay path; different courier.

---

## 10. Long-term north star

The extension is the pragmatic path while DespHub uses the public DETRAN-RS
portal. The definitive route is an **official DETRAN-RS / WSDenatran convênio**
(API credentials, no browser, no captcha, no token capture) — at which point the
courier is retired. This is the evolution to cite in the thesis.
```
