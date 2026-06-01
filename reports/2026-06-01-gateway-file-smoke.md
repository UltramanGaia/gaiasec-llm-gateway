# 2026-06-01 Gateway File Smoke Report

## Scope

This report records a focused real-upstream file-input smoke run against the current `gaiasec-llm-gateway` worktree.

Validated scenario:

- non-stream file input via `/v1/chat/completions`

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-20260601-file-rerun
```

## Result

Status: `failed upstream`

Observed behavior:

- the smoke suite completed successfully and classified the scenario as `failed_upstream`
- the gateway returned a non-2xx upstream error for file input

Relevant summary excerpt:

```json
{
  "file_nonstream": "failed_upstream"
}
```

## Interpretation

This is stronger than an unverified gap:

- the file scenario has now been exercised against the real upstream
- the current upstream/model combination does not accept this file-style input shape

It does **not** prove that the gateway has a transform bug; it proves that this upstream/model combination is currently incompatible with the tested file input path.
