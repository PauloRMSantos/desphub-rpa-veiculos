// FALLBACK capture strategy (NOT registered in the manifest by default).
//
// The primary path (background.js + webRequest + extraHeaders) already reads the
// Authorization / X-User-Id headers. Keep this only if a future Chrome release
// stops exposing Authorization to webRequest. To enable it, add to the manifest:
//
//   "content_scripts": [{
//     "matches": ["https://pcsdetran.rs.gov.br/*"],
//     "js": ["content-govbr.js"],
//     "run_at": "document_start",
//     "world": "MAIN"
//   }]
//
// Running in the MAIN world at document_start, it wraps the page's own fetch so
// it can read the header the DETRAN SPA attaches, then forwards it to the
// extension via window.postMessage (bridged by a companion content script in the
// ISOLATED world — omitted here for brevity).

(() => {
  const origFetch = window.fetch;
  if (!origFetch) return;

  window.fetch = function (input, init) {
    try {
      const headers = new Headers((init && init.headers) || undefined);
      const authz = headers.get("authorization");
      const userId = headers.get("x-user-id");
      const url = typeof input === "string" ? input : input && input.url;
      if (authz && userId && url && url.includes("pcsdetran.procergs.com.br")) {
        window.postMessage(
          {
            __desphubCapture: true,
            bearer: authz.replace(/^Bearer\s+/i, "").trim(),
            userId,
          },
          window.location.origin
        );
      }
    } catch (_) {
      /* never break the page */
    }
    return origFetch.apply(this, arguments);
  };
})();
