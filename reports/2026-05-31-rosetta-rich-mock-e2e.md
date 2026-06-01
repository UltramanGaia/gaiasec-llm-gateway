# 2026-05-31 Rosetta Rich Mock Integration Report

## Scope

This report records mock-level richer integration coverage added for `gaiasec-llm-gateway` while closing the Rosetta gap plan.

The goal of this suite is not real upstream validation. It is to prove that the gateway handler and stream paths preserve higher-fidelity semantics that were previously dropped.

## Execution Entry Point

Run:

```bash
./scripts/e2e_gateway_rich_mock.sh
```

Artifacts are written to:

```text
/tmp/gateway-rich-mock-e2e
```

## Covered Checks

The script executes targeted `go test` cases covering:

- Responses request `previous_response_id` passthrough to Responses upstream
- Responses non-stream `custom_tool_call` same-protocol preservation
- Responses non-stream `custom_tool_call` conversion into Chat tool calls
- Responses non-stream `annotations` same-protocol preservation
- Responses non-stream `refusal` conversion into Chat `message.refusal`
- Responses non-stream `annotations` conversion into Chat content annotations
- Responses stream `annotations` conversion into Chat stream annotations
- Responses stream `reasoning` conversion into Chat `reasoning_content`
- Responses non-stream `mcp_call` conversion into Chat tool calls
- Responses non-stream `web_search_call` conversion into Chat tool calls
- Responses non-stream `image_generation_call` conversion into Chat tool calls
- Responses non-stream `custom_tool_call` conversion into Anthropic `tool_use`
- Responses non-stream `image_generation_call` conversion into Anthropic `tool_use`
- Responses non-stream `mcp_call` conversion into Anthropic `tool_use`
- Responses non-stream `web_search_call` conversion into Anthropic `tool_use`
- Anthropic non-stream `tool_use` conversion into Responses `function_call`
- Anthropic `thinking` stream conversion into Responses `reasoning` stream events
- Anthropic `tool_use` stream conversion into Responses `function_call` stream
- Anthropic `tool_use` stream conversion into Chat `tool_calls`
- Responses `reasoning` stream conversion into Anthropic `thinking`
- Chat richer stream conversion into Responses `refusal/audio/annotation` events
- Responses stream `custom_tool_call` conversion into Chat tool-call finish
- Responses stream `custom_tool_call` conversion into Anthropic `tool_use`
- Responses stream `mcp_call/web_search_call/image_generation_call/...` conversion into Chat tool-call finish
- Responses stream `mcp_call/web_search_call/image_generation_call/...` conversion into Anthropic `tool_use`
- Chat stream log aggregation preserving `audio`
- Chat stream log aggregation preserving `annotations`
- Anthropic `thinking` lifecycle preservation as a Responses `reasoning` item

## Expected Result Shape

Successful execution prints a JSON summary like:

```json
{
  "status": "passed",
  "checks": [
    {
      "name": "responses previous_response_id",
      "status": "passed"
    }
  ]
}
```

## Current Status

Status: `passed`

This suite provides repeatable evidence for richer mock integration paths that are broader than codec-only unit tests and cheaper than real-client E2E.

It now covers both:

- richer non-stream item preservation/mapping
- richer tool-call stream preservation/mapping

## Remaining Gaps

This report does not replace:

- real upstream client validation
- annotation stream parity
- full richer reasoning event parity across all protocol pairs
- broader replay and logging verification
