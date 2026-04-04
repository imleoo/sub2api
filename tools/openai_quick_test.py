#!/usr/bin/env python3
import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from urllib.parse import urljoin


def load_env_file(path: str) -> None:
    if not path:
        return
    try:
        with open(path, "r", encoding="utf-8") as handle:
            for raw in handle:
                line = raw.strip()
                if not line or line.startswith("#"):
                    continue
                if line.startswith("export "):
                    line = line[len("export ") :].strip()
                if "=" not in line:
                    continue
                key, value = line.split("=", 1)
                key = key.strip()
                value = value.strip()
                if not key:
                    continue
                if (value.startswith('"') and value.endswith('"')) or (
                    value.startswith("'") and value.endswith("'")
                ):
                    value = value[1:-1]
                os.environ.setdefault(key, value)
    except FileNotFoundError:
        return


def pick_env(*keys: str) -> str:
    for k in keys:
        v = os.getenv(k)
        if v is not None and str(v).strip() != "":
            return str(v)
    return ""


def build_chat_completions_url(base_url: str) -> str:
    normalized = (base_url or "").strip()
    if not normalized:
        raise ValueError("base_url is required")
    if not normalized.endswith("/"):
        normalized += "/"
    return urljoin(normalized, "v1/chat/completions")


def post_json(url: str, payload: dict, headers: dict, timeout_s: float) -> tuple[int, dict]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(url, data=body, method="POST")
    for k, v in headers.items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=timeout_s) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            return resp.status, json.loads(raw)
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            data = {"error": {"message": raw}}
        return e.code, data


def extract_text(data: dict) -> str | None:
    choices = data.get("choices")
    if isinstance(choices, list) and choices:
        first = choices[0] or {}
        message = first.get("message") or {}
        if isinstance(message, dict):
            content = message.get("content")
            if isinstance(content, str):
                return content
        delta = first.get("delta") or {}
        if isinstance(delta, dict):
            content = delta.get("content")
            if isinstance(content, str):
                return content
        text = first.get("text")
        if isinstance(text, str):
            return text
    return None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--env-file", default=None)
    parser.add_argument("--base-url", default=None)
    parser.add_argument("--api-key", default=None)
    parser.add_argument("--model", default=None)
    parser.add_argument("--prompt", default="Hello! Please reply with a short greeting.")
    parser.add_argument("--timeout", type=float, default=60.0)
    parser.add_argument("--max-tokens", type=int, default=256)
    args = parser.parse_args()

    default_env_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".env")
    env_path = args.env_file or (default_env_path if os.path.exists(default_env_path) else "")
    load_env_file(env_path)

    base_url = args.base_url or pick_env("OPENAI_BASE_URL", "BASE_URL", "URL") or "https://openclaw.zhiguo.fan/"
    api_key = args.api_key or pick_env("OPENAI_API_KEY", "API_KEY", "KEY")
    model = args.model or pick_env("OPENAI_MODEL", "MODEL") or "gpt-5.4"

    if not api_key:
        sys.stderr.write(
            "missing api key: set OPENAI_API_KEY/BASE_URL/MODEL in tools/.env (export ...) or pass --api-key\n"
        )
        return 2

    url = build_chat_completions_url(base_url)
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": args.prompt}],
        "stream": False,
        "max_tokens": args.max_tokens,
    }
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}",
    }

    status, data = post_json(url, payload, headers, args.timeout)
    text = extract_text(data)

    sys.stdout.write(f"POST {url}\n")
    sys.stdout.write(f"HTTP {status}\n")
    if text is not None:
        sys.stdout.write("\n")
        sys.stdout.write(text)
        sys.stdout.write("\n")
        return 0 if 200 <= status < 300 else 1

    sys.stdout.write("\n")
    sys.stdout.write(json.dumps(data, ensure_ascii=False, indent=2))
    sys.stdout.write("\n")
    return 0 if 200 <= status < 300 else 1


if __name__ == "__main__":
    raise SystemExit(main())
