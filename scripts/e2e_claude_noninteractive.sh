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
ANTHROPIC_MODEL="${ANTHROPIC_MODEL:-${MODEL_NAME:-minimax-m25}}"
ANTHROPIC_AUTH_TOKEN_VALUE="${ANTHROPIC_AUTH_TOKEN_VALUE:-${ANTHROPIC_AUTH_TOKEN:-${ANTHROPIC_API_KEY:-dummy}}}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/claude-gateway-e2e}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-40}"
CLAUDE_SETTINGS_PATH="${CLAUDE_SETTINGS_PATH:-$HOME/.claude/settings.json}"
CLAUDE_SETTINGS_BACKUP_PATH="${CLAUDE_SETTINGS_BACKUP_PATH:-$ARTIFACT_DIR/claude-settings.backup.json}"
CLAUDE_SETTINGS_EXISTED=0

mkdir -p "$ARTIFACT_DIR"

restore_claude_settings() {
  if [ -f "$CLAUDE_SETTINGS_BACKUP_PATH" ]; then
    if [ "$CLAUDE_SETTINGS_EXISTED" = "1" ]; then
      mv "$CLAUDE_SETTINGS_BACKUP_PATH" "$CLAUDE_SETTINGS_PATH"
    else
      rm -f "$CLAUDE_SETTINGS_PATH" "$CLAUDE_SETTINGS_BACKUP_PATH"
    fi
  fi
}

trap restore_claude_settings EXIT

if [ -f "$CLAUDE_SETTINGS_PATH" ]; then
  CLAUDE_SETTINGS_EXISTED=1
  cp "$CLAUDE_SETTINGS_PATH" "$CLAUDE_SETTINGS_BACKUP_PATH"
else
  echo "{}" >"$CLAUDE_SETTINGS_BACKUP_PATH"
fi

python3 - "$CLAUDE_SETTINGS_BACKUP_PATH" "$CLAUDE_SETTINGS_PATH" "$GATEWAY_URL" "$ANTHROPIC_AUTH_TOKEN_VALUE" <<'PY'
import json
import pathlib
import sys

backup_path = pathlib.Path(sys.argv[1])
target_path = pathlib.Path(sys.argv[2])
gateway_url = sys.argv[3].rstrip("/")
auth_token = sys.argv[4]

payload = json.loads(backup_path.read_text(encoding="utf-8"))
env = payload.get("env")
if not isinstance(env, dict):
    env = {}

env["ANTHROPIC_BASE_URL"] = gateway_url
env["ANTHROPIC_AUTH_TOKEN"] = auth_token
payload["env"] = env

target_path.parent.mkdir(parents=True, exist_ok=True)
target_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY

run_claude_json() {
  local output_file prompt
  output_file="$1"
  prompt="$2"

  if timeout "${TIMEOUT_SECONDS}s" bash -lc "
    export ANTHROPIC_BASE_URL='$GATEWAY_URL'
    export ANTHROPIC_AUTH_TOKEN='$ANTHROPIC_AUTH_TOKEN_VALUE'
    export ANTHROPIC_API_KEY='$ANTHROPIC_AUTH_TOKEN_VALUE'
    export ANTHROPIC_MODEL='$ANTHROPIC_MODEL'
    claude --bare -p --output-format json --model '$ANTHROPIC_MODEL' \"$prompt\"
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
