#!/usr/bin/env python3
import argparse
import hashlib
import json
import subprocess
from collections import defaultdict
from datetime import datetime


def parse_args():
    parser = argparse.ArgumentParser(
        description="Infer agent request chains from request_logs without schema changes."
    )
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--user", default="sa")
    parser.add_argument("--password", default="qq123456")
    parser.add_argument("--database", default="gaiasec")
    parser.add_argument("--limit", type=int, default=500)
    parser.add_argument("--window-minutes", type=int, default=20)
    parser.add_argument("--min-chain", type=int, default=2)
    parser.add_argument("--samples", type=int, default=8)
    parser.add_argument("--max-steps-per-chain", type=int, default=30)
    return parser.parse_args()


def mysql_json_rows(args):
    limit = max(1, min(args.limit, 5000))
    query = f"""
SELECT JSON_OBJECT(
  'id', id,
  'created_at', DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s.%f'),
  'model_name', model_name,
  'backend_model_name', backend_model_name,
  'response_time', response_time,
  'request_len', CHAR_LENGTH(request),
  'response_len', CHAR_LENGTH(response),
  'request', request
)
FROM (
  SELECT id, created_at, model_name, backend_model_name, response_time, request, response
  FROM request_logs
  WHERE request IS NOT NULL AND request <> ''
  ORDER BY created_at DESC
  LIMIT {limit}
) recent
ORDER BY created_at ASC, id ASC;
"""
    cmd = [
        "mysql",
        f"-h{args.host}",
        f"-u{args.user}",
        f"-p{args.password}",
        "--batch",
        "--raw",
        "--skip-column-names",
        args.database,
        "-e",
        query,
    ]
    proc = subprocess.run(cmd, text=True, capture_output=True, check=True)
    rows = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        rows.append(json.loads(line))
    return rows


def parse_mysql_time(value):
    if not value:
        return None
    value = value.rstrip("0").rstrip(".") if "." in value else value
    for fmt in ("%Y-%m-%d %H:%M:%S.%f", "%Y-%m-%d %H:%M:%S"):
        try:
            return datetime.strptime(value, fmt)
        except ValueError:
            pass
    return None


def compact_text(value):
    return " ".join(str(value).strip().split())


def normalize_value(value):
    if value is None:
        return None
    if isinstance(value, str):
        return compact_text(value)
    if isinstance(value, list):
        return [normalize_value(item) for item in value]
    if isinstance(value, dict):
        keep = {}
        for key in sorted(value):
            if key in {"id", "created", "model", "usage", "logprobs"}:
                continue
            keep[key] = normalize_value(value[key])
        return keep
    return value


def normalize_message(message):
    if not isinstance(message, dict):
        return {"content": normalize_value(message)}
    keep_keys = [
        "role",
        "name",
        "content",
        "tool_calls",
        "tool_call_id",
        "type",
    ]
    normalized = {}
    for key in keep_keys:
        if key in message:
            normalized[key] = normalize_value(message[key])
    return normalized


def extract_messages(request_text):
    try:
        payload = json.loads(request_text)
    except Exception:
        return [], []

    messages = []
    system_items = []

    system = payload.get("system")
    if system:
        if isinstance(system, list):
            system_content = normalize_value(system)
        else:
            system_content = compact_text(system)
        system_items.append({"role": "system", "content": system_content})

    raw_messages = payload.get("messages")
    if isinstance(raw_messages, list):
        messages.extend(normalize_message(item) for item in raw_messages)

    prompt = payload.get("prompt")
    if not messages and prompt is not None:
        messages.append({"role": "user", "content": normalize_value(prompt)})

    if messages and messages[0].get("role") in {"system", "developer"}:
        system_items.append(messages[0])

    return system_items, messages


def digest(value):
    data = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(data.encode("utf-8")).hexdigest()


def first_user_message(messages):
    for message in messages:
        if message.get("role") == "user":
            return message
    return messages[0] if messages else None


def message_preview(messages):
    for message in reversed(messages):
        if message.get("role") == "user":
            return compact_text(json.dumps(message.get("content", ""), ensure_ascii=False))[:90]
    return ""


def build_features(row):
    system_items, messages = extract_messages(row.get("request") or "")
    canonical = {"messages": messages}
    if system_items:
        canonical["system"] = system_items

    root_parts = []
    root_parts.extend(system_items)
    first_user = first_user_message(messages)
    if first_user:
        root_parts.append(first_user)

    prefixes = []
    for i in range(1, len(messages)):
        prefix = {"messages": messages[:i]}
        if system_items:
            prefix["system"] = system_items
        prefixes.append((i, digest(prefix)))

    created_at = parse_mysql_time(row.get("created_at"))
    return {
        **row,
        "created_dt": created_at,
        "message_count": len(messages),
        "roles": ",".join(str(m.get("role", "?")) for m in messages),
        "request_key": digest(canonical),
        "root_key": digest(root_parts) if root_parts else "",
        "prefix_keys": prefixes,
        "preview": message_preview(messages),
    }


def minutes_between(a, b):
    if not a or not b:
        return 10**9
    return abs((b - a).total_seconds()) / 60


def infer_parents(items, window_minutes):
    by_request_key = {}
    recent_by_root = defaultdict(list)
    parent = {}
    reason = {}
    confidence = {}

    for item in items:
        best = None
        best_prefix_len = -1
        for prefix_len, prefix_key in item["prefix_keys"]:
            candidate = by_request_key.get(prefix_key)
            if not candidate:
                continue
            if candidate["id"] == item["id"]:
                continue
            if candidate["created_dt"] and item["created_dt"] and candidate["created_dt"] > item["created_dt"]:
                continue
            if prefix_len > best_prefix_len:
                best = candidate
                best_prefix_len = prefix_len

        if best:
            parent[item["id"]] = best["id"]
            reason[item["id"]] = f"prefix:{best_prefix_len}"
            confidence[item["id"]] = 0.98
        else:
            fallback = None
            for candidate in reversed(recent_by_root[item["root_key"]]):
                if minutes_between(candidate["created_dt"], item["created_dt"]) > window_minutes:
                    continue
                candidate_len = candidate.get("request_len") or 0
                item_len = item.get("request_len") or 0
                candidate_messages = candidate.get("message_count") or 0
                item_messages = item.get("message_count") or 0
                grew = candidate_len < item_len or candidate_messages < item_messages
                if grew:
                    fallback = candidate
                    break
            if fallback:
                parent[item["id"]] = fallback["id"]
                reason[item["id"]] = "root+time+len"
                confidence[item["id"]] = 0.72

        by_request_key[item["request_key"]] = item
        if item["root_key"]:
            recent_by_root[item["root_key"]].append(item)

    return parent, reason, confidence


def trace_root(item_id, parent):
    seen = set()
    current = item_id
    while current in parent and current not in seen:
        seen.add(current)
        current = parent[current]
    return current


def print_report(items, parent, reason, confidence, min_chain, samples, max_steps_per_chain):
    by_id = {item["id"]: item for item in items}
    groups = defaultdict(list)
    for item in items:
        groups[trace_root(item["id"], parent)].append(item)

    chains = [chain for chain in groups.values() if len(chain) >= min_chain]
    chains.sort(key=lambda chain: (len(chain), chain[-1]["created_dt"] or datetime.min), reverse=True)

    exact_edges = sum(1 for item_id, why in reason.items() if why.startswith("prefix"))
    fallback_edges = sum(1 for item_id, why in reason.items() if why == "root+time+len")

    print(f"rows_analyzed={len(items)}")
    print(f"chains_found={len(chains)} min_chain={min_chain}")
    print(f"edges={len(parent)} exact_prefix_edges={exact_edges} fallback_edges={fallback_edges}")
    print()

    for idx, chain in enumerate(chains[:samples], start=1):
        conf_values = [confidence.get(item["id"], 1.0) for item in chain if item["id"] in parent]
        avg_conf = sum(conf_values) / len(conf_values) if conf_values else 1.0
        start = chain[0]["created_at"]
        end = chain[-1]["created_at"]
        print(f"chain#{idx} steps={len(chain)} confidence={avg_conf:.2f} start={start} end={end}")
        visible_chain = chain[:max_steps_per_chain]
        for item in visible_chain:
            marker = "ROOT"
            if item["id"] in parent:
                marker = f"parent={parent[item['id']]} {reason[item['id']]} conf={confidence[item['id']]:.2f}"
            print(
                "  "
                f"id={item['id']} {marker} "
                f"msgs={item['message_count']} len={item.get('request_len') or 0} "
                f"model={item.get('model_name') or ''}/{item.get('backend_model_name') or ''} "
                f"preview={item['preview']}"
            )
        if len(chain) > len(visible_chain):
            print(f"  ... {len(chain) - len(visible_chain)} more steps")
        print()


def main():
    args = parse_args()
    rows = mysql_json_rows(args)
    items = [build_features(row) for row in rows]
    parent, reason, confidence = infer_parents(items, args.window_minutes)
    print_report(
        items,
        parent,
        reason,
        confidence,
        args.min_chain,
        args.samples,
        args.max_steps_per_chain,
    )


if __name__ == "__main__":
    main()
