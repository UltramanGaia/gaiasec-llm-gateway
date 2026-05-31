#!/usr/bin/env bash
set -euo pipefail

if ! command -v claude >/dev/null 2>&1; then
  echo "claude CLI is required" >&2
  exit 1
fi

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8090}"
MODEL_NAME="${MODEL_NAME:-minimax-m25}"
ANTHROPIC_API_KEY_VALUE="${ANTHROPIC_API_KEY_VALUE:-dummy}"
CLAUDE_ENV_DIR="${CLAUDE_ENV_DIR:-$HOME/.config/gaiasec-llm-gateway}"
CLAUDE_ENV_NAME="${CLAUDE_ENV_NAME:-${1:-claude-gateway.env}}"
TEMPLATE_PATH="${TEMPLATE_PATH:-$(cd "$(dirname "$0")/.." && pwd)/templates/claude/gateway.env}"
DEST_PATH="$CLAUDE_ENV_DIR/$CLAUDE_ENV_NAME"

if [ ! -f "$TEMPLATE_PATH" ]; then
  echo "template not found: $TEMPLATE_PATH" >&2
  exit 1
fi

mkdir -p "$CLAUDE_ENV_DIR"

python3 - "$TEMPLATE_PATH" "$DEST_PATH" "$GATEWAY_URL" "$ANTHROPIC_API_KEY_VALUE" "$MODEL_NAME" <<'PY'
import pathlib
import sys

template_path = pathlib.Path(sys.argv[1])
dest_path = pathlib.Path(sys.argv[2])
gateway_url = sys.argv[3].rstrip("/")
api_key = sys.argv[4]
model_name = sys.argv[5]

content = template_path.read_text(encoding="utf-8")
content = content.replace("__GATEWAY_BASE_URL__", gateway_url)
content = content.replace("__ANTHROPIC_API_KEY__", api_key)
content = content.replace("__MODEL_NAME__", model_name)
dest_path.write_text(content, encoding="utf-8")
print(dest_path)
PY

chmod 600 "$DEST_PATH"

echo "Installed Claude gateway env"
echo "Path: $DEST_PATH"
echo "Gateway: $GATEWAY_URL"
echo "Model: $MODEL_NAME"
echo
echo "Next:"
echo "  CLAUDE_GATEWAY_ENV=$DEST_PATH ./scripts/e2e_claude_code.sh"
