"""Exercise the installed official client against a loopback-only synthetic API."""

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
import unittest


class NativeClaudeContract(unittest.TestCase):
    @unittest.skipUnless(shutil.which("claude"), "official Claude Code is not installed")
    def test_bare_client_uses_only_selected_api_endpoint(self):
        requests = []

        class Stub(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
                requests.append((self.path, dict(self.headers), body))
                if self.path.split("?")[0].endswith("/messages/count_tokens"):
                    data = b'{"input_tokens":8}'
                    kind = "application/json"
                elif self.path.split("?")[0] == "/anthropic-api/v1/messages":
                    message = {"id": "msg_local", "type": "message", "role": "assistant",
                               "model": body["model"], "content": [], "stop_reason": None,
                               "stop_sequence": None, "usage": {"input_tokens":8, "output_tokens":0}}
                    events = [
                        {"type":"message_start", "message":message},
                        {"type":"content_block_start", "index":0, "content_block":{"type":"text", "text":""}},
                        {"type":"content_block_delta", "index":0, "delta":{"type":"text_delta", "text":"LOCAL_CONTRACT_OK"}},
                        {"type":"content_block_stop", "index":0},
                        {"type":"message_delta", "delta":{"stop_reason":"end_turn", "stop_sequence":None}, "usage":{"output_tokens":3}},
                        {"type":"message_stop"},
                    ]
                    data = "".join(f"event: {e['type']}\ndata: {json.dumps(e)}\n\n" for e in events).encode()
                    kind = "text/event-stream"
                else:
                    self.send_error(404)
                    return
                self.send_response(200)
                self.send_header("Content-Type", kind)
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        with tempfile.TemporaryDirectory() as directory, ThreadingHTTPServer(("127.0.0.1", 0), Stub) as server:
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            # No real HOME, provider secrets, proxy or user settings enter the process.
            env = {"PATH": os.environ["PATH"], "HOME": directory,
                   "CLAUDE_CONFIG_DIR": str(Path(directory) / "claude"),
                   "ANTHROPIC_CONFIG_DIR": str(Path(directory) / "anthropic"),
                   "ANTHROPIC_API_KEY": "synthetic-loopback-only",
                   "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{server.server_port}/anthropic-api",
                   "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"}
            try:
                result = subprocess.run([shutil.which("claude"), "--bare", "-p", "Reply LOCAL_CONTRACT_OK",
                                         "--model", "claude-sonnet-4-6", "--tools", "", "--max-turns", "1",
                                         "--no-session-persistence", "--output-format", "json"],
                                        cwd=directory, env=env, capture_output=True, text=True, timeout=45)
            finally:
                server.shutdown()
                thread.join(timeout=5)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertIn("LOCAL_CONTRACT_OK", result.stdout)
            self.assertTrue(requests)
            for path, headers, body in requests:
                self.assertTrue(path.startswith("/anthropic-api/v1/messages"), path)
                self.assertEqual(headers.get("x-api-key"), "synthetic-loopback-only")
                self.assertNotIn("Authorization", headers)
                self.assertEqual(body["model"], "claude-sonnet-4-6")


if __name__ == "__main__":
    unittest.main()
