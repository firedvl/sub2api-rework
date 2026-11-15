"""Check how the installed official client handles the gateway's usage-limit 429.

The loopback stub answers /v1/messages the way the gateway does when every
account is out of usage (see docs/CLAUDE_CODE_ACCOUNTS.md), then succeeds once
the reset has passed. The client authenticates with a synthetic gateway key.
"""

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
import time
import unittest


def run_client(reset_in):
    reset_at = None
    message = f"All accounts that can serve this model are rate-limited or out of usage. Usage resets at SYNTHETIC_RESET (in {reset_in}s)."
    attempts = []

    class Stub(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def reply(self, status, kind, data, headers=()):
            self.send_response(status)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(data)))
            for name, value in headers:
                self.send_header(name, value)
            self.end_headers()
            self.wfile.write(data)

        def do_POST(self):
            nonlocal reset_at
            body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
            if self.path.split("?")[0].endswith("/messages/count_tokens"):
                self.reply(200, "application/json", b'{"input_tokens":8}')
                return
            if reset_at is None:
                reset_at = int(time.time()) + reset_in
            attempts.append(time.time())
            if time.time() < reset_at:
                data = json.dumps({"type": "error", "error": {"type": "rate_limit_error", "message": message}}).encode()
                self.reply(429, "application/json", data, (
                    ("Retry-After", str(max(1, reset_at - int(time.time())))),
                    ("anthropic-ratelimit-unified-status", "rejected"),
                    ("anthropic-ratelimit-unified-reset", str(reset_at)),
                    ("anthropic-ratelimit-unified-representative-claim", "five_hour"),
                    ("anthropic-ratelimit-unified-5h-status", "rejected"),
                    ("anthropic-ratelimit-unified-5h-reset", str(reset_at)),
                ))
                return
            start = {"id": "msg_local", "type": "message", "role": "assistant", "model": body["model"],
                     "content": [], "stop_reason": None, "stop_sequence": None,
                     "usage": {"input_tokens": 8, "output_tokens": 0}}
            events = [
                {"type": "message_start", "message": start},
                {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}},
                {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "AFTER_RESET_OK"}},
                {"type": "content_block_stop", "index": 0},
                {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": None}, "usage": {"output_tokens": 3}},
                {"type": "message_stop"},
            ]
            self.reply(200, "text/event-stream",
                       "".join(f"event: {e['type']}\ndata: {json.dumps(e)}\n\n" for e in events).encode())

    with tempfile.TemporaryDirectory() as directory, ThreadingHTTPServer(("127.0.0.1", 0), Stub) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        # No real HOME, provider secrets, proxy or user settings enter the process.
        env = {"PATH": os.environ["PATH"], "HOME": directory,
               "CLAUDE_CONFIG_DIR": str(Path(directory) / "claude"),
               "ANTHROPIC_CONFIG_DIR": str(Path(directory) / "anthropic"),
               "ANTHROPIC_AUTH_TOKEN": "synthetic-loopback-only",
               "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{server.server_port}",
               "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"}
        try:
            result = subprocess.run([shutil.which("claude"), "--bare", "-p", "Reply AFTER_RESET_OK",
                                     "--model", "claude-sonnet-4-6", "--tools", "", "--max-turns", "1",
                                     "--no-session-persistence", "--output-format", "json"],
                                    cwd=directory, env=env, capture_output=True, text=True, timeout=60)
        finally:
            server.shutdown()
            thread.join(timeout=5)
    return result, attempts, message


@unittest.skipUnless(shutil.which("claude"), "official Claude Code is not installed")
class UsageLimitContract(unittest.TestCase):
    def test_short_reset_is_waited_out_and_the_request_continues(self):
        result, attempts, _ = run_client(reset_in=3)
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertIn("AFTER_RESET_OK", result.stdout)
        self.assertEqual(len(attempts), 2)
        self.assertGreaterEqual(attempts[1] - attempts[0], 2, "the client must wait for Retry-After")

    def test_long_reset_stops_at_once_and_shows_the_reset_message(self):
        result, attempts, message = run_client(reset_in=3600)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(attempts), 1, "a reset far away must not be retried in a loop")
        self.assertIn(message, result.stdout)


if __name__ == "__main__":
    unittest.main()
