"""Synthetic-only warm-up tests: no login, provider request, or installed schedule."""
import contextlib
import copy
from datetime import datetime, timezone
import io
import json
import multiprocessing
import os
from pathlib import Path
import subprocess
import signal
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

import claude_warmup as w


NOW = datetime(2026, 10, 18, 8, 1, tzinfo=timezone.utc).timestamp()
AUTH = {'loggedIn': True, 'authMethod': 'claude.ai', 'apiProvider': 'firstParty',
        'subscriptionType': 'pro', 'email': 'example@example.invalid'}


def racing_tick(path, queue, profile='personal'):
    with patch.object(w, 'auth', return_value=w.identity(AUTH)), patch.object(
            w, 'dispatch', side_effect=lambda *a: (time.sleep(.2) or 'request_completed_reset_unverified')):
        queue.put(w.tick(Path(path), profile, True, NOW))


class WarmupTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.parent = Path(self.temporary.name)
        self.state_root = self.parent / 'state'
        patch.object(w, 'STATE_ROOT', self.state_root).start()
        self.addCleanup(patch.stopall)
        self.path = self.parent / 'config.json'
        self.root = self.parent / 'accounts'
        w.profile_environment(self.root, 'personal', True)
        self.p = {'enabled': True, 'mode': 'observed_reset', 'timezone': 'UTC', 'slots': ['08:00'],
                  'allow_unverified_schedule': False, 'native_default': False,
                  'billing_identity_digest': w.identity(AUTH), 'billing_mode': 'credits_disabled_attested', 'billing_acknowledged_at': w.iso(NOW),
                  'identity_digest': w.identity(AUTH), 'observation': {
                      'source': 'manual_official_ui', 'identity_digest': w.identity(AUTH), 'observed_at': w.iso(NOW),
                      'five_hour_reset': w.iso(NOW - 1), 'weekly_reset': w.iso(NOW + 86400),
                      'five_hour_utilization': 0, 'weekly_utilization': 1}}
        self.config = {'enabled': True, 'claude_binary': '/synthetic/claude', 'profile_root': str(self.root), 'profiles': {'personal': self.p}}
        self.save()

    def save(self):
        w.write_config(self.path, self.config)

    def invoke(self, *args):
        output = io.StringIO()
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            code = w.main(['--config', str(self.path), *args])
        return code, json.loads(output.getvalue())

    def run_tick(self, now=NOW):
        with patch.object(w, 'auth', return_value=w.identity(AUTH)), patch.object(
                w, 'dispatch', return_value='request_completed_reset_unverified') as dispatch:
            result = w.tick(self.path, 'personal', True, now)
        return result, dispatch

    def test_init_disabled_and_dry_run_do_not_infer(self):
        self.path.unlink()
        with patch.object(w.subprocess, 'run') as run:
            code, value = self.invoke('init', 'personal', '--root', str(self.root), '--timezone', 'UTC')
            self.assertEqual(code, 0)
            self.assertFalse(value['enabled'])
            self.assertEqual(w.tick(self.path, 'personal', now=NOW)['skipped'], 'disabled')
            run.assert_not_called()
        self.assertFalse((self.state_root / 'state.sqlite3').exists())
        self.save()
        self.assertTrue(w.tick(self.path, 'personal', now=NOW)['dry_run'])
        self.assertFalse((self.state_root / 'state.sqlite3').exists())

    def test_authorization_gates(self):
        for key, value, expected in [('enabled', False, 'disabled'), ('billing_mode', 'unverified', 'billing_unverified'),
                                    ('billing_acknowledged_at', w.iso(NOW - 86401), 'billing_acknowledgment_expired'),
                                    ('billing_acknowledged_at', w.iso(NOW + 1), 'billing_acknowledgment_expired'),
                                    ('identity_digest', None, 'billing_identity_unverified')]:
            with self.subTest(key=key, value=value):
                p = copy.deepcopy(self.p)
                p[key] = value
                self.assertEqual(w.gate(self.config, p, NOW), expected)

    def test_observation_gates(self):
        for key, value, reason in [('weekly_utilization', 100, 'weekly_exhausted'),
                                  ('five_hour_reset', w.iso(NOW + 1), 'five_hour_window_active'),
                                  ('weekly_reset', w.iso(NOW), 'quota_unknown'),
                                  ('observed_at', w.iso(NOW - 181), 'quota_unknown'),
                                  ('observed_at', w.iso(NOW + 1), 'quota_unknown'),
                                  ('weekly_utilization', float('nan'), 'quota_unknown'),
                                  ('five_hour_utilization', float('inf'), 'quota_unknown'),
                                  ('five_hour_utilization', None, 'quota_unknown'),
                                  ('five_hour_utilization', True, 'quota_unknown'),
                                  ('weekly_utilization', -1, 'quota_unknown')]:
            with self.subTest(key=key, value=value):
                p = copy.deepcopy(self.p)
                p['observation'][key] = value
                self.assertEqual(w.gate(self.config, p, NOW), reason)
        p = copy.deepcopy(self.p)
        p['observation'] = None
        self.assertEqual(w.gate(self.config, p, NOW), 'quota_unknown')
        p.update(mode='scheduled_unverified', allow_unverified_schedule=True)
        self.assertIsNone(w.gate(self.config, p, NOW))
        p['allow_unverified_schedule'] = False
        self.assertEqual(w.gate(self.config, p, NOW), 'unverified_schedule_not_authorized')
        self.assertIsNone(w.gate(self.config, self.p, NOW))

    def test_identity_strict_and_normalized(self):
        self.assertEqual(w.identity(AUTH), w.identity(dict(AUTH, email='  EXAMPLE@EXAMPLE.INVALID ')))
        for key, value in [('loggedIn', 1), ('authMethod', 'api_key'), ('apiProvider', 'bedrock'),
                           ('subscriptionType', None), ('email', ''), ('apiKeySource', 'ANTHROPIC_API_KEY')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                w.identity(dict(AUTH, **{key: value}))

    def test_completed_still_unverified_and_duplicate_guard(self):
        result, process = self.run_tick()
        self.assertEqual(result['outcome'], 'request_completed_reset_unverified')
        process.assert_called_once()
        self.assertEqual(self.run_tick()[0]['skipped'], 'five_hour_attempt_guard')
        status = w.status(self.path, 'personal', NOW)
        self.assertEqual(status['reset_attribution'], 'unverified')
        self.assertEqual(status['last_completed'], w.iso(NOW))
        self.assertNotIn('email', (self.state_root / 'state.sqlite3').read_bytes().decode('utf8', errors='ignore'))

    def test_failed_dispatch_and_crash_suppress_resend(self):
        with patch.object(w, 'auth', return_value=w.identity(AUTH)), patch.object(w, 'dispatch', side_effect=OSError):
            self.assertIn('unknown', w.tick(self.path, 'personal', True, NOW)['outcome'])
        self.assertEqual(self.run_tick()[0]['skipped'], 'five_hour_attempt_guard')
        db = w.state(self.path)
        db.execute("UPDATE profiles SET outcome='in_flight'")
        db.close()
        self.assertEqual(w.status(self.path, 'personal', NOW)['outcome'], 'outcome_unknown')
        self.assertEqual(self.run_tick()[0]['skipped'], 'five_hour_attempt_guard')

    def test_preflight_max_two_backoff_and_identity_mismatch(self):
        self.p['mode'] = 'scheduled_unverified'
        self.p['allow_unverified_schedule'] = True
        self.save()
        with patch.object(w, 'auth', return_value='wrong'), patch.object(w, 'dispatch') as process:
            self.assertEqual(w.tick(self.path, 'personal', True, NOW)['skipped'], 'auth_preflight_failed')
            self.assertEqual(w.tick(self.path, 'personal', True, NOW + 1)['skipped'], 'duplicate_or_preflight_backoff')
            self.assertEqual(w.tick(self.path, 'personal', True, NOW + 61)['skipped'], 'auth_preflight_failed')
            self.assertEqual(w.tick(self.path, 'personal', True, NOW + 122)['skipped'], 'duplicate_or_preflight_backoff')
            process.assert_not_called()
        self.assertIsNone(w.status(self.path, 'personal', NOW)['last_attempt'])

    def test_disable_or_cancel_during_preflight(self):
        def cancel(*args):
            db = w.state(self.path)
            db.execute('UPDATE profiles SET cancel=cancel+1')
            db.close()
            return w.identity(AUTH)
        with patch.object(w, 'auth', side_effect=cancel), patch.object(w, 'dispatch') as process:
            self.assertEqual(w.tick(self.path, 'personal', True, NOW)['skipped'], 'authorization_changed')
            process.assert_not_called()
        code, result = self.invoke('disable', 'personal')
        self.assertEqual(code, 0)
        self.assertTrue(result['disabled'])
        self.assertFalse(w.load(self.path, 'personal')[1]['enabled'])

    @unittest.skipUnless(os.name == 'posix', 'fork race test')
    def test_crossprocess_single_dispatch(self):
        context = multiprocessing.get_context('fork')
        queue = context.Queue()
        processes = [context.Process(target=racing_tick, args=(str(self.path), queue)) for _ in range(2)]
        for process in processes:
            process.start()
        for process in processes:
            process.join(5)
            self.assertEqual(process.exitcode, 0)
        results = [queue.get(timeout=1), queue.get(timeout=1)]
        self.assertEqual(sum('outcome' in result for result in results), 1)

    @unittest.skipUnless(os.name == 'posix', 'fork race test')
    def test_same_identity_aliases_share_atomic_guard(self):
        self.config['profiles']['alias'] = copy.deepcopy(self.p)
        w.profile_environment(self.root, 'alias', True)
        self.save()
        context = multiprocessing.get_context('fork')
        queue = context.Queue()
        processes = [context.Process(target=racing_tick, args=(str(self.path), queue, name))
                     for name in ('personal', 'alias')]
        for process in processes:
            process.start()
        for process in processes:
            process.join(5)
            self.assertEqual(process.exitcode, 0)
        results = [queue.get(timeout=1), queue.get(timeout=1)]
        self.assertEqual(sum('outcome' in result for result in results), 1)
        with patch.object(w, 'auth'), patch.object(w, 'dispatch') as dispatch:
            for name in ('personal', 'alias'):
                self.assertEqual(w.tick(self.path, name, True, NOW)['skipped'], 'five_hour_attempt_guard')
            dispatch.assert_not_called()

    def test_config_mutation_during_auth_aborts(self):
        for mutation in ('slots', 'binary', 'mode'):
            def change(*args):
                changed = copy.deepcopy(self.config)
                if mutation == 'slots':
                    changed['profiles']['personal']['slots'] = ['22:00']
                elif mutation == 'binary':
                    changed['claude_binary'] = '/other/claude'
                else:
                    changed['profiles']['personal']['mode'] = 'scheduled_unverified'
                    changed['profiles']['personal']['allow_unverified_schedule'] = True
                w.write_config(self.path, changed)
                return w.identity(AUTH)
            with self.subTest(mutation=mutation), patch.object(w, 'auth', side_effect=change), patch.object(w, 'dispatch') as dispatch:
                self.assertEqual(w.tick(self.path, 'personal', True, NOW)['skipped'], 'authorization_changed')
                dispatch.assert_not_called()
            # New occurrence/profile state for each case, same canonical identity ledger.
            db = w.state(self.path)
            db.execute('DELETE FROM slots')
            db.close()
            self.save()

    @unittest.skipUnless(os.name == 'posix', 'fork race test')
    def test_same_identity_across_configs(self):
        second = self.parent / 'second.json'
        w.write_config(second, self.config)
        context = multiprocessing.get_context('fork')
        queue = context.Queue()
        processes = [context.Process(target=racing_tick, args=(str(path), queue))
                     for path in (self.path, second)]
        for process in processes:
            process.start()
        for process in processes:
            process.join(5)
            self.assertEqual(process.exitcode, 0)
        results = [queue.get(timeout=1), queue.get(timeout=1)]
        self.assertEqual(sum('outcome' in result for result in results), 1)

    def test_rebind_clears_billing_and_observation(self):
        other = w.identity(dict(AUTH, email='other@example.invalid'))
        with patch.object(w, 'auth', return_value=other):
            code, _ = self.invoke('bind', 'personal')
        self.assertEqual(code, 0)
        config, p = w.load(self.path, 'personal')
        self.assertFalse(p['enabled'])
        self.assertEqual(p['identity_digest'], other)
        self.assertIsNone(p['billing_acknowledged_at'])
        self.assertIsNone(p['observation'])
        self.assertEqual(p['billing_mode'], 'unverified')
        p['enabled'] = True
        self.assertEqual(w.gate(config, p, NOW), 'billing_identity_unverified')
        self.p['identity_digest'] = other
        self.assertEqual(w.gate(self.config, self.p, NOW), 'billing_identity_unverified')

    def test_explicit_billing_acknowledgment_and_observation_binding(self):
        self.p['billing_mode'] = 'unverified'
        self.save()
        with patch.object(w.time, 'time', return_value=NOW):
            code, result = self.invoke('acknowledge-billing', 'personal', '--credits-disabled',
                                      '--acknowledge-no-overflow-guarantee')
        self.assertEqual(code, 0)
        self.assertTrue(result['billing_attestation_recorded'])
        _, p = w.load(self.path, 'personal')
        self.assertEqual(p['billing_identity_digest'], p['identity_digest'])
        p['observation']['identity_digest'] = '0' * 64
        self.assertEqual(w.gate(self.config, p, NOW), 'quota_unknown')

    @unittest.skipUnless(os.name == 'posix', 'POSIX child group cleanup')
    def test_real_synthetic_child_terminated_on_sigterm(self):
        pidfile = self.parent / 'child.pid'
        executable = self.parent / 'synthetic-cli'
        executable.write_text('#!' + sys.executable + '\nimport os,time\n'
                              + 'open(' + repr(str(pidfile)) + ',"w").write(str(os.getpid()))\n'
                              + 'time.sleep(30)\n')
        executable.chmod(0o700)
        code = ('import sys; sys.path.insert(0, ' + repr(str(Path(w.__file__).parent)) + '); '
                'import claude_warmup as w; w.dispatch({}, ' + repr(str(self.parent))
                + ', lambda: False, ' + repr(str(executable)) + ')')
        process = subprocess.Popen([sys.executable, '-c', code], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            deadline = time.monotonic() + 5
            while not pidfile.exists() and time.monotonic() < deadline:
                time.sleep(.02)
            self.assertTrue(pidfile.exists())
            pid = int(pidfile.read_text())
            process.send_signal(signal.SIGTERM)
            process.wait(timeout=5)
            with self.assertRaises(ProcessLookupError):
                os.kill(pid, 0)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=5)

    def test_dst_slots_and_midnight_grace(self):
        p = dict(self.p, timezone='America/Los_Angeles', slots=['02:30'])
        spring = w.stamp('2026-03-08T03:00:00-07:00')
        self.assertIsNone(w.occurrence(p, spring))
        p['slots'] = ['01:30']
        first = w.occurrence(p, w.stamp('2026-11-01T01:31:00-07:00'))
        self.assertIsNotNone(first)
        self.assertIsNone(w.occurrence(p, w.stamp('2026-11-01T01:31:00-08:00')))
        p.update(timezone='UTC', slots=['23:59'])
        self.assertIsNotNone(w.occurrence(p, w.stamp('2026-10-19T00:01:00Z')))
        self.assertIsNone(w.occurrence(p, w.stamp('2026-10-19T00:10:00Z')))

    def test_auth_contract_no_inference_and_native_default(self):
        env = w.profile_environment(self.root, 'personal', native_default=True)
        self.assertNotIn('CLAUDE_CONFIG_DIR', env)
        with patch.object(w.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, json.dumps(AUTH))) as run:
            self.assertEqual(w.auth(env, self.parent), w.identity(AUTH))
            command = run.call_args.args[0]
            self.assertEqual(command[-3:], ['auth', 'status', '--json'])
            self.assertEqual(command[command.index('--setting-sources') + 1], '')
            self.assertNotIn('-p', command)

    def test_dispatch_contract_and_result_validation(self):
        child = unittest.mock.Mock()
        child.returncode = 0
        child.poll.return_value = 0
        def spawn(command, **kwargs):
            self.assertNotIn('--bare', command)
            self.assertIn('--safe-mode', command)
            self.assertEqual(kwargs['env']['CLAUDE_CODE_MAX_RETRIES'], '0')
            self.assertEqual(kwargs['env']['CLAUDE_CODE_MAX_OUTPUT_TOKENS'], '16')
            kwargs['stdout'].write(json.dumps({'type': 'result', 'subtype': 'success', 'is_error': False}).encode())
            return child
        with patch.object(w.subprocess, 'Popen', side_effect=spawn):
            self.assertEqual(w.dispatch({}, self.parent, lambda: False), 'request_completed_reset_unverified')
        child.returncode = 1
        with patch.object(w.subprocess, 'Popen', side_effect=spawn):
            self.assertEqual(w.dispatch({}, self.parent, lambda: False), 'execution_failed_outcome_unknown')

    def test_dispatch_cancellation_terminates_owned_group(self):
        child = unittest.mock.Mock(pid=987654)
        child.poll.return_value = None
        with patch.object(w.subprocess, 'Popen', return_value=child), patch.object(w.os, 'killpg') as kill:
            self.assertEqual(w.dispatch({}, self.parent, lambda: True), 'cancelled_outcome_unknown')
            kill.assert_called_once_with(child.pid, w.signal.SIGTERM)

    def test_dispatch_timeout_and_malformed_success(self):
        child = unittest.mock.Mock(pid=987654)
        child.poll.return_value = None
        with patch.object(w.subprocess, 'Popen', return_value=child), patch.object(w.os, 'killpg'), patch.object(
                w.time, 'monotonic', side_effect=[0, 61]):
            self.assertEqual(w.dispatch({}, self.parent, lambda: False), 'timeout_outcome_unknown')
        child.poll.return_value = 0
        child.returncode = 0
        for value in ({'type': 'result', 'subtype': 'success', 'is_error': True}, {}, []):
            def spawn(command, **kwargs):
                kwargs['stdout'].write(json.dumps(value).encode())
                return child
            with patch.object(w.subprocess, 'Popen', side_effect=spawn):
                self.assertEqual(w.dispatch({}, self.parent, lambda: False), 'invalid_result_outcome_unknown')

    def test_auth_exception_does_not_claim_dispatch(self):
        with patch.object(w, 'auth', side_effect=subprocess.TimeoutExpired('status', 15)), patch.object(w, 'dispatch') as dispatch:
            self.assertEqual(w.tick(self.path, 'personal', True, NOW)['skipped'], 'auth_preflight_failed')
            dispatch.assert_not_called()
        self.assertIsNone(w.status(self.path, 'personal', NOW)['last_attempt'])

    def test_running_callback_observes_disable(self):
        def run(env, directory, cancelled, binary):
            self.assertFalse(cancelled())
            self.p['enabled'] = False
            self.save()
            self.assertTrue(cancelled())
            return 'cancelled_outcome_unknown'
        with patch.object(w, 'auth', return_value=w.identity(AUTH)), patch.object(w, 'dispatch', side_effect=run):
            self.assertEqual(w.tick(self.path, 'personal', True, NOW)['outcome'], 'cancelled_outcome_unknown')

    def test_invalid_config_and_timestamp_rejected(self):
        for value in ('not-a-date', '2026-10-18T00:00:00', None):
            with self.subTest(value=value), self.assertRaises((TypeError, ValueError)):
                w.stamp(value)
        for field, value in [('enabled', 1), ('slots', ['24:00']), ('slots', []),
                             ('native_default', 'true'), ('allow_unverified_schedule', 1), ('mode', 'other')]:
            old = self.p[field]
            self.p[field] = value
            self.save()
            with self.subTest(field=field), self.assertRaises(ValueError):
                w.load(self.path, 'personal')
            self.p[field] = old
        self.save()

    def test_schedule_generation_only_and_no_overwrite(self):
        output = self.parent / 'schedule'
        with patch.object(w.subprocess, 'run') as process:
            self.assertFalse(w.schedules(self.path, 'personal', output)['installed'])
            process.assert_not_called()
        self.assertEqual(len(list(output.iterdir())), 4)
        self.assertIn('--execute', (output / 'local.claude-warmup.personal.service').read_text())
        with self.assertRaises(ValueError):
            w.schedules(self.path, 'personal', output)

    def test_observed_advancement_not_attributed(self):
        with patch.object(w.time, 'time', return_value=NOW):
            code, result = self.invoke('observe', 'personal', '--five-hour-reset', w.iso(NOW + 3600),
                                      '--weekly-reset', w.iso(NOW + 86400), '--five-hour-utilization', '1',
                                      '--weekly-utilization', '1', '--observed-at', w.iso(NOW))
        self.assertEqual(code, 0)
        self.assertTrue(result['reset_advancement_observed'])
        self.assertEqual(result['attribution'], 'unverified')


if __name__ == '__main__':
    unittest.main()
