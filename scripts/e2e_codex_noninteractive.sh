#!/usr/bin/env bash
set -euo pipefail

if ! command -v codex >/dev/null 2>&1; then
  echo "codex CLI is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8090}"
MODEL_NAME="${MODEL_NAME:-minimax-m25}"
CODEX_HOME_DIR="${CODEX_HOME_DIR:-/tmp/codex-gateway-e2e}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/codex-gateway-e2e}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"

mkdir -p "$CODEX_HOME_DIR" "$ARTIFACT_DIR"

run_codex_json() {
  local output_file prompt
  output_file="$1"
  prompt="$2"

  timeout "${TIMEOUT_SECONDS}s" bash -lc "
    CODEX_HOME='$CODEX_HOME_DIR' codex exec --json \
      --ignore-rules \
      --skip-git-repo-check \
      --sandbox read-only \
      -c 'model_provider=\"GatewayOpenAI\"' \
      -c 'model=\"$MODEL_NAME\"' \
      -c 'model_reasoning_effort=\"none\"' \
      -c 'model_providers.GatewayOpenAI={name=\"GatewayOpenAI\",base_url=\"$GATEWAY_URL/v1\",wire_api=\"responses\",requires_openai_auth=false}' \
      \"$prompt\"
  " >"$output_file"
}

echo "==> Codex plain-text validation"
run_codex_json \
  "$ARTIFACT_DIR/codex-pong.jsonl" \
  "Reply with exactly: pong"

echo "==> Codex tool-path validation"
run_codex_json \
  "$ARTIFACT_DIR/codex-tool.jsonl" \
  "Use a tool named get_weather for Hangzhou before answering"

python3 - "$ARTIFACT_DIR" <<'PY'
import json
import pathlib
import sys

artifact_dir = pathlib.Path(sys.argv[1])

def parse_jsonl(path: pathlib.Path):
    events = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or not line.startswith("{"):
            continue
        events.append(json.loads(line))
    return events

def latest_agent_message(events):
    messages = [
        event["item"]["text"]
        for event in events
        if event.get("type") == "item.completed"
        and isinstance(event.get("item"), dict)
        and event["item"].get("type") == "agent_message"
    ]
    return messages[-1] if messages else ""

def fail(msg: str) -> None:
    raise SystemExit(msg)

pong_events = parse_jsonl(artifact_dir / "codex-pong.jsonl")
tool_events = parse_jsonl(artifact_dir / "codex-tool.jsonl")

pong_text = latest_agent_message(pong_events).strip()
tool_text = latest_agent_message(tool_events).strip()

if pong_text != "pong":
    fail(f"codex plain-text check failed: {pong_text!r}")

if not tool_text:
    fail("codex tool-path check failed: empty agent message")

summary = {
    "plain_text": {
        "status": "passed",
        "message": pong_text,
    },
    "tool_path": {
        "status": "passed",
        "message_preview": tool_text[:200],
    },
}
print(json.dumps(summary, ensure_ascii=True, indent=2))
PY

echo "Artifacts saved to $ARTIFACT_DIR"
