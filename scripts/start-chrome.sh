set -euo pipefail

PORT="${CHROME_DEBUG_PORT:-9222}"
PROFILE="${CHROME_PROFILE_DIR:-$HOME/.desphub-chrome}"
URL="https://pcsdetran.rs.gov.br/consulta-veiculo?contabiliza=true"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

if [ ! -x "$CHROME" ]; then
  echo "Google Chrome not found at: $CHROME" >&2
  exit 1
fi

echo "Opening Chrome (debug port $PORT, profile $PROFILE)..."
exec "$CHROME" \
  --remote-debugging-port="$PORT" \
  --user-data-dir="$PROFILE" \
  "$URL"
