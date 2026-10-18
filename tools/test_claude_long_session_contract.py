"""Installed Claude 2.1.300 API protocol checks, using only disposable loopback data.

These exercise repeated Read tools and process cancellation, not subscription
authentication, native warm-up behavior, hours of latency, or real billing.
"""

from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import tempfile
import threading
import unittest


CLI = shutil.which("claude")
MODEL = "claude-sonnet-4-6"
API_PREFIX = "/synthetic-anthropic/v1/messages"


def sse(event):
    return f"event: {event['type']}\ndata: {json.dumps(event)}\n\n".encode()


def response_events(model, content, stop):
    yield {"type": "message_start", "message": {
        "id": "msg_synthetic", "type": "message", "role": "assistant",
        "model": model, "content": [], "stop_reason": None, "stop_sequence": None,
        "usage": {"input_tokens": 8, "output_tokens": 0,
                  "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}}
    if content["type"] == "tool_use":
        start = {**content, "input": {}}
        delta = {"type": "input_json_delta", "partial_json": json.dumps(content["input"])}
    else:
        start = {"type": "text", "text": ""}
        delta = {"type": "text_delta", "text": content["text"]}
    yield {"type": "content_block_start", "index": 0, "content_block": start}
    yield {"type": "content_block_delta", "index": 0, "delta": delta}
    yield {"type": "content_block_stop", "index": 0}
    yield {"type": "message_delta", "delta": {
        "stop_reason": stop, "stop_sequence": None}, "usage": {"output_tokens": 3}}
    yield {"type": "message_stop"}


def semantic_history(messages):
    def without_cache(value):
        if isinstance(value, dict):
            return {key: without_cache(item) for key, item in value.items() if key != "cache_control"}
        if isinstance(value, list):
            return [without_cache(item) for item in value]
        return value

    result = {}
    for message in without_cache(messages):
        content = message["content"]
        if isinstance(content, str):
            content = [{"type": "text", "text": content}]
        result.setdefault(message["role"], []).extend(content)
    return result


@unittest.skipUnless(CLI and os.name == "posix", "requires installed Claude and POSIX process cleanup")
class ClaudeLongSessionContract(unittest.TestCase):
    @contextmanager
    def client(self, mode):
        requests = []
        streaming = threading.Event()
        disconnected = threading.Event()
        stopping = threading.Event()
        errors = []
        with tempfile.TemporaryDirectory(prefix="claude-contract-") as directory:
            root = Path(directory)
            fixtures = [root / f"fixture-{turn}.txt" for turn in range(1, 7)]
            for fixture in fixtures:
                fixture.write_text("\n".join(
                    f"SAFE_FIXTURE_{i:03d}: " + "disposable-contract-data " * 4 for i in range(128)))
                fixture.chmod(0o600)

            class Stub(BaseHTTPRequestHandler):
                def log_message(self, *args):
                    pass

                def do_POST(self):
                    try:
                        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
                        requests.append((self.path, dict(self.headers), body))
                        path = self.path.split("?")[0]
                        if path == API_PREFIX + "/count_tokens":
                            data = b'{"input_tokens":8}'
                            self.send_response(200)
                            self.send_header("Content-Type", "application/json")
                            self.send_header("Content-Length", str(len(data)))
                            self.end_headers()
                            self.wfile.write(data)
                            return
                        if path != API_PREFIX:
                            self.send_error(404)
                            return
                        if mode == "cancel":
                            self.send_response(200)
                            self.send_header("Content-Type", "text/event-stream")
                            self.end_headers()
                            start = next(response_events(body["model"], {}, "end_turn"))
                            self.wfile.write(sse(start))
                            self.wfile.flush()
                            streaming.set()
                            self.connection.settimeout(0.1)
                            while not stopping.is_set():
                                try:
                                    if not self.connection.recv(1, socket.MSG_PEEK):
                                        disconnected.set()
                                        break
                                except socket.timeout:
                                    continue
                                except ConnectionResetError:
                                    disconnected.set()
                                    break
                            return
                        turn = sum(path.split("?")[0] == API_PREFIX for path, _, _ in requests)
                        content = ({"type": "tool_use", "id": f"read_{turn}", "name": "Read",
                                    "input": {"file_path": str(fixtures[turn - 1])}} if turn <= 6 else
                                   {"type": "text", "text": "SYNTHETIC_SIX_READS_COMPLETE"})
                        data = b"".join(sse(e) for e in response_events(
                            body["model"], content, "tool_use" if turn <= 6 else "end_turn"))
                        self.send_response(200)
                        self.send_header("Content-Type", "text/event-stream")
                        self.send_header("Content-Length", str(len(data)))
                        self.end_headers()
                        self.wfile.write(data)
                    except (BrokenPipeError, ConnectionResetError):
                        if mode == "cancel":
                            disconnected.set()
                        else:
                            errors.append("unexpected client disconnect")
                    except Exception as exc:
                        errors.append(repr(exc))

            with ThreadingHTTPServer(("127.0.0.1", 0), Stub) as server:
                server.daemon_threads = True
                thread = threading.Thread(target=server.serve_forever, daemon=True)
                thread.start()
                env = {"PATH": os.environ["PATH"], "HOME": directory,
                       "CLAUDE_CONFIG_DIR": str(root / "claude"),
                       "ANTHROPIC_CONFIG_DIR": str(root / "anthropic"),
                       "ANTHROPIC_API_KEY": "synthetic-loopback-only",
                       "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{server.server_port}/synthetic-anthropic",
                       "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"}
                version = subprocess.run([CLI, "--version"], env=env, cwd=root,
                                         capture_output=True, text=True, timeout=10)
                self.assertEqual(version.returncode, 0, version.stderr)
                self.assertEqual(version.stdout.strip(), "2.1.300 (Claude Code)",
                                 "This installed-client contract is qualified for exactly 2.1.300")
                command = [CLI, "--bare", "--restricted", "-p", "Read the disposable fixture as requested.",
                           "--model", MODEL, "--tools", "Read", "--allowedTools", "Read",
                           "--permission-mode", "dontAsk", "--strict-mcp-config",
                           "--setting-sources", "", "--disable-slash-commands",
                           "--system-prompt", "Offline synthetic contract. Only read the supplied fixture.",
                           "--max-turns", "8", "--no-session-persistence", "--output-format", "json"]
                process = subprocess.Popen(command, cwd=root, env=env, stdout=subprocess.PIPE,
                                           stderr=subprocess.PIPE, text=True, start_new_session=True)
                try:
                    yield process, requests, streaming, disconnected, fixtures
                finally:
                    # The owned process group also covers accidental descendants on timeout.
                    try:
                        os.killpg(process.pid, signal.SIGTERM)
                    except ProcessLookupError:
                        pass
                    try:
                        process.communicate(timeout=5)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGKILL)
                        process.communicate(timeout=5)
                    stopping.set()
                    server.shutdown()
                    thread.join(timeout=5)
                    self.assertFalse(thread.is_alive())
                    self.assertFalse(errors, errors)

    def assert_requests_are_synthetic(self, requests):
        self.assertTrue(requests)
        for path, headers, body in requests:
            self.assertIn(path.split("?")[0], (API_PREFIX, API_PREFIX + "/count_tokens"))
            self.assertEqual(headers.get("x-api-key"), "synthetic-loopback-only")
            self.assertNotIn("authorization", {key.lower() for key in headers})
            self.assertEqual(body["model"], MODEL)

    def test_six_real_read_roundtrips_preserve_large_history(self):
        with self.client("tools") as (process, requests, _, _, fixtures):
            stdout, stderr = process.communicate(timeout=60)
            self.assertEqual(process.returncode, 0, stderr + stdout)
            self.assertIn("SYNTHETIC_SIX_READS_COMPLETE", stdout)
            self.assert_requests_are_synthetic(requests)
            turns = [body for path, _, body in requests if path.split("?")[0] == API_PREFIX]
            self.assertEqual(len(turns), 7)
            for index, body in enumerate(turns):
                self.assertEqual([tool["name"] for tool in body["tools"]], ["Read"])
                blocks = [block for message in body["messages"] for block in message["content"]
                          if isinstance(block, dict)]
                uses = [block for block in blocks if block.get("type") == "tool_use"]
                results = [block for block in blocks if block.get("type") == "tool_result"]
                self.assertEqual([block["id"] for block in uses],
                                 [f"read_{n}" for n in range(1, index + 1)])
                self.assertEqual([block["tool_use_id"] for block in results],
                                 [f"read_{n}" for n in range(1, index + 1)])
                for use, result, fixture in zip(uses, results, fixtures):
                    self.assertEqual(use["input"]["file_path"], str(fixture))
                    self.assertFalse(result.get("is_error", False), result)
                    self.assertIn("SAFE_FIXTURE_000", json.dumps(result))
                    self.assertIn("SAFE_FIXTURE_127", json.dumps(result))
                if index:
                    # The client coalesces same-role messages and moves cache markers.
                    previous = semantic_history(turns[index - 1]["messages"])
                    current = semantic_history(body["messages"])
                    for role, blocks in previous.items():
                        self.assertEqual(current[role][:len(blocks)], blocks)
            self.assertGreater(len(json.dumps(turns[-1]["messages"])), 65000)
            self.assertEqual(turns[-1]["thinking"], {"type": "adaptive", "display": "omitted"})
            self.assertEqual(turns[-1]["context_management"], {
                "edits": [{"type": "clear_thinking_20251015", "keep": "all"}]})
            self.assertIn('"cache_control": {"type": "ephemeral"}', json.dumps(turns[-1]))
            # Report fields actually emitted by this client; do not fabricate them.
            print("Installed CLI protocol observation: " + json.dumps({
                "version": "2.1.300", "message_requests": len(turns),
                "history_bytes": len(json.dumps(turns[-1]["messages"])),
                "thinking": turns[-1].get("thinking"),
                "context_management": turns[-1].get("context_management"),
                "cache_control_blocks": json.dumps(turns[-1]).count('"cache_control"')}))

    def test_terminating_actual_client_closes_idle_stream_without_replay(self):
        with self.client("cancel") as (process, requests, streaming, disconnected, _):
            self.assertTrue(streaming.wait(timeout=30), "client did not open the synthetic stream")
            process.terminate()
            process.communicate(timeout=10)
            self.assertIsNotNone(process.returncode)
            with self.assertRaises(ProcessLookupError):
                os.killpg(process.pid, 0)
            self.assertTrue(disconnected.wait(timeout=5), "client left its upstream stream open")
            self.assert_requests_are_synthetic(requests)
            self.assertEqual(sum(path.split("?")[0] == API_PREFIX for path, _, _ in requests), 1)


if __name__ == "__main__":
    unittest.main()
