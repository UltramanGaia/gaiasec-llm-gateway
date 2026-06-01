#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "go is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/gateway-rich-mock-e2e}"
mkdir -p "$ARTIFACT_DIR"

TEST_REGEX='TestResponsesHandlerPreservesPreviousResponseIDForResponsesUpstream|TestResponsesHandlerPassthroughsCustomToolCallFromResponsesUpstream|TestChatHandlerTransformsResponsesCustomToolCallToChatToolCall|TestResponsesHandlerPassthroughsAnnotationsFromResponsesUpstream|TestChatHandlerTransformsResponsesRefusalToChatMessage|TestChatHandlerTransformsResponsesAnnotationsToChatMessage|TestChatHandlerTransformsResponsesAnnotationStreamToChatStream|TestChatHandlerTransformsResponsesReasoningStreamToChatStream|TestChatHandlerTransformsResponsesMCPCallToChatToolCall|TestChatHandlerTransformsResponsesWebSearchCallToChatToolCall|TestChatHandlerTransformsResponsesImageGenerationCallToChatToolCall|TestAnthropicHandlerTransformsResponsesCustomToolCallToAnthropicToolUse|TestAnthropicHandlerTransformsResponsesImageGenerationCallToAnthropicToolUse|TestAnthropicHandlerTransformsResponsesMCPCallToAnthropicToolUse|TestAnthropicHandlerTransformsResponsesWebSearchCallToAnthropicToolUse|TestResponsesHandlerTransformsAnthropicThinkingStreamToResponsesReasoningStream|TestResponsesHandlerTransformsAnthropicToolUseToResponsesFunctionCall|TestResponsesHandlerTransformsAnthropicToolUseStreamToResponsesFunctionCallStream|TestChatHandlerTransformsAnthropicToolUseStreamToChatToolCall|TestAnthropicHandlerTransformsResponsesReasoningStreamToAnthropicThinkingStream|TestResponsesHandlerTransformsChatRicherStreamToResponsesEvents|TestChatHandlerTransformsResponsesCustomToolStreamToChatToolFinish|TestAnthropicHandlerTransformsResponsesCustomToolStreamToToolUse|TestChatHandlerTransformsAdditionalResponsesBuiltInStreamsToChatToolFinish|TestAnthropicHandlerTransformsAdditionalResponsesBuiltInStreamsToToolUse|TestBuildOpenAIStreamLogResponseIncludesAudio|TestBuildOpenAIStreamLogResponseIncludesAnnotations|TestResponsesEventsFromAnthropicFramePreservesThinkingAsReasoningItem'

echo "==> Running richer mock integration tests"
(
  cd "$ROOT_DIR"
  go test ./handlers ./protocol -run "$TEST_REGEX" -json
) >"$ARTIFACT_DIR/go-test.jsonl"

python3 - "$ARTIFACT_DIR/go-test.jsonl" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
events = []
for line in path.read_text(encoding="utf-8").splitlines():
    line = line.strip()
    if not line.startswith("{"):
        continue
    events.append(json.loads(line))

expected = {
    "TestResponsesHandlerPreservesPreviousResponseIDForResponsesUpstream": "responses previous_response_id",
    "TestResponsesHandlerPassthroughsCustomToolCallFromResponsesUpstream": "responses custom_tool_call passthrough",
    "TestChatHandlerTransformsResponsesCustomToolCallToChatToolCall": "responses custom_tool_call to chat",
    "TestResponsesHandlerPassthroughsAnnotationsFromResponsesUpstream": "responses annotations passthrough",
    "TestChatHandlerTransformsResponsesRefusalToChatMessage": "responses refusal to chat",
    "TestChatHandlerTransformsResponsesAnnotationsToChatMessage": "responses annotations to chat",
    "TestChatHandlerTransformsResponsesAnnotationStreamToChatStream": "responses annotations stream to chat",
    "TestChatHandlerTransformsResponsesReasoningStreamToChatStream": "responses reasoning stream to chat",
    "TestChatHandlerTransformsResponsesMCPCallToChatToolCall": "responses mcp_call to chat",
    "TestChatHandlerTransformsResponsesWebSearchCallToChatToolCall": "responses web_search_call to chat",
    "TestChatHandlerTransformsResponsesImageGenerationCallToChatToolCall": "responses image_generation_call to chat",
    "TestAnthropicHandlerTransformsResponsesCustomToolCallToAnthropicToolUse": "responses custom_tool_call to anthropic",
    "TestAnthropicHandlerTransformsResponsesImageGenerationCallToAnthropicToolUse": "responses image_generation_call to anthropic",
    "TestAnthropicHandlerTransformsResponsesMCPCallToAnthropicToolUse": "responses mcp_call to anthropic",
    "TestAnthropicHandlerTransformsResponsesWebSearchCallToAnthropicToolUse": "responses web_search_call to anthropic",
    "TestResponsesHandlerTransformsAnthropicThinkingStreamToResponsesReasoningStream": "anthropic thinking to responses reasoning stream",
    "TestResponsesHandlerTransformsAnthropicToolUseToResponsesFunctionCall": "anthropic tool_use to responses function_call",
    "TestResponsesHandlerTransformsAnthropicToolUseStreamToResponsesFunctionCallStream": "anthropic tool_use stream to responses function_call",
    "TestChatHandlerTransformsAnthropicToolUseStreamToChatToolCall": "anthropic tool_use stream to chat",
    "TestAnthropicHandlerTransformsResponsesReasoningStreamToAnthropicThinkingStream": "responses reasoning stream to anthropic",
    "TestResponsesHandlerTransformsChatRicherStreamToResponsesEvents": "chat richer stream to responses events",
    "TestChatHandlerTransformsResponsesCustomToolStreamToChatToolFinish": "responses custom_tool stream to chat",
    "TestAnthropicHandlerTransformsResponsesCustomToolStreamToToolUse": "responses custom_tool stream to anthropic",
    "TestChatHandlerTransformsAdditionalResponsesBuiltInStreamsToChatToolFinish": "responses built-in tool streams to chat",
    "TestAnthropicHandlerTransformsAdditionalResponsesBuiltInStreamsToToolUse": "responses built-in tool streams to anthropic",
    "TestBuildOpenAIStreamLogResponseIncludesAudio": "chat stream log audio aggregation",
    "TestBuildOpenAIStreamLogResponseIncludesAnnotations": "chat stream log annotations aggregation",
    "TestResponsesEventsFromAnthropicFramePreservesThinkingAsReasoningItem": "anthropic thinking reasoning lifecycle",
}

seen = {name: False for name in expected}
failures = []
for event in events:
    test = event.get("Test")
    action = event.get("Action")
    if test in seen and action == "pass":
        seen[test] = True
    if test in seen and action == "fail":
        failures.append(test)

missing = [name for name, ok in seen.items() if not ok]
if failures or missing:
    problems = {
        "failed": failures,
        "missing_pass": missing,
    }
    raise SystemExit(json.dumps(problems, ensure_ascii=True))

summary = {
    "status": "passed",
    "checks": [
        {"name": expected[name], "test": name, "status": "passed"}
        for name in expected
    ],
}
print(json.dumps(summary, ensure_ascii=True, indent=2))
PY

echo "Artifacts saved to $ARTIFACT_DIR"
