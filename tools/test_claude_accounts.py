"""Offline checks: no official Claude process, credentials or inference are used."""

import contextlib
import io
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import claude_accounts


class ClaudeAccountsTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "accounts"
        self.stdout = io.StringIO()
        self.stderr = io.StringIO()
        self.process = patch.object(claude_accounts.subprocess, "run",
                                    return_value=subprocess.CompletedProcess([], 0)).start()
        self.addCleanup(patch.stopall)

    def invoke(self, command, profile="personal", extra=()):
        with contextlib.redirect_stdout(self.stdout), contextlib.redirect_stderr(self.stderr):
            return claude_accounts.main(["--root", str(self.root), command, profile, *extra])

    def test_login_and_independent_profiles(self):
        for command, name in (("init", "personal"), ("login", "work")):
            self.assertEqual(self.invoke(command, name), 0)
            args, kwargs = self.process.call_args
            self.assertEqual(args[0], ["claude", "--settings", '{"forceLoginMethod":"claudeai"}',
                                      "--setting-sources", "user", "auth", "login", "--claudeai"])
            self.assertEqual(kwargs["env"]["CLAUDE_CONFIG_DIR"], str(self.root / name))
            self.assertEqual(kwargs["env"]["ANTHROPIC_CONFIG_DIR"], str(self.root / name / "anthropic"))
            self.assertIn(f"Profile: {name}", self.stdout.getvalue())
            if os.name == "posix":
                self.assertEqual((self.root / name).stat().st_mode & 0o777, 0o700)
        self.assertNotEqual(self.process.call_args_list[0].kwargs["env"]["CLAUDE_CONFIG_DIR"],
                            self.process.call_args_list[1].kwargs["env"]["CLAUDE_CONFIG_DIR"])

    def test_run_and_status_remove_conflicting_environment(self):
        self.invoke("init")
        inherited = {"HOME": str(Path.home()), "PATH": "/bin", "KEEP_ME": "yes",
                     "ANTHROPIC_API_KEY": "fake", "ANTHROPIC_BASE_URL": "fake",
                     "CLAUDE_CODE_OAUTH_TOKEN": "fake", "CLAUDE_CONFIG_DIR": "fake",
                     "ANTHROPIC_CONFIG_DIR": "fake-global-profile",
                     "CLAUDE_CODE_USE_BEDROCK": "1", "CLAUDE_CODE_REMOTE": "1",
                     "AWS_PROFILE": "fake", "GOOGLE_APPLICATION_CREDENTIALS": "fake",
                     "AZURE_API_KEY": "fake", "OPENAI_API_KEY": "fake"}
        with patch.dict(os.environ, inherited, clear=True):
            for command, tail in (("run", []), ("status", ["auth", "status", "--text"])):
                self.assertEqual(self.invoke(command), 0)
                args, kwargs = self.process.call_args
                self.assertEqual(args[0][5:], tail)
                self.assertEqual(kwargs["env"], {"HOME": inherited["HOME"], "PATH": "/bin",
                                                "KEEP_ME": "yes", "CLAUDE_CONFIG_DIR": str(self.root / "personal"),
                                                "ANTHROPIC_CONFIG_DIR": str(self.root / "personal" / "anthropic")})
                self.assertNotIn("cwd", kwargs)

    def test_missing_profiles_do_not_launch_or_create(self):
        for command in ("status", "run"):
            self.assertEqual(self.invoke(command), 1)
        self.assertFalse(self.root.exists())
        self.process.assert_not_called()

    def test_list_is_read_only_and_does_not_authenticate(self):
        with contextlib.redirect_stdout(self.stdout):
            self.assertEqual(claude_accounts.main(['--root', str(self.root), 'list']), 0)
        self.assertFalse(self.root.exists())
        for name in ('work', 'personal'):
            claude_accounts.profile_environment(self.root, name, True)
        self.assertEqual(claude_accounts.list_profiles(self.root), ['personal', 'work'])
        self.process.assert_not_called()

    def test_unsafe_names_and_passthrough_are_rejected(self):
        for name in ("../other", "a/b", ".", "..", "a.b", "a b", "a\\b", "-a", "a" * 65):
            with self.subTest(name=name), self.assertRaises(SystemExit):
                self.invoke("init", name)
        with self.assertRaises(SystemExit):
            self.invoke("run", extra=("--settings", "{}"))
        self.process.assert_not_called()

    @unittest.skipUnless(os.name == "posix", "POSIX private permissions")
    def test_insecure_root_or_profile_is_refused(self):
        self.root.mkdir(mode=0o755)
        self.assertEqual(self.invoke("init"), 1)
        self.root.chmod(0o700)
        (self.root / "personal").mkdir(mode=0o755)
        self.assertEqual(self.invoke("run"), 1)
        self.process.assert_not_called()

    def test_symlink_root_or_profile_is_refused(self):
        target = Path(self.temp.name) / "target"
        target.mkdir(mode=0o700)
        try:
            self.root.symlink_to(target, target_is_directory=True)
        except OSError:
            self.skipTest("Symlink creation unavailable")
        self.assertEqual(self.invoke("init"), 1)
        self.root.unlink()
        self.root.mkdir(mode=0o700)
        (self.root / "personal").symlink_to(target, target_is_directory=True)
        self.assertEqual(self.invoke("init"), 1)
        self.process.assert_not_called()

    def test_shared_claude_root_is_refused(self):
        self.root = Path.home() / ".claude"
        self.assertEqual(self.invoke("init"), 1)
        self.process.assert_not_called()

    def test_symlink_anthropic_profile_root_is_refused(self):
        self.invoke("init")
        self.process.reset_mock()
        config = self.root / "personal" / "anthropic"
        config.rmdir()
        try:
            config.symlink_to(Path(self.temp.name), target_is_directory=True)
        except OSError:
            self.skipTest("Symlink creation unavailable")
        self.assertEqual(self.invoke("run"), 1)
        self.process.assert_not_called()

    def test_warmup_is_disabled_without_files_or_processes(self):
        self.assertEqual(self.invoke("warmup"), 2)
        self.assertIn("no supported request-scoped guarantee", self.stderr.getvalue())
        self.assertFalse(self.root.exists())
        self.process.assert_not_called()

    def test_child_exit_status_and_missing_cli_are_not_hidden(self):
        self.process.return_value.returncode = 7
        self.assertEqual(self.invoke("init"), 7)
        self.process.side_effect = FileNotFoundError("claude not installed")
        self.assertEqual(self.invoke("run"), 1)
        self.assertIn("claude not installed", self.stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
