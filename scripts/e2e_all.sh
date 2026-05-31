#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8090}"
CONFIG_NAME="${CONFIG_NAME:-minimax-m25}"
UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-http://172.31.29.10/v1}"
UPSTREAM_MODEL="${UPSTREAM_MODEL:-MiniMax/MiniMax-M2.5}"
UPSTREAM_API_KEY="${UPSTREAM_API_KEY:-}"
CODEX_PROFILE_NAME="${CODEX_PROFILE_NAME:-rosetta-openai}"
CLAUDE_ENV_NAME="${CLAUDE_ENV_NAME:-claude-gateway.env}"
CLIENT="${CLIENT:-none}"

if [ -z "$UPSTREAM_API_KEY" ]; then
  echo "UPSTREAM_API_KEY is required" >&2
  exit 1
fi

echo "==> Running gateway smoke test"
UPSTREAM_API_KEY="$UPSTREAM_API_KEY" \
GATEWAY_URL="$GATEWAY_URL" \
CONFIG_NAME="$CONFIG_NAME" \
UPSTREAM_BASE_URL="$UPSTREAM_BASE_URL" \
UPSTREAM_MODEL="$UPSTREAM_MODEL" \
"$ROOT_DIR/scripts/e2e_gateway_smoke.sh"

echo "==> Installing Codex profile"
GATEWAY_URL="$GATEWAY_URL" \
MODEL_NAME="$CONFIG_NAME" \
"$ROOT_DIR/scripts/install_codex_profile.sh" "$CODEX_PROFILE_NAME"

echo "==> Installing Claude env"
GATEWAY_URL="$GATEWAY_URL" \
MODEL_NAME="$CONFIG_NAME" \
"$ROOT_DIR/scripts/install_claude_env.sh" "$CLAUDE_ENV_NAME"

case "$CLIENT" in
  none)
    cat <<EOF
All setup completed.

Next commands:
  CODEX_PROFILE=$CODEX_PROFILE_NAME MODEL_NAME=$CONFIG_NAME ./scripts/e2e_codex.sh
  CLAUDE_GATEWAY_ENV=\$HOME/.config/gaiasec-llm-gateway/$CLAUDE_ENV_NAME ./scripts/e2e_claude_code.sh
EOF
    ;;
  codex)
    exec env CODEX_PROFILE="$CODEX_PROFILE_NAME" MODEL_NAME="$CONFIG_NAME" \
      "$ROOT_DIR/scripts/e2e_codex.sh"
    ;;
  claude)
    exec env CLAUDE_GATEWAY_ENV="$HOME/.config/gaiasec-llm-gateway/$CLAUDE_ENV_NAME" MODEL_NAME="$CONFIG_NAME" GATEWAY_URL="$GATEWAY_URL" \
      "$ROOT_DIR/scripts/e2e_claude_code.sh"
    ;;
  *)
    echo "Unsupported CLIENT=$CLIENT. Use none|codex|claude" >&2
    exit 1
    ;;
esac
