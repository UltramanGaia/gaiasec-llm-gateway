#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8090}"
CONFIG_NAME="${CONFIG_NAME:-minimax-m25}"
UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-http://172.31.29.10/v1}"
UPSTREAM_API_KEY="${UPSTREAM_API_KEY:-}"
UPSTREAM_MODEL="${UPSTREAM_MODEL:-MiniMax/MiniMax-M2.5}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/gateway-e2e-smoke}"

if [ -z "$UPSTREAM_API_KEY" ]; then
  echo "UPSTREAM_API_KEY is required" >&2
  exit 1
fi

mkdir -p "$ARTIFACT_DIR"
export CONFIG_NAME UPSTREAM_MODEL UPSTREAM_BASE_URL UPSTREAM_API_KEY

CONFIG_PAYLOAD="$(python3 - <<'PY'
import json
import os

payload = {
    "name": os.environ["CONFIG_NAME"],
    "model_name": os.environ["UPSTREAM_MODEL"],
    "api_base_url": os.environ["UPSTREAM_BASE_URL"],
    "api_key": os.environ["UPSTREAM_API_KEY"],
    "upstream_type": "openai_chat",
    "max_tokens": 4096,
    "priority": 0,
    "max_concurrency": 0,
    "temperature": 0,
    "description": "Gateway smoke-test config",
    "supports_tools": True,
    "supports_stream": True,
    "supports_reasoning": True,
    "supports_json_schema": False,
    "supports_vision": False,
    "supports_parallel_tool_calls": True,
    "enabled": True,
}
print(json.dumps(payload, ensure_ascii=True))
PY
)"

echo "==> Looking up config: $CONFIG_NAME"
curl -fsS "$GATEWAY_URL/api/model-configs" >"$ARTIFACT_DIR/model-configs.json"

CONFIG_ID="$(python3 - "$ARTIFACT_DIR/model-configs.json" "$CONFIG_NAME" <<'PY'
import json
import sys

items = json.load(open(sys.argv[1], "r", encoding="utf-8"))
target = sys.argv[2]
for item in items:
    if item.get("name") == target:
        print(item["id"])
        break
PY
)"

if [ -n "$CONFIG_ID" ]; then
  echo "==> Updating existing config id=$CONFIG_ID"
  curl -fsS -X PUT "$GATEWAY_URL/api/model-configs/$CONFIG_ID" \
    -H 'Content-Type: application/json' \
    -d "$CONFIG_PAYLOAD" >"$ARTIFACT_DIR/config-upsert.json"
else
  echo "==> Creating config"
  curl -fsS -X POST "$GATEWAY_URL/api/model-configs" \
    -H 'Content-Type: application/json' \
    -d "$CONFIG_PAYLOAD" >"$ARTIFACT_DIR/config-upsert.json"
  CONFIG_ID="$(python3 - "$ARTIFACT_DIR/config-upsert.json" <<'PY'
import json
import sys
print(json.load(open(sys.argv[1], "r", encoding="utf-8"))["id"])
PY
)"
fi

echo "==> Testing config id=$CONFIG_ID"
curl -fsS -X POST "$GATEWAY_URL/api/model-configs/$CONFIG_ID/test" \
  >"$ARTIFACT_DIR/config-test.json"

echo "==> Non-stream, no tools"
curl -fsS -D "$ARTIFACT_DIR/nonstream.headers" \
  -o "$ARTIFACT_DIR/nonstream.json" \
  -H 'Content-Type: application/json' \
  "$GATEWAY_URL/v1/chat/completions" \
  -d "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with exactly: pong\"}],\"stream\":false}"

echo "==> Stream, no tools"
curl -fsS -N \
  -H 'Content-Type: application/json' \
  "$GATEWAY_URL/v1/chat/completions" \
  -d "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":\"Count from 1 to 3, one token at a time.\"}],\"stream\":true}" \
  >"$ARTIFACT_DIR/stream-notool.txt"

echo "==> Non-stream, with tools"
curl -fsS -D "$ARTIFACT_DIR/tool.headers" \
  -o "$ARTIFACT_DIR/tool.json" \
  -H 'Content-Type: application/json' \
  "$GATEWAY_URL/v1/chat/completions" \
  -d "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\"}],\"tools\":[{\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}}],\"tool_choice\":\"required\",\"stream\":false}"

echo "==> Stream, with tools"
curl -fsS -N \
  -H 'Content-Type: application/json' \
  "$GATEWAY_URL/v1/chat/completions" \
  -d "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\"}],\"tools\":[{\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}}],\"tool_choice\":\"required\",\"stream\":true}" \
  >"$ARTIFACT_DIR/stream-tool.txt"

python3 - "$ARTIFACT_DIR" <<'PY'
import json
import pathlib
import sys

artifact_dir = pathlib.Path(sys.argv[1])

def fail(msg: str) -> None:
    raise SystemExit(msg)

config_test = json.loads((artifact_dir / "config-test.json").read_text(encoding="utf-8"))
if not config_test.get("success") or not config_test.get("data", {}).get("success"):
    fail("config test failed")

nonstream = json.loads((artifact_dir / "nonstream.json").read_text(encoding="utf-8"))
choice = nonstream["choices"][0]
message = choice["message"]
if choice["finish_reason"] != "stop":
    fail(f"non-stream finish_reason mismatch: {choice['finish_reason']}")
if message.get("content") != "pong":
    fail(f"non-stream content mismatch: {message.get('content')!r}")
if "<think>" in message.get("content", ""):
    fail("non-stream content still contains <think>")
if not message.get("reasoning_content"):
    fail("non-stream reasoning_content missing")

tool = json.loads((artifact_dir / "tool.json").read_text(encoding="utf-8"))
tool_choice = tool["choices"][0]
tool_message = tool_choice["message"]
tool_calls = tool_message.get("tool_calls") or []
if tool_choice["finish_reason"] != "tool_calls":
    fail(f"tool non-stream finish_reason mismatch: {tool_choice['finish_reason']}")
if not tool_calls:
    fail("tool non-stream tool_calls missing")
fn = tool_calls[0]["function"]
if fn.get("name") != "get_weather":
    fail(f"tool function name mismatch: {fn.get('name')!r}")
if "Hangzhou" not in fn.get("arguments", ""):
    fail(f"tool arguments missing Hangzhou: {fn.get('arguments')!r}")

stream_notool = (artifact_dir / "stream-notool.txt").read_text(encoding="utf-8")
if "data: [DONE]" not in stream_notool:
    fail("stream no-tool missing [DONE]")
if '"finish_reason":"stop"' not in stream_notool:
    fail("stream no-tool missing finish_reason stop")
if '"reasoning_content"' not in stream_notool:
    fail("stream no-tool missing reasoning_content")
if '"content"' not in stream_notool:
    fail("stream no-tool missing content delta")

stream_tool = (artifact_dir / "stream-tool.txt").read_text(encoding="utf-8")
if "data: [DONE]" not in stream_tool:
    fail("stream tool missing [DONE]")
if '"finish_reason":"tool_calls"' not in stream_tool:
    fail("stream tool missing finish_reason tool_calls")
if '"tool_calls"' not in stream_tool:
    fail("stream tool missing tool_calls")
if '"get_weather"' not in stream_tool:
    fail("stream tool missing get_weather")

summary = {
    "config_id": json.loads((artifact_dir / "config-upsert.json").read_text(encoding="utf-8"))["id"],
    "config_test": "passed",
    "nonstream_no_tools": {
        "content": message["content"],
        "reasoning_chars": len(message.get("reasoning_content", "")),
    },
    "nonstream_with_tools": {
        "tool_name": fn["name"],
        "arguments": fn["arguments"],
    },
    "stream_no_tools": "passed",
    "stream_with_tools": "passed",
}
print(json.dumps(summary, ensure_ascii=True, indent=2))
PY

echo "Artifacts saved to $ARTIFACT_DIR"
