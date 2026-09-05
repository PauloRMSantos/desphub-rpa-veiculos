#!/usr/bin/env bash
# Abre um Chrome dedicado ao DespHub RPA com depuração remota e perfil próprio.
#
# Perfil persistente ($HOME/.desphub-chrome): guarda os cookies do gov.br, então
# o "dispositivo confiável" é lembrado e o código/captcha tende a não repetir.
# O RPA se conecta a este Chrome (CHROME_DEBUG_URL=http://localhost:9222) — como
# é um navegador real, o Turnstile do gov.br não invalida o login.
set -euo pipefail

PORT="${CHROME_DEBUG_PORT:-9222}"
PROFILE="${CHROME_PROFILE_DIR:-$HOME/.desphub-chrome}"
URL="https://pcsdetran.rs.gov.br/consulta-veiculo?contabiliza=true"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

if [ ! -x "$CHROME" ]; then
  echo "Google Chrome não encontrado em: $CHROME" >&2
  exit 1
fi

echo "Abrindo Chrome (porta de depuração $PORT, perfil $PROFILE)..."
exec "$CHROME" \
  --remote-debugging-port="$PORT" \
  --user-data-dir="$PROFILE" \
  "$URL"
