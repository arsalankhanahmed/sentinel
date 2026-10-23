#!/usr/bin/env python3
"""
Mock notification receiver for demos and automated tests.

Pretends to be Microsoft Teams / Zoom / WhatsApp webhooks so you can show
notifications live without real accounts. Standard library only.

  POST /hook/<channel>     store a notification (what Sentinel sends)
  GET  /api/messages       JSON list of received notifications (used by tests)
  DELETE /api/messages     clear
  GET  /health             200 OK   (also used as an "UP" HTTP probe target)
  GET  /down               503      (an always-failing "IIS" endpoint for outage demos)
  GET  /                   live HTML inbox that renders Teams cards / Zoom / WhatsApp
"""
import html
import json
import os
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MESSAGES = []
LOCK = threading.Lock()


def summarize(channel, body):
    """Extract a readable view of each channel's payload."""
    try:
        if "attachments" in body:  # Teams adaptive card
            card = body["attachments"][0]["content"]
            texts = [b.get("text", "") for b in card.get("body", []) if b.get("type") == "TextBlock"]
            facts = next((b["facts"] for b in card.get("body", []) if b.get("type") == "FactSet"), [])
            link = (card.get("actions") or [{}])[0].get("url", "")
            return {"title": texts[1] if len(texts) > 1 else texts[0], "badge": texts[0],
                    "lines": [f'{f["title"]}: {f["value"]}' for f in facts], "link": link}
        if "content" in body and "head" in body["content"]:  # Zoom
            text = body["content"]["body"][0]["text"]
            link = text.split("<")[-1].split("|")[0] if "<" in text else ""
            head = body["content"]["head"]["text"]
            badge = head[1:head.index("]")] if head.startswith("[") and "]" in head else ""
            return {"title": head, "badge": badge,
                    "lines": text.split("\n"), "link": link}
        if body.get("messaging_product") == "whatsapp":
            p = [x["text"] for x in body["template"]["components"][0]["parameters"]]
            link = p[4] if p[4].startswith("http") else ""  # resolved messages carry "-"
            return {"title": p[1], "badge": p[0], "lines": [f"Source: {p[2]}", f"Time: {p[3]}"], "link": link}
    except Exception:  # noqa: BLE001
        pass
    return {"title": channel, "badge": "", "lines": [json.dumps(body)[:300]], "link": ""}


PAGE = """<!doctype html><html><head><meta charset=utf-8><meta name=viewport content="width=device-width,initial-scale=1">
<meta http-equiv=refresh content=3><title>Mock Notification Inbox</title>
<style>body{font-family:Segoe UI,system-ui,sans-serif;background:#f3f2f1;margin:0;padding:24px;color:#242424}
h1{font-size:20px;margin:0 0 4px}.sub{color:#666;font-size:13px;margin-bottom:20px}
.card{background:#fff;border-radius:8px;box-shadow:0 1px 3px #0002;padding:16px;margin:0 0 12px;max-width:640px;border-left:5px solid #c4314b}
.card.RESOLVED{border-color:#237b4b}.ch{font-size:11px;color:#6264a7;font-weight:700;text-transform:uppercase}
.badge{font-size:12px;font-weight:700;color:#c4314b}.RESOLVED .badge{color:#237b4b}
.t{font-size:16px;font-weight:600;margin:4px 0 8px}.l{font-size:13px;color:#444;margin:2px 0}
a.btn{display:inline-block;margin-top:10px;background:#5b5fc7;color:#fff;padding:6px 14px;border-radius:4px;text-decoration:none;font-size:13px}
.when{float:right;font-size:11px;color:#999}</style></head><body>
<h1>Mock Notification Inbox</h1><div class=sub>Simulated Teams / Zoom / WhatsApp channels &middot; %d message(s) &middot; auto-refresh 3s &middot; raw data is never included</div>%s</body></html>"""


class Handler(BaseHTTPRequestHandler):
    def _send(self, code, body, ctype="application/json"):
        data = body.encode() if isinstance(body, str) else body
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(n)
        parts = self.path.split("?")[0].strip("/").split("/")
        channel = parts[1] if parts[0] == "hook" and len(parts) > 1 else (parts[-1] or "default")
        try:
            body = json.loads(raw or b"{}")
        except json.JSONDecodeError:
            body = {"raw": raw.decode(errors="replace")}
        with LOCK:
            MESSAGES.append({"channel": channel, "received_at": time.time(), "body": body,
                             "view": summarize(channel, body), "auth": self.headers.get("Authorization", "")})
        self._send(200, '{"ok":true}')

    def do_DELETE(self):
        with LOCK:
            MESSAGES.clear()
        self._send(200, '{"ok":true}')

    def do_GET(self):
        if self.path.startswith("/health"):
            return self._send(200, '{"status":"up"}')
        if self.path.startswith("/down"):
            return self._send(503, '{"status":"service unavailable"}')
        if self.path.startswith("/api/messages"):
            with LOCK:
                return self._send(200, json.dumps(MESSAGES))
        with LOCK:
            items = list(reversed(MESSAGES))
        cards = "".join(
            f'<div class="card {html.escape(m["view"]["badge"])}"><span class=when>{time.strftime("%H:%M:%S", time.localtime(m["received_at"]))}</span>'
            f'<div class=ch>{html.escape(m["channel"])}</div><div class=badge>{html.escape(m["view"]["badge"])}</div>'
            f'<div class=t>{html.escape(m["view"]["title"])}</div>'
            + "".join(f'<div class=l>{html.escape(l)}</div>' for l in m["view"]["lines"] if l.strip())
            + (f'<a class=btn href="{html.escape(m["view"]["link"])}">Open Incident</a>' if m["view"]["link"] else "")
            + "</div>" for m in items)
        self._send(200, PAGE % (len(items), cards or "<p>No notifications yet.</p>"), "text/html; charset=utf-8")

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "9911"))
    print(f"mock webhook listening on :{port}", flush=True)
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()
