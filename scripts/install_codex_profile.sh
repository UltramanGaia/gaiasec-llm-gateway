#!/usr/bin/env bash
set -euo pipefail

if ! command -v codex >/dev/null 2>&1; then
  echo "codex CLI is required" >&2
  exit 1
fi

PROFILE_NAME="${PROFILE_NAME:-${1:-rosetta-openai}}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8090}"
MODEL_NAME="${MODEL_NAME:-minimax-m25}"
CODEX_HOME_DIR="${CODEX_HOME_DIR:-${CODEX_HOME:-$HOME/.codex}}"
TEMPLATE_PATH="${TEMPLATE_PATH:-$(cd "$(dirname "$0")/.." && pwd)/templates/codex/gateway-openai-profile.config.toml}"
DEST_PATH="$CODEX_HOME_DIR/$PROFILE_NAME.config.toml"

if [ ! -f "$TEMPLATE_PATH" ]; then
  echo "template not found: $TEMPLATE_PATH" >&2
  exit 1
fi

mkdir -p "$CODEX_HOME_DIR"

python3 - "$TEMPLATE_PATH" "$DEST_PATH" "$GATEWAY_URL" "$MODEL_NAME" <<'PY'
import pathlib
import sys

template_path = pathlib.Path(sys.argv[1])
dest_path = pathlib.Path(sys.argv[2])
gateway_url = sys.argv[3].rstrip("/")
model_name = sys.argv[4]

content = template_path.read_text(encoding="utf-8")
content = content.replace("__GATEWAY_BASE_URL__", gateway_url)
content = content.replace("__MODEL_NAME__", model_name)
dest_path.write_text(content, encoding="utf-8")
print(dest_path)
PY

echo "Installed Codex profile: $PROFILE_NAME"
echo "Path: $DEST_PATH"
echo "Gateway: $GATEWAY_URL"
echo "Model: $MODEL_NAME"
echo
echo "Next:"
echo "  CODEX_PROFILE=$PROFILE_NAME MODEL_NAME=$MODEL_NAME ./scripts/e2e_codex.sh"
