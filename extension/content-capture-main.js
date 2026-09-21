// MAIN-world capture: runs inside the DETRAN SPA page and wraps fetch + XHR so
// it can read the gov.br session headers (Authorization + X-User-Id) the app
// sends to the procergs API. Runs at document_start, BEFORE the Angular app
// makes any request. Captured values go to the isolated bridge via postMessage.
//
// This replaces the webRequest approach, which is unreliable in an MV3 service
// worker (lifecycle) and, crucially, the DETRAN app uses XMLHttpRequest
// (Angular HttpClient), not fetch.
(() => {
  const API_HOST = "pcsdetran.procergs.com.br";
  const ORIGIN = window.location.origin;

  function emit(rawAuthz, userId) {
    if (!rawAuthz || !userId) return;
    const bearer = String(rawAuthz).replace(/^Bearer\s+/i, "").trim();
    if (!bearer) return;
    window.postMessage({ __desphubCapture: true, bearer, userId: String(userId) }, ORIGIN);
  }

  // --- fetch (in case the app or a dependency uses it) ---
  const origFetch = window.fetch;
  if (typeof origFetch === "function") {
    window.fetch = function (input, init) {
      try {
        const url = typeof input === "string" ? input : (input && input.url) || "";
        if (url.includes(API_HOST)) {
          const h = new Headers((init && init.headers) || (input && input.headers) || undefined);
          emit(h.get("authorization"), h.get("x-user-id"));
        }
      } catch (_) {
        /* never break the page */
      }
      return origFetch.apply(this, arguments);
    };
  }

  // --- XMLHttpRequest (what Angular HttpClient actually uses) ---
  const proto = XMLHttpRequest.prototype;
  const origOpen = proto.open;
  const origSet = proto.setRequestHeader;
  const origSend = proto.send;

  proto.open = function (_method, url) {
    this.__dhUrl = url || "";
    this.__dhHeaders = {};
    return origOpen.apply(this, arguments);
  };

  proto.setRequestHeader = function (name, value) {
    try {
      if (this.__dhHeaders) this.__dhHeaders[String(name).toLowerCase()] = value;
    } catch (_) {}
    return origSet.apply(this, arguments);
  };

  proto.send = function () {
    try {
      if (this.__dhUrl && String(this.__dhUrl).includes(API_HOST) && this.__dhHeaders) {
        emit(this.__dhHeaders["authorization"], this.__dhHeaders["x-user-id"]);
      }
    } catch (_) {}
    return origSend.apply(this, arguments);
  };
})();
