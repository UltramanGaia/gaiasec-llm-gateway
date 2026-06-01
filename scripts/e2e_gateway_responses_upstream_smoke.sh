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
CONFIG_NAME="${CONFIG_NAME:-minimax-m25-responses-upstream}"
UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-http://172.31.29.10/v1}"
UPSTREAM_API_KEY="${UPSTREAM_API_KEY:-}"
UPSTREAM_MODEL="${UPSTREAM_MODEL:-MiniMax/MiniMax-M2.5}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/gateway-responses-upstream-smoke}"

if [ -z "$UPSTREAM_API_KEY" ]; then
  echo "UPSTREAM_API_KEY is required" >&2
  exit 1
fi

mkdir -p "$ARTIFACT_DIR"
export CONFIG_NAME UPSTREAM_MODEL UPSTREAM_BASE_URL UPSTREAM_API_KEY

run_http_capture() {
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

run_stream_capture() {
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
    "upstream_type": "openai_responses",
    "max_tokens": 4096,
    "priority": 0,
    "max_concurrency": 0,
    "temperature": 0,
    "description": "Responses-upstream smoke-test config",
    "supports_tools": True,
    "supports_stream": True,
    "supports_reasoning": True,
    "supports_json_schema": True,
    "supports_parallel_tool_calls": True,
    "supports_prompt_cache": True,
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
curl -fsS -X POST "$GATEWAY_URL/api/model-configs/$CONFIG_ID/test" >"$ARTIFACT_DIR/config-test.json"

echo "==> Responses baseline non-stream"
run_http_capture \
  "$ARTIFACT_DIR/responses-baseline" \
  "POST" \
  "$GATEWAY_URL/v1/responses" \
  "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":false}"

echo "==> Responses baseline stream"
run_stream_capture \
  "$ARTIFACT_DIR/responses-baseline-stream" \
  "POST" \
  "$GATEWAY_URL/v1/responses" \
  "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"stream\":true}"

echo "==> Responses previous_response_id non-stream"
run_http_capture \
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
payload = json.loads(body)
print(payload.get("id", ""))
PY
)"

if [ -n "$FIRST_RESPONSE_ID" ]; then
  run_http_capture \
    "$ARTIFACT_DIR/responses-previous-second" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"stream\":false}"

  echo "==> Responses previous_response_id stream"
  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_stream_capture \
      "$ARTIFACT_DIR/responses-previous-stream" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"stream\":true}"
  fi

  echo "==> Responses previous_response_id + tool non-stream"
  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_http_capture \
      "$ARTIFACT_DIR/responses-previous-tool-second" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"tools\":[{\"type\":\"function\",\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"function\",\"name\":\"get_weather\"},\"stream\":false}"
  fi

  echo "==> Responses previous_response_id + tool stream"
  if [ -n "$FIRST_RESPONSE_ID" ]; then
    run_stream_capture \
      "$ARTIFACT_DIR/responses-previous-tool-stream" \
      "POST" \
      "$GATEWAY_URL/v1/responses" \
      "{\"model\":\"$CONFIG_NAME\",\"input\":\"Use the provided function to get weather for Hangzhou. Do not answer directly.\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"tools\":[{\"type\":\"function\",\"name\":\"get_weather\",\"description\":\"Get weather by city\",\"parameters\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}},\"required\":[\"city\"]}}],\"tool_choice\":{\"type\":\"function\",\"name\":\"get_weather\"},\"stream\":true}"
  fi
fi

echo "==> Responses previous_response_id + structured output non-stream"
if [ -n "$FIRST_RESPONSE_ID" ]; then
  run_http_capture \
    "$ARTIFACT_DIR/responses-previous-structured-second" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Return a JSON object with exactly one field named pong and value true.\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"text\":{\"format\":{\"type\":\"json_schema\",\"name\":\"pong_result\",\"schema\":{\"type\":\"object\",\"properties\":{\"pong\":{\"type\":\"boolean\"}},\"required\":[\"pong\"],\"additionalProperties\":false}}},\"stream\":false}"
fi

echo "==> Responses previous_response_id + structured output stream"
if [ -n "$FIRST_RESPONSE_ID" ]; then
  run_stream_capture \
    "$ARTIFACT_DIR/responses-previous-structured-stream" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Return a JSON object with exactly one field named pong and value true.\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"text\":{\"format\":{\"type\":\"json_schema\",\"name\":\"pong_result\",\"schema\":{\"type\":\"object\",\"properties\":{\"pong\":{\"type\":\"boolean\"}},\"required\":[\"pong\"],\"additionalProperties\":false}}},\"stream\":true}"
fi

echo "==> Responses prompt_cache non-stream"
run_http_capture \
  "$ARTIFACT_DIR/responses-prompt-cache" \
  "POST" \
  "$GATEWAY_URL/v1/responses" \
  "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong\",\"prompt_cache_key\":\"cache-key\",\"prompt_cache_retention\":\"24h\",\"stream\":false}"

echo "==> Responses previous_response_id + prompt_cache non-stream"
if [ -n "$FIRST_RESPONSE_ID" ]; then
  run_http_capture \
    "$ARTIFACT_DIR/responses-previous-prompt-second" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"prompt_cache_key\":\"cache-key\",\"prompt_cache_retention\":\"24h\",\"stream\":false}"
fi

echo "==> Responses previous_response_id + prompt_cache stream"
if [ -n "$FIRST_RESPONSE_ID" ]; then
  run_stream_capture \
    "$ARTIFACT_DIR/responses-previous-prompt-stream" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"prompt_cache_key\":\"cache-key\",\"prompt_cache_retention\":\"24h\",\"stream\":true}"
fi

echo "==> Responses previous_response_id + include non-stream"
if [ -n "$FIRST_RESPONSE_ID" ]; then
  run_http_capture \
    "$ARTIFACT_DIR/responses-previous-include-second" \
    "POST" \
    "$GATEWAY_URL/v1/responses" \
    "{\"model\":\"$CONFIG_NAME\",\"input\":\"Reply with exactly: pong-2\",\"previous_response_id\":\"$FIRST_RESPONSE_ID\",\"include\":[\"reasoning.encrypted_content\"],\"stream\":false}"
fi

python3 - "$ARTIFACT_DIR" <<'PY'
import json
import pathlib
import sys

artifact_dir = pathlib.Path(sys.argv[1])

def classify_http(headers_path: pathlib.Path) -> str:
    if not headers_path.exists():
        return "missing"
    headers = headers_path.read_text(encoding="utf-8")
    if " 200 " in headers:
        return "passed"
    if any(code in headers for code in [" 400 ", " 401 ", " 403 ", " 404 ", " 409 ", " 422 ", " 429 ", " 500 ", " 502 ", " 503 ", " 504 "]):
        return "failed_upstream"
    return "unexpected"

def classify_followup(first_headers: pathlib.Path, second_headers: pathlib.Path) -> str:
    first = classify_http(first_headers)
    if first == "passed":
        return classify_http(second_headers)
    if first == "failed_upstream":
        return "failed_upstream"
    return first

summary = {
    "config_id": json.loads((artifact_dir / "config-upsert.json").read_text(encoding="utf-8"))["id"],
    "responses_baseline_nonstream": classify_http(artifact_dir / "responses-baseline.headers"),
    "responses_baseline_stream": classify_http(artifact_dir / "responses-baseline-stream.headers"),
    "responses_previous_response_nonstream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-second.headers"),
    "responses_previous_response_stream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-stream.headers"),
    "responses_previous_response_tool_nonstream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-tool-second.headers"),
    "responses_previous_response_tool_stream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-tool-stream.headers"),
    "responses_previous_response_structured_nonstream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-structured-second.headers"),
    "responses_previous_response_structured_stream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-structured-stream.headers"),
    "responses_prompt_cache_nonstream": classify_http(artifact_dir / "responses-prompt-cache.headers"),
    "responses_previous_response_prompt_cache_nonstream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-prompt-second.headers"),
    "responses_previous_response_prompt_cache_stream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-prompt-stream.headers"),
    "responses_previous_response_include_nonstream": classify_followup(artifact_dir / "responses-previous-first.headers", artifact_dir / "responses-previous-include-second.headers"),
}
(artifact_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=True, indent=2), encoding="utf-8")
print(json.dumps(summary, ensure_ascii=True, indent=2))
PY

echo "Artifacts saved to $ARTIFACT_DIR"
