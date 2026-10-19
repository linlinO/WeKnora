"""Bounded live QA tests. Credential stays in the container and is never printed."""
import hashlib
import json
import os
from pathlib import Path
import secrets
import sys
import time
import urllib.request

BASE = "http://app:8080/api/v1"
KB = "ee340af4-f112-4e85-8f61-6841f542195d"
ROOT = Path("/tmp/codex-clarification")
KEY_FILE = ROOT / "smoke-key"
RESULT_FILE = ROOT / "smoke-results.json"
KEY_NAME = "codex-clarification-smoke-20261014"

def check(label, answer, events):
    serialized = json.dumps(events, ensure_ascii=False)
    refs = [ref for event in events if event.get("response_type") == "references"
            for ref in event.get("knowledge_references", [])]
    grounded = any(ref.get("knowledge_id") and ref.get("knowledge_base_id") == KB for ref in refs)
    if label in ("vague", "partial", "isolated"):
        passed = "clarification" in serialized and "请补充" in answer and "pip install" not in answer
        passed = passed and not any("memory" in str(e.get("response_type", "")).lower() for e in events)
        if label == "partial":
            passed = passed and "哪个产品" not in answer
        return passed
    if label == "complete":
        # The UI exposes sources through the references event even when the
        # model omits inline <kb> citation markers. Check the actual cited KB
        # and its applicable source text, not a particular Markdown spelling.
        return grounded and "readiness_kind" not in serialized and "ulimit" in answer.lower() and "102400" in answer and any("ulimit" in r.get("content", "").lower() for r in refs)
    return grounded and "xtwin" in answer.lower() and "ulimit" not in answer.lower()

def request(method, path, body=None):
    payload = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(BASE+path, data=payload, method=method, headers={"X-API-Key": KEY_FILE.read_text(), "Content-Type": "application/json"})
    return urllib.request.urlopen(req, timeout=240)

def initialize():
    key = secrets.token_urlsafe(40)
    fd = os.open(str(KEY_FILE), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as out:
        out.write(key)
    digest = hashlib.sha256(key.encode()).hexdigest()
    print("INSERT INTO tenant_api_keys (tenant_id,name,key_hash,full_access,knowledge_base_ids,capabilities,expires_at) VALUES "
          f"(10000,'{KEY_NAME}','{digest}',false,'[\"{KB}\"]','[\"chat\"]',NOW()+INTERVAL '1 hour');")

def run():
    sessions, results = [], []
    def new_session(label):
        with request("POST", "/sessions", {"title":"部署验收_追问_20261014_"+label}) as response:
            session = json.load(response)["data"]["id"]
        sessions.append(session)
        return session
    def ask(label, session, query, expected):
        body = {"query":query, "agent_id":"builtin-quick-answer", "agent_enabled":False, "knowledge_base_ids":[KB], "disable_title":True, "channel":"api", "web_search_enabled":False}
        started = time.monotonic()
        events = []
        with request("POST", "/knowledge-chat/"+session, body) as response:
            for line in response:
                text = line.decode().strip()
                if text.startswith("data:"):
                    events.append(json.loads(text[5:].strip()))
        answer = "".join(e.get("content","") for e in events if e.get("response_type")=="answer")
        passed = check(label, answer, events)
        result = {"case":label,"session_id":session,"seconds":round(time.monotonic()-started,2),"passed":passed,"answer":answer,"events":events}
        results.append(result)
        RESULT_FILE.write_text(json.dumps(results,ensure_ascii=False,indent=2))
        print(json.dumps({k:result[k] for k in ("case","session_id","seconds","passed","answer")},ensure_ascii=False),flush=True)
    try:
        a = new_session("连续补充")
        ask("vague", a, "升级失败了怎么办？", "clarify")
        ask("partial", a, "是KE平台升级", "clarify")
        ask("complete", a, "安装器前置检查提示ulimit-check，max process值过小，建议大于61812；双机环境，使用root安装。", "answer")
        ask("topic-switch", a, "xtwin服务的作用是什么？", "concept")
        b = new_session("隔离")
        ask("isolated", b, "报错是ulimit-check，max process值过小，建议大于61812，该怎么解决？", "clarify")
    finally:
        for session in sessions:
            try:
                with request("DELETE","/sessions/"+session) as response:
                    print("removed_test_session="+session,flush=True)
            except Exception as exc:
                print("cleanup_failed="+session+" "+type(exc).__name__,flush=True)
    if not all(r["passed"] for r in results):
        raise AssertionError("Live clarification tests failed; inspect saved report")

if __name__=="__main__":
    if sys.argv[1]=="init":
        initialize()
    elif sys.argv[1]=="verify":
        results = json.loads(RESULT_FILE.read_text())
        for result in results:
            result["passed"] = check(result["case"], result["answer"], result["events"])
            print(json.dumps({k:result[k] for k in ("case","session_id","seconds","passed")},ensure_ascii=False))
        RESULT_FILE.write_text(json.dumps(results,ensure_ascii=False,indent=2))
        if not all(r["passed"] for r in results):
            raise AssertionError("Recorded live test result failed verification")
    else:
        run()
