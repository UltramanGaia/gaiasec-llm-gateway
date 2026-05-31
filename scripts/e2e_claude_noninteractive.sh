#!/usr/bin/env bash
set -euo pipefail

if ! command -v claude >/dev/null 2>&1; then
  echo "claude CLI is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

CLAUDE_GATEWAY_ENV="${CLAUDE_GATEWAY_ENV:-$HOME/.config/gaiasec-llm-gateway/claude-gateway.env}"
if [ -f "$CLAUDE_GATEWAY_ENV" ]; then
  # shellcheck disable=SC1090
  . "$CLAUDE_GATEWAY_ENV"
fi

GATEWAY_URL="${GATEWAY_URL:-${ANTHROPIC_BASE_URL:-http://127.0.0.1:8090}}"
MODEL_NAME="${MODEL_NAME:-minimax-m25}"
ANTHROPIC_API_KEY_VALUE="${ANTHROPIC_API_KEY_VALUE:-${ANTHROPIC_API_KEY:-dummy}}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/claude-gateway-e2e}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-40}"

mkdir -p "$ARTIFACT_DIR"

run_claude_json() {
  local output_file prompt
  output_file="$1"
  prompt="$2"

  if timeout "${TIMEOUT_SECONDS}s" bash -lc "
    export ANTHROPIC_BASE_URL='$GATEWAY_URL'
    export ANTHROPIC_API_KEY='$ANTHROPIC_API_KEY_VALUE'
    claude --bare -p --output-format json --model '$MODEL_NAME' \"$prompt\"
  " >"$output_file"; then
    echo "ok" >"${output_file}.status"
  else
    status=$?
    echo "$status" >"${output_file}.status"
  fi
}

echo "==> Claude plain-text validation"
run_claude_json \
  "$ARTIFACT_DIR/claude-pong.json" \
  "Reply with exactly: pong"

echo "==> Claude tool-path validation"
run_claude_json \
  "$ARTIFACT_DIR/claude-tool.json" \
  "Use a tool named get_weather for Hangzhou before answering"

python3 - "$ARTIFACT_DIR" <<'PY'
import json
import pathlib
import sys

artifact_dir = pathlib.Path(sys.argv[1])

def read_status(path: pathlib.Path) -> str:
    return path.read_text(encoding="utf-8").strip()

def summarize(path: pathlib.Path, expected_pong: bool) -> dict:
    status = read_status(path.with_suffix(path.suffix + ".status"))
    if status != "ok":
        return {"status": "timeout_or_error", "exit_code": status}

    raw = path.read_text(encoding="utf-8").strip()
    if not raw:
        return {"status": "empty_output"}

    try:
        payload = json.loads(raw)
    except Exception:
        payload = {"raw": raw}

    text = ""
    if isinstance(payload, dict):
        text = str(payload.get("result") or payload.get("text") or payload.get("content") or payload.get("raw") or "")
    else:
        text = str(payload)

    result = {
        "status": "passed",
        "message_preview": text[:200],
    }
    if expected_pong and text.strip() != "pong":
        result["status"] = "unexpected_output"
    return result

summary = {
    "plain_text": summarize(artifact_dir / "claude-pong.json", expected_pong=True),
    "tool_path": summarize(artifact_dir / "claude-tool.json", expected_pong=False),
}
print(json.dumps(summary, ensure_ascii=True, indent=2))
PY

echo "Artifacts saved to $ARTIFACT_DIR"
