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
UPSTREAM_TYPE="${UPSTREAM_TYPE:-openai_chat}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/gateway-e2e-smoke}"
ENABLE_STRUCTURED_OUTPUT_SMOKE="${ENABLE_STRUCTURED_OUTPUT_SMOKE:-0}"
ENABLE_RESPONSES_STRUCTURED_OUTPUT_SMOKE="${ENABLE_RESPONSES_STRUCTURED_OUTPUT_SMOKE:-0}"
ENABLE_VISION_SMOKE="${ENABLE_VISION_SMOKE:-0}"
ENABLE_FILE_SMOKE="${ENABLE_FILE_SMOKE:-0}"
ENABLE_RESPONSES_PREVIOUS_RESPONSE_SMOKE="${ENABLE_RESPONSES_PREVIOUS_RESPONSE_SMOKE:-0}"
ENABLE_RESPONSES_PREVIOUS_RESPONSE_STREAM_SMOKE="${ENABLE_RESPONSES_PREVIOUS_RESPONSE_STREAM_SMOKE:-0}"
ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_SMOKE="${ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_SMOKE:-0}"
ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_STREAM_SMOKE="${ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_STREAM_SMOKE:-0}"
ENABLE_RESPONSES_SMOKE="${ENABLE_RESPONSES_SMOKE:-0}"
ENABLE_ANTHROPIC_SMOKE="${ENABLE_ANTHROPIC_SMOKE:-0}"
ENABLE_RESPONSES_STREAM_SMOKE="${ENABLE_RESPONSES_STREAM_SMOKE:-0}"
ENABLE_ANTHROPIC_STREAM_SMOKE="${ENABLE_ANTHROPIC_STREAM_SMOKE:-0}"
ENABLE_RESPONSES_TOOL_SMOKE="${ENABLE_RESPONSES_TOOL_SMOKE:-0}"
ENABLE_ANTHROPIC_TOOL_SMOKE="${ENABLE_ANTHROPIC_TOOL_SMOKE:-0}"
ENABLE_RESPONSES_TOOL_STREAM_SMOKE="${ENABLE_RESPONSES_TOOL_STREAM_SMOKE:-0}"
ENABLE_ANTHROPIC_TOOL_STREAM_SMOKE="${ENABLE_ANTHROPIC_TOOL_STREAM_SMOKE:-0}"
VISION_IMAGE_URL="${VISION_IMAGE_URL:-https://example.com/a.png}"
FILE_ID="${FILE_ID:-file_123}"

if [ -z "$UPSTREAM_API_KEY" ]; then
  echo "UPSTREAM_API_KEY is required" >&2
  exit 1
fi

mkdir -p "$ARTIFACT_DIR"
export CONFIG_NAME UPSTREAM_MODEL UPSTREAM_BASE_URL UPSTREAM_API_KEY UPSTREAM_TYPE
export ENABLE_STRUCTURED_OUTPUT_SMOKE ENABLE_RESPONSES_STRUCTURED_OUTPUT_SMOKE ENABLE_VISION_SMOKE ENABLE_FILE_SMOKE ENABLE_RESPONSES_PREVIOUS_RESPONSE_SMOKE ENABLE_RESPONSES_PREVIOUS_RESPONSE_STREAM_SMOKE ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_SMOKE ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_STREAM_SMOKE ENABLE_RESPONSES_SMOKE ENABLE_ANTHROPIC_SMOKE ENABLE_RESPONSES_STREAM_SMOKE ENABLE_ANTHROPIC_STREAM_SMOKE ENABLE_RESPONSES_TOOL_SMOKE ENABLE_ANTHROPIC_TOOL_SMOKE ENABLE_RESPONSES_TOOL_STREAM_SMOKE ENABLE_ANTHROPIC_TOOL_STREAM_SMOKE

run_optional_http_capture() {
  local output_base method url body
  output_base="$1"
  method="$2"
  url="$3"
  body="$4"

  curl -sS -X "$method" \
    -D "${output_base}.headers" \
    -o "${output_base}.body" \
    -H 'Content-Type: application/json' \
    "$url" \
    -d "$body" \
    > /dev/null
  printf 'ok' > "${output_base}.status"
}

run_optional_stream_capture() {
  local output_base method url body
  output_base="$1"
  method="$2"
  url="$3"
  body="$4"

  curl -sS -N -X "$method" \
    -D "${output_base}.headers" \
    -o "${output_base}.body" \
    -H 'Content-Type: application/json' \
    "$url" \
    -d "$body" \
    > /dev/null
  printf 'ok' > "${output_base}.status"
}

CONFIG_PAYLOAD="$(python3 - <<'PY'
import json
import os

payload = {
    "name": os.environ["CONFIG_NAME"],
    "model_name": os.environ["UPSTREAM_MODEL"],
    "api_base_url": os.environ["UPSTREAM_BASE_URL"],
    "api_key": os.environ["UPSTREAM_API_KEY"],
    "upstream_type": os.environ["UPSTREAM_TYPE"],
    "max_tokens": 4096,
    "priority": 0,
    "max_concurrency": 0,
    "temperature": 0,
    "description": "Gateway smoke-test config",
    "supports_tools": True,
    "supports_stream": True,
    "supports_reasoning": True,
    "supports_json_schema": os.environ["ENABLE_STRUCTURED_OUTPUT_SMOKE"] == "1" or os.environ["ENABLE_RESPONSES_STRUCTURED_OUTPUT_SMOKE"] == "1",
    "supports_vision": os.environ["ENABLE_VISION_SMOKE"] == "1" or os.environ["ENABLE_FILE_SMOKE"] == "1",
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

if [ "$ENABLE_STRUCTURED_OUTPUT_SMOKE" = "1" ]; then
  echo "==> Non-stream, structured output"
  run_optional_http_capture \
    "$ARTIFACT_DIR/structured" \
    "POST" \
    "$GATEWAY_URL/v1/chat/completions" \
    "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":\"Return a JSON object with exactly one field named pong and value true.\"}],\"response_format\":{\"type\":\"json_schema\",\"json_schema\":{\"name\":\"pong_result\",\"schema\":{\"type\":\"object\",\"properties\":{\"pong\":{\"type\":\"boolean\"}},\"required\":[\"pong\"],\"additionalProperties\":false}}},\"stream\":false}"
fi

if [ "$ENABLE_RESPONSES_STRUCTURED_OUTPUT_SMOKE" = "1" ]; then
  echo "==> Non-stream, responses structured output"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses-structured" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Return a JSON object with exactly one field named pong and value true.\",\"text\":{\"format\":{\"type\":\"json_schema\",\"name\":\"pong_result\",\"schema\":{\"type\":\"object\",\"properties\":{\"pong\":{\"type\":\"boolean\"}},\"required\":[\"pong\"],\"additionalProperties\":false}}},\"stream\":false}"
fi

if [ "$ENABLE_VISION_SMOKE" = "1" ]; then
  echo "==> Non-stream, vision"
  run_optional_http_capture \
    "$ARTIFACT_DIR/vision" \
    "POST" \
    "$GATEWAY_URL/v1/chat/completions" \
    "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Describe the attached image in a few words.\"},{\"type\":\"image_url\",\"image_url\":{\"url\":\"$VISION_IMAGE_URL\"}}]}],\"stream\":false}"
fi

if [ "$ENABLE_FILE_SMOKE" = "1" ]; then
  echo "==> Non-stream, file"
  run_optional_http_capture \
    "$ARTIFACT_DIR/file" \
    "POST" \
    "$GATEWAY_URL/v1/chat/completions" \
    "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Acknowledge the attached file.\"},{\"type\":\"input_file\",\"file_id\":\"$FILE_ID\"}]}],\"stream\":false}"
fi

if [ "$ENABLE_RESPONSES_SMOKE" = "1" ]; then
  echo "==> Non-stream, responses endpoint"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":false}"
fi

if [ "$ENABLE_RESPONSES_PREVIOUS_RESPONSE_SMOKE" = "1" ]; then
  if [ "$UPSTREAM_TYPE" != "openai_responses" ]; then
    echo "==> Skipping responses previous_response_id follow-up (requires UPSTREAM_TYPE=openai_responses)"
  else
  echo "==> Non-stream, responses previous_response_id follow-up"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses-previous-first" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":false}"

  FIRST_RESPONSE_ID="$(python3 - "$ARTIFACT_DIR/responses-previous-first.headers" "$ARTIFACT_DIR/responses-previous-first.body" <<'PY'
import json
import pathlib
import sys

headers = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
body = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
if " 200 " not in headers:
    print("")
    raise SystemExit(0)
try:
    payload = json.loads(body)
except Exception:
    print("")
    raise SystemExit(0)
print(payload.get("id", ""))
PY
)"

  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_optional_http_capture \
      "$ARTIFACT_DIR/responses-previous-second" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"stream\":false}"
  fi
  fi
fi

if [ "$ENABLE_RESPONSES_PREVIOUS_RESPONSE_STREAM_SMOKE" = "1" ]; then
  if [ "$UPSTREAM_TYPE" != "openai_responses" ]; then
    echo "==> Skipping responses previous_response_id stream follow-up (requires UPSTREAM_TYPE=openai_responses)"
  else
  echo "==> Stream, responses previous_response_id follow-up"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses-previous-stream-first" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":false}"

  FIRST_RESPONSE_ID="$(python3 - "$ARTIFACT_DIR/responses-previous-stream-first.headers" "$ARTIFACT_DIR/responses-previous-stream-first.body" <<'PY'
import json
import pathlib
import sys

headers = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
body = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
if " 200 " not in headers:
    print("")
    raise SystemExit(0)
try:
    payload = json.loads(body)
except Exception:
    print("")
    raise SystemExit(0)
print(payload.get("id", ""))
PY
)"

  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_optional_stream_capture \
      "$ARTIFACT_DIR/responses-previous-stream" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"stream\":true}"
  fi
  fi
fi

if [ "$ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_SMOKE" = "1" ]; then
  if [ "$UPSTREAM_TYPE" != "openai_responses" ]; then
    echo "==> Skipping responses previous_response_id + tool follow-up (requires UPSTREAM_TYPE=openai_responses)"
  else
  echo "==> Non-stream, responses previous_response_id + tool follow-up"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses-previous-tool-first" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":false}"

  FIRST_RESPONSE_ID="$(python3 - "$ARTIFACT_DIR/responses-previous-tool-first.headers" "$ARTIFACT_DIR/responses-previous-tool-first.body" <<'PY'
import json
import pathlib
import sys

headers = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
body = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
if " 200 " not in headers:
    print("")
    raise SystemExit(0)
try:
    payload = json.loads(body)
except Exception:
    print("")
    raise SystemExit(0)
print(payload.get("id", ""))
PY
)"

  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_optional_http_capture \
      "$ARTIFACT_DIR/responses-previous-tool-second" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"tools\":[{\"type\":\"function\",\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"function\",\"name\":\"get_weather\"},\"stream\":false}"
  fi
  fi
fi

if [ "$ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_STREAM_SMOKE" = "1" ]; then
  if [ "$UPSTREAM_TYPE" != "openai_responses" ]; then
    echo "==> Skipping responses previous_response_id + tool stream follow-up (requires UPSTREAM_TYPE=openai_responses)"
  else
  echo "==> Stream, responses previous_response_id + tool follow-up"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses-previous-tool-stream-first" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":false}"

  FIRST_RESPONSE_ID="$(python3 - "$ARTIFACT_DIR/responses-previous-tool-stream-first.headers" "$ARTIFACT_DIR/responses-previous-tool-stream-first.body" <<'PY'
import json
import pathlib
import sys

headers = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
body = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
if " 200 " not in headers:
    print("")
    raise SystemExit(0)
try:
    payload = json.loads(body)
except Exception:
    print("")
    raise SystemExit(0)
print(payload.get("id", ""))
PY
)"

  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_optional_stream_capture \
      "$ARTIFACT_DIR/responses-previous-tool-stream" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"tools\":[{\"type\":\"function\",\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"function\",\"name\":\"get_weather\"},\"stream\":true}"
  fi
  fi
fi

if [ "$ENABLE_ANTHROPIC_SMOKE" = "1" ]; then
  echo "==> Non-stream, anthropic messages endpoint"
  run_optional_http_capture \
    "$ARTIFACT_DIR/anthropic" \
    "POST" \
    "$GATEWAY_URL/v1/messages" \
    "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Reply with exactly: pong\"}]}],\"max_tokens\":16,\"stream\":false}"
fi

if [ "$ENABLE_RESPONSES_STREAM_SMOKE" = "1" ]; then
  echo "==> Stream, responses endpoint"
  curl -fsS -N \
    -H 'Content-Type: application/json' \
    "$GATEWAY_URL/v1/responses" \
    -d "{\"model\":\"$CONFIG_NAME\",\"input\":\"Count from 1 to 3, one token at a time.\",\"stream\":true}" \
    >"$ARTIFACT_DIR/responses-stream.txt"
fi

if [ "$ENABLE_ANTHROPIC_STREAM_SMOKE" = "1" ]; then
  echo "==> Stream, anthropic messages endpoint"
  curl -fsS -N \
    -H 'Content-Type: application/json' \
    "$GATEWAY_URL/v1/messages" \
    -d "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Count from 1 to 3, one token at a time.\"}]}],\"max_tokens\":32,\"stream\":true}" \
    >"$ARTIFACT_DIR/anthropic-stream.txt"
fi

if [ "$ENABLE_RESPONSES_TOOL_SMOKE" = "1" ]; then
  echo "==> Non-stream, responses endpoint with tools"
  run_optional_http_capture \
    "$ARTIFACT_DIR/responses-tool" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\",\"tools\":[{\"type\":\"function\",\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"function\",\"name\":\"get_weather\"},\"stream\":false}"
fi

if [ "$ENABLE_ANTHROPIC_TOOL_SMOKE" = "1" ]; then
  echo "==> Non-stream, anthropic messages endpoint with tools"
  run_optional_http_capture \
    "$ARTIFACT_DIR/anthropic-tool" \
    "POST" \
    "$GATEWAY_URL/v1/messages" \
    "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\"}]}],\"tools\":[{\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"input_schema\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"tool\",\"name\":\"get_weather\"},\"max_tokens\":64,\"stream\":false}"
fi

if [ "$ENABLE_RESPONSES_TOOL_STREAM_SMOKE" = "1" ]; then
  echo "==> Stream, responses endpoint with tools"
  curl -fsS -N \
    -H 'Content-Type: application/json' \
    "$GATEWAY_URL/v1/responses" \
    -d "{\"model\":\"$CONFIG_NAME\",\"input\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\",\"tools\":[{\"type\":\"function\",\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"function\",\"name\":\"get_weather\"},\"stream\":true}" \
    >"$ARTIFACT_DIR/responses-tool-stream.txt"
fi

if [ "$ENABLE_ANTHROPIC_TOOL_STREAM_SMOKE" = "1" ]; then
  echo "==> Stream, anthropic messages endpoint with tools"
  curl -fsS -N \
    -H 'Content-Type: application/json' \
    "$GATEWAY_URL/v1/messages" \
    -d "{\"model\":\"$CONFIG_NAME\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\"}]}],\"tools\":[{\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"input_schema\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"tool\",\"name\":\"get_weather\"},\"max_tokens\":64,\"stream\":true}" \
    >"$ARTIFACT_DIR/anthropic-tool-stream.txt"
fi

python3 - "$ARTIFACT_DIR" <<'PY'
import json
import pathlib
import os
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
content_value = message.get("content")
content_text = None
if isinstance(content_value, str):
    content_text = content_value
elif isinstance(content_value, list):
    for part in content_value:
        if isinstance(part, dict) and part.get("type") in ("text", "output_text") and isinstance(part.get("text"), str):
            content_text = part["text"].strip()
            break
if content_text != "pong":
    fail(f"non-stream content mismatch: {content_value!r}")
if isinstance(content_value, str) and "<think>" in content_value:
    fail("non-stream content still contains <think>")

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
if '"content"' not in stream_notool:
    fail("stream no-tool missing content delta")

stream_tool = (artifact_dir / "stream-tool.txt").read_text(encoding="utf-8")
if "data: [DONE]" not in stream_tool:
    fail("stream tool missing [DONE]")
if '"tool_calls"' not in stream_tool:
    fail("stream tool missing tool_calls")
if '"get_weather"' not in stream_tool:
    fail("stream tool missing get_weather")

structured_status = "skipped"
if os.environ["ENABLE_STRUCTURED_OUTPUT_SMOKE"] == "1":
    structured = json.loads((artifact_dir / "structured.body").read_text(encoding="utf-8"))
    structured_choice = structured["choices"][0]
    structured_message = structured_choice["message"]
    if structured_choice["finish_reason"] != "stop":
        fail(f"structured output finish_reason mismatch: {structured_choice['finish_reason']}")
    structured_content = structured_message.get("content", "")
    if not structured_content:
        fail("structured output content missing")
    structured_status = "passed"

responses_structured_status = "skipped"
if os.environ["ENABLE_RESPONSES_STRUCTURED_OUTPUT_SMOKE"] == "1":
    responses_structured_headers = (artifact_dir / "responses-structured.headers").read_text(encoding="utf-8")
    responses_structured_body = (artifact_dir / "responses-structured.body").read_text(encoding="utf-8")
    if " 200 " in responses_structured_headers:
        responses_structured = json.loads(responses_structured_body)
        output = responses_structured.get("output") or []
        if not output:
            fail("responses structured output missing")
        first = output[0].get("content") if isinstance(output[0], dict) else None
        if not first:
            fail("responses structured content missing")
        responses_structured_status = "passed"
    elif any(code in responses_structured_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        responses_structured_status = "failed_upstream"
    else:
        fail("responses structured scenario returned unexpected status")

vision_status = "skipped"
if os.environ["ENABLE_VISION_SMOKE"] == "1":
    vision_headers = (artifact_dir / "vision.headers").read_text(encoding="utf-8")
    vision_body = (artifact_dir / "vision.body").read_text(encoding="utf-8")
    if " 200 " in vision_headers:
        vision = json.loads(vision_body)
        if vision["choices"][0]["message"].get("content") in (None, ""):
            fail("vision response content missing")
        vision_status = "passed"
    elif any(code in vision_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        vision_status = "failed_upstream"
    else:
        fail("vision scenario returned unexpected status")

file_status = "skipped"
if os.environ["ENABLE_FILE_SMOKE"] == "1":
    file_headers = (artifact_dir / "file.headers").read_text(encoding="utf-8")
    file_body = (artifact_dir / "file.body").read_text(encoding="utf-8")
    if " 200 " in file_headers:
        file_resp = json.loads(file_body)
        if file_resp["choices"][0]["message"].get("content") in (None, ""):
            fail("file response content missing")
        file_status = "passed"
    elif any(code in file_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        file_status = "failed_upstream"
    else:
        fail("file scenario returned unexpected status")

responses_status = "skipped"
if os.environ["ENABLE_RESPONSES_SMOKE"] == "1":
    responses_resp = json.loads((artifact_dir / "responses.body").read_text(encoding="utf-8"))
    output = responses_resp.get("output") or []
    if not output:
        fail("responses endpoint output missing")
    responses_status = "passed"

responses_previous_status = "skipped"
if os.environ["ENABLE_RESPONSES_PREVIOUS_RESPONSE_SMOKE"] == "1":
    first_headers = (artifact_dir / "responses-previous-first.headers").read_text(encoding="utf-8")
    first_body = (artifact_dir / "responses-previous-first.body").read_text(encoding="utf-8")
    if " 200 " in first_headers:
        first_resp = json.loads(first_body)
        first_id = first_resp.get("id", "")
        if not first_id:
            fail("responses previous_response_id first call missing id")
        second_headers_path = artifact_dir / "responses-previous-second.headers"
        second_body_path = artifact_dir / "responses-previous-second.body"
        if not second_headers_path.exists() or not second_body_path.exists():
            fail("responses previous_response_id follow-up was not executed")
        second_headers = second_headers_path.read_text(encoding="utf-8")
        second_body = second_body_path.read_text(encoding="utf-8")
        if " 200 " in second_headers:
            second_resp = json.loads(second_body)
            output = second_resp.get("output") or []
            if not output:
                fail("responses previous_response_id follow-up output missing")
            responses_previous_status = "passed"
        elif any(code in second_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
            responses_previous_status = "failed_upstream"
        else:
            fail("responses previous_response_id follow-up returned unexpected status")
    elif any(code in first_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        responses_previous_status = "failed_upstream"
    else:
        fail("responses previous_response_id first call returned unexpected status")

responses_previous_stream_status = "skipped"
if os.environ["ENABLE_RESPONSES_PREVIOUS_RESPONSE_STREAM_SMOKE"] == "1":
    first_headers = (artifact_dir / "responses-previous-stream-first.headers").read_text(encoding="utf-8")
    first_body = (artifact_dir / "responses-previous-stream-first.body").read_text(encoding="utf-8")
    if " 200 " in first_headers:
        first_resp = json.loads(first_body)
        first_id = first_resp.get("id", "")
        if not first_id:
            fail("responses previous_response_id stream first call missing id")
        stream_headers_path = artifact_dir / "responses-previous-stream.headers"
        stream_body_path = artifact_dir / "responses-previous-stream.body"
        if not stream_headers_path.exists() or not stream_body_path.exists():
            fail("responses previous_response_id stream follow-up was not executed")
        stream_headers = stream_headers_path.read_text(encoding="utf-8")
        stream_body = stream_body_path.read_text(encoding="utf-8")
        if " 200 " in stream_headers:
            if "data: [DONE]" not in stream_body:
                fail("responses previous_response_id stream missing [DONE]")
            if "event: response." not in stream_body:
                fail("responses previous_response_id stream missing response.* events")
            responses_previous_stream_status = "passed"
        elif any(code in stream_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
            responses_previous_stream_status = "failed_upstream"
        else:
            fail("responses previous_response_id stream follow-up returned unexpected status")
    elif any(code in first_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        responses_previous_stream_status = "failed_upstream"
    else:
        fail("responses previous_response_id stream first call returned unexpected status")

responses_previous_tool_status = "skipped"
if os.environ["ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_SMOKE"] == "1":
    first_headers = (artifact_dir / "responses-previous-tool-first.headers").read_text(encoding="utf-8")
    first_body = (artifact_dir / "responses-previous-tool-first.body").read_text(encoding="utf-8")
    if " 200 " in first_headers:
        first_resp = json.loads(first_body)
        first_id = first_resp.get("id", "")
        if not first_id:
            fail("responses previous_response_id tool first call missing id")
        second_headers_path = artifact_dir / "responses-previous-tool-second.headers"
        second_body_path = artifact_dir / "responses-previous-tool-second.body"
        if not second_headers_path.exists() or not second_body_path.exists():
            fail("responses previous_response_id tool follow-up was not executed")
        second_headers = second_headers_path.read_text(encoding="utf-8")
        second_body = second_body_path.read_text(encoding="utf-8")
        if " 200 " in second_headers:
            second_resp = json.loads(second_body)
            output = second_resp.get("output") or []
            if not any(isinstance(item, dict) and item.get("type") == "function_call" for item in output):
                fail("responses previous_response_id tool follow-up missing function_call item")
            responses_previous_tool_status = "passed"
        elif any(code in second_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
            responses_previous_tool_status = "failed_upstream"
        else:
            fail("responses previous_response_id tool follow-up returned unexpected status")
    elif any(code in first_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        responses_previous_tool_status = "failed_upstream"
    else:
        fail("responses previous_response_id tool first call returned unexpected status")

responses_previous_tool_stream_status = "skipped"
if os.environ["ENABLE_RESPONSES_PREVIOUS_RESPONSE_TOOL_STREAM_SMOKE"] == "1":
    first_headers = (artifact_dir / "responses-previous-tool-stream-first.headers").read_text(encoding="utf-8")
    first_body = (artifact_dir / "responses-previous-tool-stream-first.body").read_text(encoding="utf-8")
    if " 200 " in first_headers:
        first_resp = json.loads(first_body)
        first_id = first_resp.get("id", "")
        if not first_id:
            fail("responses previous_response_id tool stream first call missing id")
        stream_headers_path = artifact_dir / "responses-previous-tool-stream.headers"
        stream_body_path = artifact_dir / "responses-previous-tool-stream.body"
        if not stream_headers_path.exists() or not stream_body_path.exists():
            fail("responses previous_response_id tool stream follow-up was not executed")
        stream_headers = stream_headers_path.read_text(encoding="utf-8")
        stream_body = stream_body_path.read_text(encoding="utf-8")
        if " 200 " in stream_headers:
            if "data: [DONE]" not in stream_body:
                fail("responses previous_response_id tool stream missing [DONE]")
            if '"type":"function_call"' not in stream_body:
                fail("responses previous_response_id tool stream missing function_call item")
            if 'event: response.function_call_arguments.delta' not in stream_body:
                fail("responses previous_response_id tool stream missing function_call lifecycle")
            responses_previous_tool_stream_status = "passed"
        elif any(code in stream_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
            responses_previous_tool_stream_status = "failed_upstream"
        else:
            fail("responses previous_response_id tool stream follow-up returned unexpected status")
    elif any(code in first_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        responses_previous_tool_stream_status = "failed_upstream"
    else:
        fail("responses previous_response_id tool stream first call returned unexpected status")

anthropic_status = "skipped"
if os.environ["ENABLE_ANTHROPIC_SMOKE"] == "1":
    anthropic_resp = json.loads((artifact_dir / "anthropic.body").read_text(encoding="utf-8"))
    content = anthropic_resp.get("content") or []
    if not content:
        fail("anthropic endpoint content missing")
    anthropic_status = "passed"

responses_stream_status = "skipped"
if os.environ["ENABLE_RESPONSES_STREAM_SMOKE"] == "1":
    responses_stream = (artifact_dir / "responses-stream.txt").read_text(encoding="utf-8")
    if "data: [DONE]" not in responses_stream:
        fail("responses stream missing [DONE]")
    if "event: response." not in responses_stream:
        fail("responses stream missing response.* events")
    responses_stream_status = "passed"

anthropic_stream_status = "skipped"
if os.environ["ENABLE_ANTHROPIC_STREAM_SMOKE"] == "1":
    anthropic_stream = (artifact_dir / "anthropic-stream.txt").read_text(encoding="utf-8")
    if "event: message_start" not in anthropic_stream:
        fail("anthropic stream missing message_start")
    if "event: message_stop" not in anthropic_stream:
        fail("anthropic stream missing message_stop")
    anthropic_stream_status = "passed"

responses_tool_status = "skipped"
if os.environ["ENABLE_RESPONSES_TOOL_SMOKE"] == "1":
    responses_tool_headers = (artifact_dir / "responses-tool.headers").read_text(encoding="utf-8")
    responses_tool_body = (artifact_dir / "responses-tool.body").read_text(encoding="utf-8")
    if " 200 " in responses_tool_headers:
        responses_tool = json.loads(responses_tool_body)
        output = responses_tool.get("output") or []
        if not output:
            fail("responses tool output missing")
        if not any(isinstance(item, dict) and item.get("type") == "function_call" for item in output):
            fail("responses tool output missing function_call item")
        responses_tool_status = "passed"
    elif any(code in responses_tool_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        responses_tool_status = "failed_upstream"
    else:
        fail("responses tool scenario returned unexpected status")

anthropic_tool_status = "skipped"
if os.environ["ENABLE_ANTHROPIC_TOOL_SMOKE"] == "1":
    anthropic_tool_headers = (artifact_dir / "anthropic-tool.headers").read_text(encoding="utf-8")
    anthropic_tool_body = (artifact_dir / "anthropic-tool.body").read_text(encoding="utf-8")
    if " 200 " in anthropic_tool_headers:
        anthropic_tool = json.loads(anthropic_tool_body)
        content = anthropic_tool.get("content") or []
        if not content:
            fail("anthropic tool content missing")
        if not any(isinstance(item, dict) and item.get("type") == "tool_use" for item in content):
            fail("anthropic tool content missing tool_use block")
        anthropic_tool_status = "passed"
    elif any(code in anthropic_tool_headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        anthropic_tool_status = "failed_upstream"
    else:
        fail("anthropic tool scenario returned unexpected status")

responses_tool_stream_status = "skipped"
if os.environ["ENABLE_RESPONSES_TOOL_STREAM_SMOKE"] == "1":
    responses_tool_stream = (artifact_dir / "responses-tool-stream.txt").read_text(encoding="utf-8")
    if "data: [DONE]" not in responses_tool_stream:
        fail("responses tool stream missing [DONE]")
    if '"type":"function_call"' not in responses_tool_stream:
        fail("responses tool stream missing function_call item")
    if 'event: response.function_call_arguments.delta' not in responses_tool_stream:
        fail("responses tool stream missing function_call lifecycle")
    responses_tool_stream_status = "passed"

anthropic_tool_stream_status = "skipped"
if os.environ["ENABLE_ANTHROPIC_TOOL_STREAM_SMOKE"] == "1":
    anthropic_tool_stream = (artifact_dir / "anthropic-tool-stream.txt").read_text(encoding="utf-8")
    if "event: message_start" not in anthropic_tool_stream:
        fail("anthropic tool stream missing message_start")
    if "event: content_block_start" not in anthropic_tool_stream:
        fail("anthropic tool stream missing content_block_start")
    if "event: message_stop" not in anthropic_tool_stream:
        fail("anthropic tool stream missing message_stop")
    if '"type":"tool_use"' not in anthropic_tool_stream:
        fail("anthropic tool stream missing tool_use")
    anthropic_tool_stream_status = "passed"

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
    "structured_output": structured_status,
    "responses_structured_output": responses_structured_status,
    "vision_nonstream": vision_status,
    "file_nonstream": file_status,
    "responses_previous_response_nonstream": responses_previous_status,
    "responses_previous_response_stream": responses_previous_stream_status,
    "responses_previous_response_tool_nonstream": responses_previous_tool_status,
    "responses_previous_response_tool_stream": responses_previous_tool_stream_status,
    "responses_nonstream": responses_status,
    "anthropic_nonstream": anthropic_status,
    "responses_stream": responses_stream_status,
    "anthropic_stream": anthropic_stream_status,
    "responses_tool_nonstream": responses_tool_status,
    "anthropic_tool_nonstream": anthropic_tool_status,
    "responses_tool_stream": responses_tool_stream_status,
    "anthropic_tool_stream": anthropic_tool_stream_status,
}
print(json.dumps(summary, ensure_ascii=True, indent=2))
PY

echo "Artifacts saved to $ARTIFACT_DIR"
