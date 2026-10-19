"""Read model configuration from stdin; never print its credential."""
import base64
import json
import sys
import urllib.error
import urllib.request

model = json.load(sys.stdin)
with open(sys.argv[1], "rb") as image:
    encoded = base64.b64encode(image.read()).decode("ascii")
body = {
    "model": model["name"],
    "messages": [{"role": "user", "content": [
        {"type": "text", "text": "Transcribe the text in this image exactly. Do not infer missing text. If you cannot see the image say CANNOT_READ_IMAGE."},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64," + encoded}},
    ]}],
    "max_tokens": 350,
    "temperature": 0,
    "stream": False,
}
request = urllib.request.Request(
    model["parameters"]["base_url"].rstrip("/") + "/chat/completions",
    data=json.dumps(body).encode("utf-8"),
    headers={"Content-Type": "application/json", "Authorization": "Bearer " + model["parameters"]["api_key"]},
)
try:
    with urllib.request.urlopen(request, timeout=60) as response:
        result = json.load(response)
    print(json.dumps({"model": model["name"], "choices": result.get("choices"), "usage": result.get("usage")}, ensure_ascii=False))
except urllib.error.HTTPError as error:
    print(json.dumps({"status": error.code, "error": error.read(2000).decode("utf-8", "replace")}, ensure_ascii=False))
    sys.exit(1)
