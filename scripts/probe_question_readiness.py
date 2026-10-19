"""Exercise the deployed model's information-check contract without exposing credentials."""
import json
import pathlib
import re
import sys
import time
import urllib.request

model = json.load(sys.stdin)
prompts = pathlib.Path(__file__).resolve().parents[1].joinpath("internal/clarification/prompts.go").read_text(encoding="utf-8")
prompt = re.search(r"AssessmentPrompt = [`](.*?)[`]", prompts, re.S).group(1)
cases = [
    ("complete", {"current":"安装器前置检查提示ulimit-check，max process值过小，建议大于61812；双机环境，使用root安装。","previous":"升级失败了怎么办？\n补充：是KE平台升级","missing":"symptom"}),
    ("vague", {"current": "升级失败了怎么办？", "previous": "", "missing": ""}),
    ("followup", {"current": "KE平台，安装器提示ulimit-check，max process值过小", "previous": "升级失败了怎么办？", "missing": "component,symptom"}),
    ("concept", {"current": "xtwin服务的作用是什么？", "previous": "KE升级失败", "missing": "symptom"}),
]
for name, value in cases:
    body = {"model": model["name"], "messages": [{"role": "system", "content": prompt}, {"role": "user", "content": json.dumps(value, ensure_ascii=False)}], "temperature": 0, "max_tokens": 2048}
    if model["parameters"].get("extra_config", {}).get("thinking_control") != "none":
        body["thinking"] = {"type":"disabled"}
    request = urllib.request.Request(model["parameters"]["base_url"].rstrip("/") + "/chat/completions", data=json.dumps(body).encode(), headers={"Authorization": "Bearer " + model["parameters"]["api_key"], "Content-Type": "application/json"})
    started = time.monotonic()
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            result = json.load(response)
        choice = result["choices"][0]
        print(json.dumps({"case": name, "seconds": round(time.monotonic()-started, 2), "finish": choice.get("finish_reason"), "assessment": choice["message"].get("content"), "completion_tokens": result.get("usage", {}).get("completion_tokens")}, ensure_ascii=False), flush=True)
    except Exception as exc:
        print(json.dumps({"case": name, "error_type": type(exc).__name__}), flush=True)
