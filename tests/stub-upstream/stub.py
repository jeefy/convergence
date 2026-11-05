"""OpenAI-compatible stub upstream (stdlib only). Echoes the last user message.
Records every received request body to /data/received.log (one JSON per line)
and serves it at GET /_received."""
import json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

LOG = "/tmp/received.log"


class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *a):
        sys.stderr.write("stub: " + fmt % a + "\n")

    def _send(self, code, body, ctype="application/json"):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.startswith("/v1/models") or self.path == "/api/tags":
            m = [{"id": "stub", "object": "model"}]
            self._send(200, json.dumps({"object": "list", "data": m, "models": [{"name": "stub"}]}).encode())
        elif self.path == "/_received":
            try:
                data = open(LOG, "rb").read()
            except FileNotFoundError:
                data = b""
            self._send(200, data, "text/plain")
        else:
            self._send(404, b"{}")

    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(n)
        with open(LOG, "ab") as f:
            f.write(raw.replace(b"\n", b" ") + b"\n")
        try:
            req = json.loads(raw)
        except ValueError:
            return self._send(400, b'{"error":"bad json"}')
        users = [m for m in req.get("messages", []) if m.get("role") == "user"]
        content = users[-1].get("content", "") if users else ""
        if isinstance(content, list):
            content = "".join(p.get("text", "") for p in content)
        model = req.get("model", "stub")
        base = {"id": "chatcmpl-stub", "created": 1, "model": model}
        if req.get("stream"):
            chunks = []
            half = len(content) // 2
            for i, piece in enumerate([content[:half], content[half:]]):
                d = {"role": "assistant", "content": piece} if i == 0 else {"content": piece}
                chunks.append({**base, "object": "chat.completion.chunk",
                               "choices": [{"index": 0, "delta": d, "finish_reason": None}]})
            chunks.append({**base, "object": "chat.completion.chunk",
                           "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
                           "usage": {"prompt_tokens": 7, "completion_tokens": 5, "total_tokens": 12}})
            body = "".join("data: %s\n\n" % json.dumps(c) for c in chunks) + "data: [DONE]\n\n"
            return self._send(200, body.encode(), "text/event-stream")
        resp = {**base, "object": "chat.completion",
                "choices": [{"index": 0, "message": {"role": "assistant", "content": content},
                             "finish_reason": "stop"}],
                "usage": {"prompt_tokens": 7, "completion_tokens": 5, "total_tokens": 12}}
        self._send(200, json.dumps(resp).encode())


ThreadingHTTPServer(("0.0.0.0", 8080), H).serve_forever()
