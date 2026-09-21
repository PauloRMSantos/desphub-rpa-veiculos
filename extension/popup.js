// Popup: reflects the courier state (idle / armed / connected / error).
// Purely informational — the real flow is driven by the DespHub web app.

const dot = document.getElementById("dot");
const state = document.getElementById("state");
const detail = document.getElementById("detail");

function fmt(ts) {
  if (!ts) return "";
  try {
    return new Date(ts).toLocaleString("pt-BR");
  } catch (_) {
    return String(ts);
  }
}

async function render() {
  let s;
  try {
    s = await chrome.runtime.sendMessage({ type: "STATUS" });
  } catch (_) {
    s = null;
  }

  dot.className = "dot";
  if (!s) {
    state.textContent = "Indisponível";
    return;
  }
  if (s.armed) {
    dot.classList.add("armed");
    state.textContent = "Aguardando login no gov.br…";
    detail.textContent = "";
    return;
  }
  if (s.lastConnect) {
    if (s.lastConnect.ok) {
      dot.classList.add("ok");
      state.textContent = "Conectado";
      detail.textContent = s.lastConnect.expiresAt
        ? "Sessão válida até " + fmt(s.lastConnect.expiresAt)
        : "Enviado ao DespHub em " + fmt(s.lastConnect.at);
    } else {
      dot.classList.add("err");
      state.textContent = "Falha ao conectar";
      detail.textContent =
        s.lastConnect.error || "HTTP " + (s.lastConnect.status || "?");
    }
    return;
  }
  state.textContent = "Ocioso";
  detail.textContent = "Acione pelo DespHub.";
}

render();
