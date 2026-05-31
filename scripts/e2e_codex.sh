#!/usr/bin/env bash
set -euo pipefail

PROFILE="${CODEX_PROFILE:-${1:-}}"
MODEL_NAME="${MODEL_NAME:-minimax-m25}"
SANDBOX_MODE="${SANDBOX_MODE:-workspace-write}"
APPROVAL_POLICY="${APPROVAL_POLICY:-never}"

if ! command -v codex >/dev/null 2>&1; then
  echo "codex CLI is required" >&2
  exit 1
fi

if [ -z "$PROFILE" ]; then
  cat >&2 <<'EOF'
CODEX_PROFILE is required.

Example:
  GATEWAY_URL=http://127.0.0.1:8090 MODEL_NAME=minimax-m25 ./scripts/install_codex_profile.sh rosetta-openai
  CODEX_PROFILE=rosetta-openai scripts/e2e_codex.sh

The profile must point Codex at this gateway's OpenAI-compatible endpoint.
Recommended test prompts once it starts:
  - "Reply with exactly: pong"
  - "Use a tool named get_weather for Hangzhou before answering"
EOF
  exit 1
fi

echo "Launching Codex via profile=$PROFILE model=$MODEL_NAME"
exec codex \
  --profile "$PROFILE" \
  --model "$MODEL_NAME" \
  --sandbox "$SANDBOX_MODE" \
  --ask-for-approval "$APPROVAL_POLICY"
