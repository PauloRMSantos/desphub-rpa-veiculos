(() => {
  const RESULT_URL = "/transito-api/veiculo/resposta-consulta";
  const ORIGIN = window.location.origin;

  function emit(data) {
    if (!data || typeof data !== "object" || !data.placa) return;
    window.postMessage({ __desphubScCapture: true, payload: data }, ORIGIN);
  }

  const origFetch = window.fetch;
  if (typeof origFetch === "function") {
    window.fetch = function (input, init) {
      const url = typeof input === "string" ? input : (input && input.url) || "";
      const p = origFetch.apply(this, arguments);
      if (url.includes(RESULT_URL)) {
        p.then((res) => {
          res.clone().json().then(emit).catch(() => {});
        }).catch(() => {});
      }
      return p;
    };
  }

  const proto = XMLHttpRequest.prototype;
  const origOpen = proto.open;
  const origSend = proto.send;

  proto.open = function (_method, url) {
    this.__scUrl = url || "";
    return origOpen.apply(this, arguments);
  };

  proto.send = function () {
    if (this.__scUrl && String(this.__scUrl).includes(RESULT_URL)) {
      this.addEventListener("load", function () {
        try {
          emit(JSON.parse(this.responseText));
        } catch (_) {
          /* ignore non-JSON */
        }
      });
    }
    return origSend.apply(this, arguments);
  };
})();
