# 2026-05-31 Gateway Client E2E Report

## Scope

This report records the end-to-end validation of `gaiasec-llm-gateway` against the real upstream:

- upstream base URL: `http://172.31.29.10/v1`
- upstream model: `MiniMax/MiniMax-M2.5`

Validated client and protocol paths:

- gateway smoke via `curl`
- Codex non-interactive via OpenAI `responses`
- Claude non-interactive via Anthropic `messages`

## Final Status

### Passed

- `curl` gateway smoke:
  - non-stream, no tools
  - stream, no tools
  - non-stream, with tools
  - stream, with tools
- Codex non-interactive:
  - plain-text turn returns `pong`
  - tool-path turn completes without upstream/tool-schema failure
- Claude non-interactive:
  - plain-text turn returns `pong`
  - tool-path turn completes and returns a non-empty assistant message

### Key Fixes Required To Reach Passing State

1. **OpenAI Chat upstream tool filtering**
   - Dropped unsupported built-in tool types such as `namespace` and `web_search` before forwarding to `MiniMax/MiniMax-M2.5`.
   - This removed real upstream `400` failures triggered by Codex.

2. **Response writer streaming preservation**
   - `responseWriterWrapper` now preserves `http.Flusher`.
   - Without this, real service runs downgraded streaming responses and broke `responses` SSE consumers.

3. **Chat-to-Responses stream lifecycle completion**
   - `chat.completion.chunk` is now converted into a more complete `response.*` SSE sequence:
     - `response.created`
     - `response.in_progress`
     - `response.output_item.added`
     - `response.content_part.added`
     - `response.output_text.delta`
     - `response.output_text.done`
     - `response.content_part.done`
     - `response.output_item.done`
     - `response.completed`
     - `response.done`
   - This was required for Codex to recognize streamed output as a valid assistant result.

4. **Anthropic stream reasoning lifecycle repair for Claude CLI**
   - Restored `reasoning_content -> thinking block` emission in the `chat -> anthropic SSE` conversion path.
   - Fixed the lifecycle so streamed `thinking` is stopped before the `text` block begins.
   - This preserved Anthropic-style reasoning output without breaking Claude non-interactive final result extraction.

5. **Claude non-interactive test isolation**
   - `scripts/e2e_claude_noninteractive.sh` now:
     - backs up `~/.claude/settings.json`
     - writes temporary gateway auth/base URL overrides
     - restores the original file on exit
   - This was required because Claude CLI was otherwise reading incompatible global config and failing auth or routing.

## Reasoning Streaming Status

### `reasoning_content -> thinking block` is restored

The gateway now preserves OpenAI Chat `reasoning_content` as Anthropic streamed `thinking` blocks for Claude-compatible clients.

The successful fix was not to suppress reasoning, but to repair block ordering:

1. emit `thinking` block start and deltas
2. emit `content_block_stop` for the `thinking` block
3. start the `text` block
4. emit `text_delta`

This sequencing keeps Claude CLI compatible while preserving reasoning fidelity.

### Remaining follow-up

Reasoning streaming is now functional again, but it still deserves broader parity verification later:

- compare emitted Anthropic SSE against `llm-rosetta` for mixed reasoning + tool-call + text responses
- verify whether upstreams with richer reasoning metadata require additional `thinking` fields
- expand regression coverage for multi-turn and mixed tool/reasoning streams

## Evidence

### Gateway smoke

The smoke script summary showed:

```json
{
  "config_test": "passed",
  "nonstream_no_tools": {
    "content": "pong"
  },
  "nonstream_with_tools": {
    "tool_name": "get_weather"
  },
  "stream_no_tools": "passed",
  "stream_with_tools": "passed"
}
```

### Codex non-interactive

The non-interactive Codex validation script produced:

```json
{
  "plain_text": {
    "status": "passed",
    "message": "pong"
  },
  "tool_path": {
    "status": "passed"
  }
}
```

### Claude non-interactive

After isolating `~/.claude/settings.json` and restoring streamed `thinking` with corrected block-stop ordering, the Claude non-interactive validation script produced:

```json
{
  "plain_text": {
    "status": "passed",
    "message_preview": "\n\npong"
  },
  "tool_path": {
    "status": "passed"
  }
}
```

## Summary

The gateway now passes real upstream E2E validation for:

- protocol smoke
- Codex client compatibility
- Claude client compatibility

Reasoning streaming for Anthropic-compatible clients is also restored and passes the current real-client validation path.
