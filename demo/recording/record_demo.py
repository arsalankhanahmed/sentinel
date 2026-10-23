#!/usr/bin/env python3
"""
Records a narrated walkthrough video of the Sentinel demo (Playwright, Chromium).

Prerequisites: demo stack running and seeded (see docs/DEMO_GUIDE.md), plus
    pip install playwright && playwright install chromium

    python demo/recording/record_demo.py --out sentinel-demo.webm
    ffmpeg -i sentinel-demo.webm -c:v libx264 -pix_fmt yuv420p sentinel-demo.mp4

Environment:
    SENTINEL_URL (default http://localhost:8080)
    MOCK_URL     (default http://localhost:9911)
    SIMULATE_CMD command that injects app errors   (default ./demo/scripts/simulate-errors.sh)
    FIX_CMD      command that fixes the business data (default ./demo/scripts/fix-data.sh)
"""
import argparse
import os
import shutil
import subprocess
import tempfile
import time

from playwright.sync_api import sync_playwright

BASE = os.environ.get("SENTINEL_URL", "http://localhost:8080")
MOCK = os.environ.get("MOCK_URL", "http://localhost:9911")
SIMULATE = os.environ.get("SIMULATE_CMD", "./demo/scripts/simulate-errors.sh")
FIX = os.environ.get("FIX_CMD", "./demo/scripts/fix-data.sh")
W, H = 1280, 720

# Fake cursor + caption bar injected into every page so the video is self-explanatory.
OVERLAY_JS = """
(() => {
  if (window.__sentinelOverlay) return; window.__sentinelOverlay = true;
  const add = () => {
    const c = document.createElement('div');
    c.id = '__cursor';
    c.style.cssText = 'position:fixed;z-index:2147483647;width:22px;height:22px;margin:-11px 0 0 -11px;border-radius:50%;' +
      'background:rgba(250,204,21,.55);border:2px solid #facc15;pointer-events:none;transition:transform .12s;left:-50px;top:-50px';
    document.body.appendChild(c);
    document.addEventListener('mousemove', e => { c.style.left = e.clientX + 'px'; c.style.top = e.clientY + 'px'; }, true);
    document.addEventListener('mousedown', () => c.style.transform = 'scale(.6)', true);
    document.addEventListener('mouseup', () => c.style.transform = 'scale(1)', true);
    const cap = document.createElement('div');
    cap.id = '__caption';
    cap.style.cssText = 'position:fixed;z-index:2147483646;left:50%;bottom:22px;transform:translateX(-50%);max-width:86%;' +
      'background:rgba(15,23,42,.92);color:#f8fafc;font:600 17px/1.45 Segoe UI,system-ui,sans-serif;padding:12px 20px;' +
      'border-radius:10px;border:1px solid #334155;box-shadow:0 8px 30px rgba(0,0,0,.45);text-align:center;display:none';
    document.body.appendChild(cap);
    const saved = sessionStorage.getItem('__caption');
    if (saved) { cap.textContent = saved; cap.style.display = 'block'; }
  };
  if (document.body) add(); else document.addEventListener('DOMContentLoaded', add);
})();
"""

CARD = """<!doctype html><html><body style="margin:0;height:100vh;display:grid;place-items:center;
background:radial-gradient(circle at 30% 20%,#0c4a6e,#020617 70%);font-family:Segoe UI,system-ui,sans-serif;color:#e2e8f0">
<div style="text-align:center;max-width:900px;padding:0 40px">
<div style="display:inline-grid;place-items:center;width:64px;height:64px;border-radius:14px;background:#0284c7;
font-weight:800;font-size:30px;color:white;margin-bottom:22px">S</div>
<div style="font-size:44px;font-weight:700;letter-spacing:-.5px">{title}</div>
<div style="font-size:21px;color:#94a3b8;margin-top:16px;line-height:1.5">{subtitle}</div></div></body></html>"""


class Director:
    def __init__(self, page):
        self.page = page

    def caption(self, text, hold=0.0):
        self.page.evaluate(
            """t => { sessionStorage.setItem('__caption', t || '');
                      const c = document.getElementById('__caption'); if (!c) return;
                      c.textContent = t; c.style.display = t ? 'block' : 'none'; }""", text)
        if hold:
            time.sleep(hold)

    def goto(self, url, caption=None, wait=None):
        self.page.goto(url)
        if wait:
            self.page.wait_for_selector(wait, timeout=60_000)
        self.caption(caption or "")

    def move_to(self, locator, steps=25):
        locator.scroll_into_view_if_needed()
        box = locator.bounding_box()
        x, y = box["x"] + box["width"] / 2, box["y"] + box["height"] / 2
        self.page.mouse.move(x, y, steps=steps)
        return x, y

    def click(self, locator, pause=0.6):
        self.move_to(locator)
        time.sleep(0.25)
        locator.click()
        time.sleep(pause)

    def type(self, locator, text, delay=90):
        self.click(locator, pause=0.2)
        locator.press_sequentially(text, delay=delay)

    def scroll(self, dy, steps=10, pause=0.04):
        for _ in range(steps):
            self.page.mouse.wheel(0, dy / steps)
            time.sleep(pause)

    def card(self, title, subtitle, hold=3.5):
        self.page.goto("about:blank")  # cancels any meta-refresh timer of the previous page
        self.page.set_content(CARD.replace("{title}", title).replace("{subtitle}", subtitle))
        time.sleep(hold)


def run(cmd):
    print(f"$ {cmd}", flush=True)
    subprocess.run(cmd, shell=True, check=True)


def open_from_inbox(d, title_substring, channel="teams"):
    """Find the inbox card for an incident and click its Open Incident button."""
    page = d.page
    card = page.locator(".card", has=page.locator(".ch", has_text=channel)).filter(has_text=title_substring).first
    card.wait_for(timeout=60_000)
    btn = card.get_by_role("link", name="Open Incident")
    d.click(btn, pause=0.2)
    page.wait_for_selector("text=Captured exception rows", timeout=30_000)
    d.caption("")


def record(out):
    tmp = tempfile.mkdtemp()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        ctx = browser.new_context(viewport={"width": W, "height": H}, record_video_dir=tmp,
                                  record_video_size={"width": W, "height": H}, device_scale_factor=1)
        ctx.add_init_script(OVERLAY_JS)
        page = ctx.new_page()
        d = Director(page)

        # 1 — Title
        d.card("Sentinel", "Zero-Trust Operational Exception &amp; Observability Platform<br>"
               "No agents · No SDK · No code changes in your applications", 4.5)

        # 2 — Dashboard fills up by itself
        d.goto(BASE + "/", "Sentinel watches your databases and servers from the outside — incidents appear automatically.",
               wait="text=Last 24h")
        page.wait_for_selector("tbody tr", timeout=60_000)
        time.sleep(4)
        d.caption("Business exceptions, runtime errors, DB outages and endpoint outages — each counted on the cards above.", 4.5)
        d.move_to(page.locator("tbody tr").first)
        d.caption("Every problem is ONE incident. While it persists Sentinel re-checks it without sending repeat alerts.", 4.5)

        # 3 — Zero-code setup
        d.goto(BASE + "/config/sources", "Setup step 1 — add a database with a read-only login. The connection string is encrypted and never shown again.",
               wait="tbody tr")
        time.sleep(2)
        row = page.locator("tbody tr", has_text="ERP Production (demo)")
        d.click(row.get_by_role("button", name="Test"), pause=2.2)
        d.caption("Connection test: a few milliseconds.", 2.5)

        d.goto(BASE + "/config/rules", "Setup step 2 — a business rule is just a SELECT that returns the BAD records.", wait="tbody tr")
        time.sleep(2.5)
        d.click(page.get_by_role("button", name="Daily"), pause=1.2)
        d.click(page.get_by_role("button", name="Weekly"), pause=1.2)
        d.click(page.get_by_role("button", name="Minutes"), pause=0.8)
        d.caption("Schedule with simple selectors or any cron expression.", 2.5)
        d.scroll(520, steps=14)
        d.caption("Results per rule: 'exception: N rows' when bad data exists, green 'ok' when healthy.", 5)

        # 4 — Notifications
        d.goto(MOCK + "/", "Alerts arrive in Microsoft Teams, Zoom and WhatsApp (shown here in a demo inbox) and by email.",
               wait=".card")
        time.sleep(3.5)
        d.caption("Compliance: NO invoice numbers, amounts or customer names leave Sentinel — only a secure link.", 4.5)
        d.scroll(500, steps=12)
        time.sleep(1)
        d.scroll(-500, steps=12)

        # 5 — Deep link
        d.caption("Click 'Open Incident' in the Teams card…", 1.5)
        open_from_inbox(d, "Invoices posted without GL entry")
        d.caption("The deep link opens the exact rows the rule captured — after sign-in (zero trust).", 4.5)
        filt = page.get_by_placeholder("Filter rows…")
        d.type(filt, "90002")
        time.sleep(1.5)
        filt.fill("")
        d.caption("Search, sort and export the captured rows to CSV.", 0)
        d.click(page.locator("table").first.get_by_role("columnheader", name="amount"), pause=1.2)
        d.move_to(page.get_by_role("button", name="Export CSV"))
        time.sleep(1.5)
        d.caption("A Recommended Corrective Action Plan tells the engineer exactly what to do.", 0)
        plan = page.locator("ol li")
        d.click(plan.nth(0).get_by_role("checkbox"), pause=0.8)
        d.click(plan.nth(1).get_by_role("checkbox"), pause=1.2)
        d.click(page.get_by_role("button", name="Acknowledge"), pause=1.5)
        d.caption("Acknowledged — the team can see someone is on it.", 3)

        # 6 — Runtime exceptions without SDK
        d.goto(BASE + "/", "Now the application throws errors — it writes them to its OWN log table as usual…", wait="tbody tr")
        time.sleep(2)
        run(SIMULATE)
        page.wait_for_selector("tbody tr:has-text('OrderPortal & BillingSvc errors')", timeout=60_000)
        d.move_to(page.locator("tbody tr", has_text="OrderPortal & BillingSvc errors"))
        d.caption("…and Sentinel raises a Runtime Exception within seconds. No SDK was installed in the app.", 4)
        d.goto(MOCK + "/", None, wait=".card")
        open_from_inbox(d, "OrderPortal & BillingSvc errors")
        d.caption("The new error rows: NullReferenceException, SQL deadlock, gateway timeout — read straight from the log table.", 5.5)

        # 7 — Infrastructure outage
        d.goto(MOCK + "/", None, wait=".card")
        d.caption("Infrastructure too: an IIS site returning HTTP 503…", 1.5)
        open_from_inbox(d, "IIS web02")
        d.caption("Outage incident with status code and latency — the plan says: recycle the IIS App Pool first.", 5)
        d.scroll(300, steps=8)
        time.sleep(1.5)

        # 8 — Self-healing
        d.goto(BASE + "/", "Finally, the finance team corrects the bad data in the ERP…", wait="tbody tr")
        time.sleep(2)
        run(FIX)
        time.sleep(9)
        d.click(page.get_by_role("button", name="resolved", exact=True), pause=2)
        d.caption("…and Sentinel closes the business incidents by itself. No one has to remember to close tickets.", 5)
        d.goto(MOCK + "/", "RESOLVED messages are posted to the same channels automatically.", wait=".card")
        time.sleep(5)

        # 9 — End
        d.card("Sentinel", "Read-only · Encrypted secrets · Deep links instead of data in chat<br>"
               "2 ms deep-link response (p95) · Teams · Zoom · WhatsApp · Email", 5)

        video = page.video
        ctx.close()
        browser.close()
        shutil.move(video.path(), out)
    print(f"saved {out}")


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default="sentinel-demo.webm")
    record(ap.parse_args().out)
