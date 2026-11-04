#!/usr/bin/env python3
"""模拟 DSH（DeepSeek Harness）向 sub2api 发一轮对话，用于验证会话存档排除是否生效。

背景：DSH 的 deepseek-official 适配器不支持自定义请求头，服务端只能靠它自带的
`x-deepseek-harness-user-id`（机器级 UUID）排除归档。这个脚本用标准库复现同一个
指纹，可以随时做「带标识 / 不带标识」的对照实验，不需要打开 DSH。

用法：
    # 1. 模拟本机 DSH：带上机器标识，预期不产生会话存档
    python3 tools/simulate_dsh_request.py --mode exclude

    # 2. 通用跳过头（任何客户端都能用），预期不产生存档
    python3 tools/simulate_dsh_request.py --mode skip

    # 3. 对照组：不带任何标识，预期正常归档（验证排除逻辑没有误杀）
    python3 tools/simulate_dsh_request.py --mode control

密钥来源优先级：--api-key > $SUB2API_API_KEY > ~/.dsh/.credentials.yaml 的 DEEPSEEK_API_KEY。
每次请求只消耗极少额度（默认 max_tokens=8）。
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import sys
import time
import urllib.error
import urllib.request
import uuid

DEFAULT_BASE_URL = "https://sub2api.lechun.cc"
DEFAULT_MODEL = "deepseek-flash"
HARNESS_USER_ID_FILE = pathlib.Path.home() / ".dsh" / ".anonymous-user-id"
DSH_CREDENTIALS_FILE = pathlib.Path.home() / ".dsh" / ".credentials.yaml"


def load_api_key(explicit: str | None) -> str:
    if explicit:
        return explicit
    env_key = os.environ.get("SUB2API_API_KEY", "").strip()
    if env_key:
        return env_key
    # 极简解析：只找 refs 段里的 DEEPSEEK_API_KEY，避免引入 yaml 依赖
    if DSH_CREDENTIALS_FILE.exists():
        for line in DSH_CREDENTIALS_FILE.read_text(encoding="utf-8").splitlines():
            stripped = line.strip()
            if stripped.startswith("DEEPSEEK_API_KEY:"):
                value = stripped.split(":", 1)[1].strip().strip("'\"")
                if value:
                    return value
    sys.exit(
        "找不到 API Key：请用 --api-key 传入，或设置 SUB2API_API_KEY，"
        f"或在 {DSH_CREDENTIALS_FILE} 中配置 DEEPSEEK_API_KEY"
    )


def load_harness_user_id(explicit: str | None) -> str:
    if explicit:
        return explicit
    if HARNESS_USER_ID_FILE.exists():
        value = HARNESS_USER_ID_FILE.read_text(encoding="utf-8").strip()
        if value:
            return value
    sys.exit(
        f"找不到本机 DSH 标识：请用 --harness-id 传入，或确认 {HARNESS_USER_ID_FILE} 存在"
    )


def build_headers(args: argparse.Namespace, api_key: str) -> dict[str, str]:
    headers = {
        "content-type": "application/json",
        "accept": "application/json",
        "anthropic-version": "2023-06-01",
        "x-api-key": api_key,
        "authorization": f"Bearer {api_key}",
        "user-agent": "deepseek-harness/0.2.1-rc.5",
    }

    if args.mode == "exclude":
        # 关键：服务端名单命中的就是这个头
        headers["x-deepseek-harness-user-id"] = load_harness_user_id(args.harness_id)
        headers["x-deepseek-harness-session-id"] = args.session_id or str(uuid.uuid4())
    elif args.mode == "skip":
        # 通用跳过头，优先级最高，读到即从请求中删除，不转发上游
        headers["x-sub2api-archive"] = "skip"
    elif args.mode == "control":
        # 不带任何标识：UA 仍然是 DSH，但没有机器码，会正常归档
        headers["x-deepseek-harness-session-id"] = args.session_id or str(uuid.uuid4())

    return headers


def build_body(args: argparse.Namespace) -> dict:
    return {
        "model": args.model,
        "max_tokens": args.max_tokens,
        "stream": False,
        "system": "archive-exclusion probe",
        "messages": [
            {
                "role": "user",
                "content": args.prompt,
            }
        ],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="模拟 DSH 请求，验证 sub2api 会话存档排除")
    parser.add_argument(
        "--mode",
        choices=("exclude", "skip", "control"),
        default="exclude",
        help="exclude=带 DSH 机器标识（预期不归档）；skip=带跳过头（预期不归档）；control=不带标识（预期归档）",
    )
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="网关地址")
    parser.add_argument("--model", default=DEFAULT_MODEL, help="模型名")
    parser.add_argument("--prompt", default="忽略这条消息，只回复 OK", help="对话内容")
    parser.add_argument("--max-tokens", type=int, default=8, help="最大输出 token")
    parser.add_argument("--api-key", default=None, help="API Key，默认从环境变量或 DSH 凭据读取")
    parser.add_argument("--harness-id", default=None, help="DSH 机器标识，默认读 ~/.dsh/.anonymous-user-id")
    parser.add_argument("--session-id", default=None, help="会话 ID，默认随机生成")
    parser.add_argument("--timeout", type=float, default=60.0, help="请求超时秒数")
    parser.add_argument("--dry-run", action="store_true", help="只打印将要发送的请求，不实际调用")
    args = parser.parse_args()

    api_key = load_api_key(args.api_key)
    url = args.base_url.rstrip("/") + "/v1/messages"
    headers = build_headers(args, api_key)
    body = build_body(args)

    masked = {k: (v[:10] + "****" if k in ("x-api-key", "authorization") else v) for k, v in headers.items()}

    if args.dry_run:
        print(f"POST {url}")
        print(json.dumps(masked, indent=2, ensure_ascii=False))
        print(json.dumps(body, indent=2, ensure_ascii=False))
        return 0

    print(f"POST {url}  mode={args.mode}  model={args.model}")
    for key in ("user-agent", "x-deepseek-harness-user-id", "x-deepseek-harness-session-id", "x-sub2api-archive"):
        print(f"  {key}: {headers.get(key, '(未发送)')}")

    data = json.dumps(body).encode("utf-8")
    request = urllib.request.Request(url, data=data, headers=headers, method="POST")

    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            status = response.status
            raw = response.read().decode("utf-8", "replace")
            request_id = response.headers.get("x-request-id") or response.headers.get("request-id") or "-"
    except urllib.error.HTTPError as exc:
        status = exc.code
        raw = exc.read().decode("utf-8", "replace")
        request_id = exc.headers.get("x-request-id", "-") if exc.headers else "-"

    elapsed_ms = int((time.time() - started) * 1000)

    text = ""
    try:
        payload = json.loads(raw)
        parts = payload.get("content") or []
        if isinstance(parts, list):
            text = "".join(p.get("text", "") for p in parts if isinstance(p, dict))
        if not text and "choices" in payload:
            text = payload["choices"][0].get("message", {}).get("content", "")
        if not text and "error" in payload:
            text = json.dumps(payload["error"], ensure_ascii=False)[:200]
    except json.JSONDecodeError:
        text = raw[:200]

    print(f"\nHTTP {status}  {elapsed_ms}ms  request_id={request_id}")
    print(f"回复：{text.strip()[:200] or '(空)'}")
    print(
        "\n预期："
        + {
            "exclude": "不产生会话存档（服务端名单命中）",
            "skip": "不产生会话存档（跳过头命中）",
            "control": "正常产生 1 条会话存档",
        }[args.mode]
    )
    return 0 if status == 200 else 1


if __name__ == "__main__":
    raise SystemExit(main())
