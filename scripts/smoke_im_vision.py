"""Run scoped QA smoke tests against an already deployed server; never print keys."""
import base64
import hashlib
import json
import os
from pathlib import Path
import secrets
import sys
import time
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
KB = "ee340af4-f112-4e85-8f61-6841f542195d"
KEY_FILE = Path("/tmp/codex-vision-smoke-key")
RESULT_FILE = Path("/tmp/codex-vision-smoke-results.json")
KEY_NAME = "codex-vision-smoke-20260923"


def initialize():
    key = secrets.token_urlsafe(40)
    fd = os.open(str(KEY_FILE), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as out:
        out.write(key)
    digest = hashlib.sha256(key.encode()).hexdigest()
    print("INSERT INTO tenant_api_keys (tenant_id,name,key_hash,full_access,"
          "knowledge_base_ids,capabilities,expires_at) VALUES "
          f"(10000,'{KEY_NAME}','{digest}',false,'[\"{KB}\"]',"
          "'[\"chat\"]',NOW()+INTERVAL '1 hour');")


def request(method, path, body=None):
    payload = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(BASE + path, data=payload, method=method,
                                 headers={"X-API-Key": KEY_FILE.read_text(),
                                          "Content-Type": "application/json"})
    return urllib.request.urlopen(req, timeout=180)


def run(image_path):
    sessions, results = [], []
    image = "data:image/png;base64," + base64.b64encode(Path(image_path).read_bytes()).decode()

    def new_session(label):
        with request("POST", "/sessions", {"title": "codex-vision-smoke-" + label}) as response:
            data = json.load(response)
        session_id = data["data"]["id"]
        sessions.append(session_id)
        return session_id

    def ask(label, session_id, query, with_image=False):
        body = {"query": query, "agent_id": "builtin-quick-answer", "agent_enabled": False,
                "knowledge_base_ids": [KB], "disable_title": True, "channel": "api",
                "web_search_enabled": False}
        if with_image:
            body["images"] = [{"data": image}]
        started = time.monotonic()
        events = []
        with request("POST", "/knowledge-chat/" + session_id, body) as response:
            for line in response:
                text = line.decode().strip()
                if text.startswith("data:"):
                    events.append(json.loads(text[5:].strip()))
        kinds = sorted(set(e.get("response_type", "") for e in events))
        answer = "".join(e.get("content", "") for e in events if e.get("response_type") == "answer")
        passed = "complete" in kinds and "error" not in kinds and bool(answer)
        if label in ("fresh-image", "same-image-with-history", "image-only"):
            passed = passed and all(word in answer for word in ("ulimit", "61812", "<kb"))
            passed = passed and "No module named" not in answer and "pip install xrocket" not in answer
        if label == "image-follow-up":
            passed = passed and "nproc" in answer and "<kb" in answer
        if label == "outside-knowledge":
            passed = passed and any(word in answer for word in ("无法", "没有", "不包含", "未检索"))
        result = {"label": label, "session_id": session_id,
                  "seconds": round(time.monotonic() - started, 2), "passed": bool(passed),
                  "kinds": kinds, "events": events}
        results.append(result)
        RESULT_FILE.write_text(json.dumps(results, ensure_ascii=False, indent=2))
        print(json.dumps({k: result[k] for k in ("label", "session_id", "seconds", "kinds", "passed")}), flush=True)

    try:
        direct = new_session("fresh")
        ask("fresh-image", direct, "安装部署遇到图片上的错误要怎么处理？", True)
        ask("image-follow-up", direct, "刚才截图里的限制项具体叫什么？应该检查哪个参数？")
        history = new_session("different-history")
        ask("history-seed", history, "平台license到期了怎么办？")
        ask("same-image-with-history", history, "安装部署遇到图片上的错误要怎么处理？", True)
        boundary = new_session("boundary")
        ask("outside-knowledge", boundary, "明天北京天气怎么样？请给出实时天气。")
        image_only = new_session("image-only")
        ask("image-only", image_only, "请识别这张图片中的内容和报错，并依据知识库说明；如果图片看不清或知识库没有相关资料，请明确说明，不要猜测。", True)
    finally:
        for session_id in sessions:
            try:
                with request("DELETE", "/sessions/" + session_id) as response:
                    print("cleaned_test_session=" + session_id + " status=" + str(response.status), flush=True)
            except Exception as exc:
                print("test_session_cleanup_failed=" + session_id + " type=" + type(exc).__name__, flush=True)
        if results and not all(item["passed"] for item in results):
            raise AssertionError("One or more QA smoke checks failed; inspect result file")


if __name__ == "__main__":
    if sys.argv[1] == "init":
        initialize()
    else:
        run(sys.argv[1])
