#!/usr/bin/env bash
set -euo pipefail

if ! command -v claude >/dev/null 2>&1; then
  echo "claude CLI is required" >&2
  exit 1
fi

CLAUDE_GATEWAY_ENV="${CLAUDE_GATEWAY_ENV:-$HOME/.config/gaiasec-llm-gateway/claude-gateway.env}"
if [ -f "$CLAUDE_GATEWAY_ENV" ]; then
  # shellcheck disable=SC1090
  . "$CLAUDE_GATEWAY_ENV"
fi

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8090}"
MODEL_NAME="${MODEL_NAME:-minimax-m25}"
ANTHROPIC_API_KEY_VALUE="${ANTHROPIC_API_KEY_VALUE:-dummy}"

echo "Launching Claude Code against $GATEWAY_URL model=$MODEL_NAME"
echo "Recommended test prompts:"
echo "  1. Reply with exactly: pong"
echo "  2. Use a tool named get_weather for Hangzhou before answering"

ANTHROPIC_BASE_URL="$GATEWAY_URL" \
ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY_VALUE" \
exec claude --model "$MODEL_NAME" --verbose
