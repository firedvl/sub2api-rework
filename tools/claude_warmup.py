#!/usr/bin/env python3
"""Disabled-by-default, local official Claude subscription warm-up companion."""
import argparse
import functools
from datetime import datetime, timedelta, timezone
import hashlib
import json
import math
import os
from pathlib import Path
import plistlib
import re
import signal
import shutil
import sqlite3
import stat
import subprocess
import sys
import tempfile
import time
from zoneinfo import ZoneInfo

from claude_accounts import list_profiles, private_directory, profile_environment

WARNING = ('No request-scoped paid-overflow guarantee exists. Account billing settings may change. '
           'Completion does not prove a quota reset. No credits are authorized or redeemed by this tool.')
GUARD = 5 * 3600
FRESH = 180
STATE_ROOT = Path.home() / ".claude-warmup-state"
QUALIFIED_CLIENT_VERSION = '2.1.300'


class UnqualifiedClientVersion(ValueError):
    pass


def stamp(value):
    if not isinstance(value, str):
        raise ValueError('timestamp must be an ISO8601 string with timezone')
    dt = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if dt.tzinfo is None:
        raise ValueError('timestamp must include timezone')
    return dt.timestamp()


def iso(value):
    return datetime.fromtimestamp(value, timezone.utc).isoformat() if value is not None else None


def private_file(path):
    if path.is_symlink():
        raise ValueError('Refusing symlink file')
    info = path.stat()
    if not stat.S_ISREG(info.st_mode):
        raise ValueError('Expected a regular file')
    if os.name == 'posix' and (info.st_mode & 0o077 or info.st_uid != os.getuid()):
        raise ValueError('File must belong to current user and be private (chmod 600)')


def write_config(path, config):
    private_directory(path.parent, False)
    if path.exists() or path.is_symlink():
        private_file(path)
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix='.config-')
    try:
        with os.fdopen(fd, 'w') as stream:
            json.dump(config, stream, indent=2, allow_nan=False)
            stream.write('\n')
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def load(path, profile):
    private_directory(path.parent, False)
    private_file(path)
    config = json.loads(path.read_text())
    p = config['profiles'][profile]
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_-]{0,63}', profile):
        raise ValueError('Invalid profile name')
    if not isinstance(config.get('claude_binary'), str) or not Path(config['claude_binary']).is_absolute():
        raise ValueError('claude_binary must be an absolute official CLI path')
    if not Path(config['profile_root']).is_absolute():
        raise ValueError('profile_root must be absolute')
    if type(config['enabled']) is not bool or type(p['enabled']) is not bool:
        raise ValueError('enabled must be boolean')
    if p['mode'] not in ('observed_reset', 'scheduled_unverified'):
        raise ValueError('Invalid mode')
    if type(p.get('native_default', False)) is not bool:
        raise ValueError('native_default must be boolean')
    if type(p['allow_unverified_schedule']) is not bool:
        raise ValueError('allow_unverified_schedule must be boolean')
    ZoneInfo(p['timezone'])
    if not isinstance(p['slots'], list) or not p['slots'] or any(
            not isinstance(s, str) or not re.fullmatch(r'(?:[01]\d|2[0-3]):[0-5]\d', s) for s in p['slots']):
        raise ValueError('slots must contain daily HH:MM times')
    return config, p


def occurrence(p, now, future=False):
    """Use wall-clock occurrence keys; roundtrip excludes nonexistent DST slots."""
    zone = ZoneInfo(p['timezone'])
    local = datetime.fromtimestamp(now, zone)
    candidates = []
    for offset in (range(3) if future else (-1, 0)):
        day = local.date() + timedelta(days=offset)
        for slot in set(p['slots']):
            hour, minute = map(int, slot.split(':'))
            dt = datetime(day.year, day.month, day.day, hour, minute, tzinfo=zone, fold=0)
            ts = dt.timestamp()
            if datetime.fromtimestamp(ts, zone).replace(tzinfo=None) != dt.replace(tzinfo=None):
                continue
            if (ts > now if future else 0 <= now - ts <= 600):
                candidates.append((ts, f'{day}:{slot}:{p["timezone"]}'))
    return min(candidates) if candidates else None


def observation(value, now):
    if not isinstance(value, dict) or value.get('source') != 'manual_official_ui':
        raise ValueError('Manual official UI observation required')
    observed = stamp(value['observed_at'])
    if not 0 <= now - observed <= FRESH:
        raise ValueError('Observation missing, future, or older than three minutes')
    for key in ('five_hour_utilization', 'weekly_utilization'):
        number = value[key]
        if type(number) not in (int, float) or not math.isfinite(number) or not 0 <= number <= 100:
            raise ValueError('Utilization must be finite and between 0 and 100')
    five, week = stamp(value['five_hour_reset']), stamp(value['weekly_reset'])
    if week <= now:
        raise ValueError('Weekly reset must be in the future')
    return five, week


def gate(config, p, now):
    if not config['enabled'] or not p['enabled']:
        return 'disabled'
    if p.get('billing_identity_digest') != p.get('identity_digest'):
        return 'billing_identity_unverified'
    if p.get('billing_mode') != 'credits_disabled_attested':
        return 'billing_unverified'
    try:
        ack = stamp(p['billing_acknowledged_at'])
        if not 0 <= now - ack <= 86400:
            return 'billing_acknowledgment_expired'
    except (KeyError, TypeError, ValueError):
        return 'billing_acknowledgment_missing'
    if not re.fullmatch('[0-9a-f]{64}', (p.get('identity_digest') or '')):
        return 'identity_not_bound'
    try:
        if not p.get('observation') or p['observation'].get('identity_digest') != p['identity_digest']:
            raise ValueError('Observation identity mismatch')
        five, _ = observation(p.get('observation'), now)
        if p['observation']['weekly_utilization'] >= 100:
            return 'weekly_exhausted'
        if five > now:
            return 'five_hour_window_active'
        if p['observation']['five_hour_utilization'] >= 100 and five == now:
            return 'five_hour_exhausted'
    except (KeyError, TypeError, ValueError):
        if p['mode'] == 'observed_reset':
            return 'quota_unknown'
    if p['mode'] == 'scheduled_unverified' and p['allow_unverified_schedule'] is not True:
        return 'unverified_schedule_not_authorized'
    return None


def identity(result):
    if result.get('apiKeySource'):
        raise ValueError('API key source is forbidden for subscription warm-up')
    if (result.get('loggedIn') is not True or result.get('authMethod') != 'claude.ai'
            or result.get('apiProvider') != 'firstParty'
            or result.get('subscriptionType') not in ('pro', 'max', 'team', 'enterprise')):
        raise ValueError('Official first-party Claude subscription identity required')
    email = result.get('email')
    if not isinstance(email, str) or not email.strip() or '@' not in email:
        raise ValueError('Official auth identity unavailable')
    return hashlib.sha256(email.strip().casefold().encode()).hexdigest()


def client_version(env, directory, binary):
    version = subprocess.run([binary, '--version'], env=env, cwd=directory,
                             capture_output=True, text=True, timeout=15)
    if version.returncode or version.stdout.strip() != QUALIFIED_CLIENT_VERSION + ' (Claude Code)':
        raise UnqualifiedClientVersion('Native client requires qualification')


def auth(env, directory, binary='claude'):
    env = dict(env, CLAUDE_CODE_SAFE_MODE='1')
    client_version(env, directory, binary)
    result = subprocess.run([binary, '--settings', '{"forceLoginMethod":"claudeai"}',
                             '--setting-sources', '', 'auth', 'status', '--json'],
                            env=env, cwd=directory,
                            capture_output=True, text=True, timeout=15)
    if result.returncode:
        raise ValueError('Native authentication status failed')
    return identity(json.loads(result.stdout))


def state(path, create=True):
    database = STATE_ROOT / 'state.sqlite3'
    if not database.exists() and not create:
        return None
    private_directory(STATE_ROOT, create)
    if database.exists() or database.is_symlink():
        private_file(database)
    else:
        try:
            fd = os.open(database, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
            os.close(fd)
        except FileExistsError:
            private_file(database)
    db = sqlite3.connect(database if create else database.resolve().as_uri() + '?mode=ro',
                         uri=not create, timeout=5, isolation_level=None)
    db.row_factory = sqlite3.Row
    if not create:
        return db
    db.execute('CREATE TABLE IF NOT EXISTS profiles (name TEXT PRIMARY KEY, last_attempt REAL, '
               'last_completed REAL, last_failure TEXT, outcome TEXT, cancel INTEGER NOT NULL DEFAULT 0)')
    db.execute('CREATE TABLE IF NOT EXISTS identities (digest TEXT PRIMARY KEY, last_attempt REAL NOT NULL)')
    db.execute('CREATE TABLE IF NOT EXISTS slots (profile TEXT, occurrence TEXT, preflights INTEGER, '
               'checked REAL, dispatched INTEGER DEFAULT 0, PRIMARY KEY(profile, occurrence))')
    db.execute('CREATE TABLE IF NOT EXISTS checks (profile TEXT PRIMARY KEY, checked REAL NOT NULL, reason TEXT)')
    return db


def state_key(path, name):
    return hashlib.sha256(str(path.resolve()).encode()).hexdigest() + ':' + name


def ensure(db, name):
    db.execute('INSERT OR IGNORE INTO profiles(name) VALUES (?)', (name,))


def schedule_label(path, name):
    return 'local.claude-warmup.' + name + '.' + state_key(path, name)[:12]


def scheduler_status(path, name):
    label = schedule_label(path, name)
    if sys.platform == 'darwin':
        command = ['/bin/launchctl', 'print', f'gui/{os.getuid()}/{label}']
    elif sys.platform == 'linux':
        command = ['systemctl', '--user', 'show', label + '.timer', '--property=LoadState', '--value']
    elif os.name == 'nt':
        command = [str(Path(os.environ['SystemRoot']) / 'System32' / 'schtasks.exe'), '/Query', '/TN', label]
    else:
        return {'label': label, 'installed': None, 'inspection': 'unsupported_platform'}
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=5)
    except (OSError, subprocess.SubprocessError):
        return {'label': label, 'installed': None, 'inspection': 'query_failed'}
    if result.returncode == 0:
        installed = result.stdout.strip() != 'not-found'
    elif sys.platform == 'darwin' and 'Could not find service' in result.stderr:
        installed = False
    else:
        return {'label': label, 'installed': None, 'inspection': 'query_failed'}
    return {'label': label, 'installed': installed, 'inspection': 'queried'}


def status(path, name, now):
    config, p = load(path, name)
    db = state(path, False)
    try:
        row = db.execute('SELECT * FROM profiles WHERE name=?', (state_key(path, name),)).fetchone() if db else None
        # Older ledgers have no scheduler-check table; do not migrate during inspection.
        has_checks = db and db.execute("SELECT 1 FROM sqlite_master WHERE type='table' AND name='checks'").fetchone()
        checked = db.execute('SELECT * FROM checks WHERE profile=?', (state_key(path, name),)).fetchone() if has_checks else None
        result = dict(row) if row else {}
    finally:
        if db:
            db.close()
    next_slot = occurrence(p, now, True)
    observed = p.get('observation')
    observation_age = None
    observation_fresh = False
    if observed:
        try:
            observation_age = now - stamp(observed['observed_at'])
            observation(observed, now)
            observation_fresh = observed.get('identity_digest') == p.get('identity_digest')
        except (KeyError, TypeError, ValueError):
            pass
    ack_expiration = None
    if p.get('billing_acknowledged_at'):
        try:
            ack_expiration = iso(stamp(p['billing_acknowledged_at']) + 86400)
        except (TypeError, ValueError):
            pass
    result.update(profile=name, enabled=config['enabled'] and p['enabled'], mode=p['mode'],
                  last_attempt=iso(result.get('last_attempt')), last_completed=iso(result.get('last_completed')),
                  last_failure=result.get('last_failure'), next_schedule=iso(next_slot[0]) if next_slot else None,
                  observation=p.get('observation'), billing_warning=WARNING,
                  observation_age_seconds=observation_age, observation_fresh=observation_fresh,
                  billing_acknowledgment_expires_at=ack_expiration,
                  scheduler=scheduler_status(path, name),
                  last_check=iso(checked['checked']) if checked else None,
                  last_skip_reason=checked['reason'] if checked else None,
                  verification=gate(config, p, now) or 'preflight_required',
                  quota_awareness='manual_official_ui' if p['mode'] == 'observed_reset' else 'unverified',
                  reset_attribution='unverified')
    if result.get('outcome') == 'in_flight':
        result['outcome'] = 'outcome_unknown'  # Never resend an interrupted dispatch.
    return result


def readiness(path, name, now):
    """Inspect local prerequisites without auth, inference or persistent writes."""
    try:
        config, p = load(path, name)
    except FileNotFoundError:
        return {'profile': name, 'configuration': 'missing', 'reason': 'configuration_or_parent_missing'}
    except (OSError, ValueError, KeyError, TypeError):
        return {'profile': name, 'configuration': 'invalid', 'reason': 'invalid_or_inaccessible_configuration'}
    result = status(path, name, now)
    try:
        env = profile_environment(config['profile_root'], name, native_default=p.get('native_default', False))
    except (OSError, ValueError):
        qualified = 'profile_unavailable'
    else:
        try:
            client_version(dict(env, CLAUDE_CODE_SAFE_MODE='1'), path.parent, config['claude_binary'])
            qualified = 'qualified'
        except UnqualifiedClientVersion:
            qualified = 'unqualified'
        except (OSError, subprocess.SubprocessError):
            qualified = 'inspection_failed'
    bound = bool(re.fullmatch('[0-9a-f]{64}', p.get('identity_digest') or ''))
    ack = 'absent'
    if p.get('billing_acknowledged_at'):
        try:
            age = now - stamp(p['billing_acknowledged_at'])
            ack = 'expired' if not 0 <= age <= 86400 else 'valid_identity_bound'
            if ack == 'valid_identity_bound' and (not bound or p.get('billing_identity_digest') != p['identity_digest']
                                                or p.get('billing_mode') != 'credits_disabled_attested'):
                ack = 'identity_or_billing_mode_unverified'
        except (TypeError, ValueError):
            ack = 'invalid'
    result.update(configuration='valid', client=qualified, qualified_client_version=QUALIFIED_CLIENT_VERSION,
                  identity_binding='bound' if bound else 'unbound', current_identity='unknown_not_checked',
                  billing_acknowledgment=ack, billing_control='operator_attestation_not_provider_verification',
                  quota_evidence=('fresh' if result['observation_fresh'] else
                                  'stale_or_invalid' if p.get('observation') else 'unavailable'),
                  live_acceptance='external_authorization_required', api_pool='unknown_external_gateway_state')
    return result


def terminate(child):
    if child.poll() is not None:
        return
    if os.name == 'posix':
        os.killpg(child.pid, signal.SIGTERM)
    else:
        subprocess.run([str(Path(os.environ['SystemRoot']) / 'System32' / 'taskkill.exe'),
                        '/PID', str(child.pid), '/T', '/F'], capture_output=True, timeout=5, check=False)
    try:
        child.wait(timeout=2)
    except subprocess.TimeoutExpired:
        if os.name == 'posix':
            os.killpg(child.pid, signal.SIGKILL)
        else:
            child.kill()
        child.wait(timeout=2)


def handle_signals(function):
    @functools.wraps(function)
    def run(*args, **kwargs):
        watched = [signal.SIGTERM, signal.SIGINT]
        if hasattr(signal, 'SIGHUP'):
            watched.append(signal.SIGHUP)
        previous = {sig: signal.getsignal(sig) for sig in watched}
        interrupted = [False]
        def interrupt(signum, frame):
            interrupted[0] = True
        try:
            for sig in watched:
                signal.signal(sig, interrupt)
            return function(*args, **kwargs, _interrupted=lambda: interrupted[0])
        finally:
            for sig, handler in previous.items():
                signal.signal(sig, handler)
    return run


@handle_signals
def dispatch(env, directory, cancelled, binary='claude', _interrupted=lambda: False):
    command = [binary, '--settings', '{"forceLoginMethod":"claudeai"}',
               '--setting-sources', '', '-p', 'Reply only OK.', '--model', 'haiku',
               '--tools', '', '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
               '--max-turns', '1', '--no-session-persistence', '--output-format', 'json',
               '--disable-slash-commands', '--permission-mode', 'dontAsk']
    env = dict(env, CLAUDE_CODE_SAFE_MODE='1', CLAUDE_CODE_MAX_OUTPUT_TOKENS='16',
               CLAUDE_CODE_MAX_RETRIES='0', CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC='1')
    # Spool bounded-duration output to private temporary files; never persist raw output in state.
    with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
        child = None
        try:
            if _interrupted():
                return 'execution_interrupted_outcome_unknown'
            child = subprocess.Popen(command, env=env, cwd=directory, stdout=output, stderr=errors,
                                     stdin=subprocess.DEVNULL, start_new_session=os.name == 'posix')
            deadline = time.monotonic() + 60
            while child.poll() is None:
                if _interrupted():
                    return 'execution_interrupted_outcome_unknown'
                if cancelled():
                    return 'cancelled_outcome_unknown'
                if time.monotonic() >= deadline:
                    return 'timeout_outcome_unknown'
                time.sleep(.2)
            output.seek(0)
            raw = output.read(1024 * 1024 + 1)
            if len(raw) > 1024 * 1024 or child.returncode:
                return 'execution_failed_outcome_unknown'
            try:
                result = json.loads(raw)
                if result.get('type') == 'result' and result.get('subtype') == 'success' and result.get('is_error') is False:
                    return 'request_completed_reset_unverified'
            except (ValueError, AttributeError):
                pass
            return 'invalid_result_outcome_unknown'
        finally:
            if child is not None:
                terminate(child)


def tick(path, name, execute=False, now=None):
    result = attempt(path, name, execute, now)
    if execute:
        db = state(path)
        try:
            db.execute('INSERT INTO checks(profile,checked,reason) VALUES (?,?,?) '
                       'ON CONFLICT(profile) DO UPDATE SET checked=excluded.checked,reason=excluded.reason',
                       (state_key(path, name), time.time() if now is None else now, result.get('skipped')))
        finally:
            db.close()
    return result


def attempt(path, name, execute=False, now=None):
    live_clock = now is None
    now = time.time() if live_clock else now
    config, p = load(path, name)
    reason = gate(config, p, now)
    slot = occurrence(p, now)
    if reason or not slot:
        return {'skipped': reason or 'not_due', 'billing_warning': WARNING}
    if not execute:
        return {'dry_run': True, 'would_preflight': True, 'occurrence': slot[1], 'billing_warning': WARNING}
    env = profile_environment(config['profile_root'], name, native_default=p.get('native_default', False))
    key = state_key(path, name)
    db = state(path)
    try:
        db.execute('BEGIN IMMEDIATE')
        ensure(db, key)
        row = db.execute('SELECT * FROM profiles WHERE name=?', (key,)).fetchone()
        account = db.execute('SELECT last_attempt FROM identities WHERE digest=?', (p['identity_digest'],)).fetchone()
        if ((account and now - account['last_attempt'] < GUARD)
                or (row['last_attempt'] is not None and now - row['last_attempt'] < GUARD)):
            db.execute('COMMIT')
            return {'skipped': 'five_hour_attempt_guard'}
        previous = db.execute('SELECT * FROM slots WHERE profile=? AND occurrence=?', (key, slot[1])).fetchone()
        if previous and (previous['dispatched'] or previous['preflights'] >= 2 or now - previous['checked'] < 60):
            db.execute('COMMIT')
            return {'skipped': 'duplicate_or_preflight_backoff'}
        db.execute('INSERT INTO slots(profile,occurrence,preflights,checked) VALUES (?,?,1,?) '
                   'ON CONFLICT(profile,occurrence) DO UPDATE SET preflights=preflights+1,checked=excluded.checked',
                   (key, slot[1], now))
        generation = row['cancel']
        db.execute('COMMIT')
        with tempfile.TemporaryDirectory(prefix='claude-warmup-', dir=path.parent) as directory:
            try:
                actual = auth(env, directory, config['claude_binary'])
                if actual != p['identity_digest']:
                    raise ValueError('Identity mismatch')
            except UnqualifiedClientVersion:
                db.execute('UPDATE profiles SET last_failure=? WHERE name=?', ('unqualified_client_version', key))
                return {'skipped': 'unqualified_client_version'}
            except (OSError, ValueError, subprocess.SubprocessError):
                db.execute('UPDATE profiles SET last_failure=? WHERE name=?', ('auth_preflight_failed', key))
                return {'skipped': 'auth_preflight_failed'}
            def cancelled():
                latest, selected = load(path, name)
                current = db.execute('SELECT cancel FROM profiles WHERE name=?', (key,)).fetchone()[0]
                return current != generation or not latest['enabled'] or not selected['enabled']
            db.execute('BEGIN IMMEDIATE')
            # Revalidate mutable authorization immediately before the durable dispatch claim.
            latest, selected = load(path, name)
            now = time.time() if live_clock else now
            reason = gate(latest, selected, now)
            row = db.execute('SELECT * FROM profiles WHERE name=?', (key,)).fetchone()
            account = db.execute('SELECT last_attempt FROM identities WHERE digest=?', (actual,)).fetchone()
            if (reason or cancelled()
                    or (account and now - account['last_attempt'] < GUARD) or selected['identity_digest'] != actual
                    or latest != config or selected != p
                    or occurrence(selected, now) != slot
                    or (row['last_attempt'] is not None and now - row['last_attempt'] < GUARD)):
                db.execute('COMMIT')
                return {'skipped': reason or 'authorization_changed'}
            db.execute('INSERT INTO identities(digest,last_attempt) VALUES (?,?) '
                       'ON CONFLICT(digest) DO UPDATE SET last_attempt=excluded.last_attempt', (actual, now))
            db.execute('UPDATE profiles SET last_attempt=?,outcome=?,last_failure=NULL WHERE name=?',
                       (now, 'in_flight', key))
            db.execute('UPDATE slots SET dispatched=1 WHERE profile=? AND occurrence=?', (key, slot[1]))
            db.execute('COMMIT')
            try:
                outcome = dispatch(env, directory, cancelled, config['claude_binary'])
            except (OSError, ValueError, subprocess.SubprocessError, KeyboardInterrupt):
                outcome = 'execution_interrupted_outcome_unknown'
            completed = outcome == 'request_completed_reset_unverified'
            db.execute('UPDATE profiles SET outcome=?,last_completed=CASE WHEN ? THEN ? ELSE last_completed END,'
                       'last_failure=? WHERE name=?',
                       (outcome, completed, time.time() if live_clock else now, None if completed else outcome, key))
            return {'outcome': outcome, 'reset_attribution': 'unverified'}
    finally:
        db.close()


def schedules(path, name, output):
    load(path, name)
    private_directory(output, True)
    command = [str(Path(sys.executable).resolve()), str(Path(__file__).resolve()),
               '--config', str(path.resolve()), 'tick', name, '--execute']
    label = schedule_label(path, name)
    filenames = [label + suffix for suffix in ('.plist', '.service', '.timer', '.ps1')]
    if any((output / filename).exists() or (output / filename).is_symlink() for filename in filenames):
        raise ValueError('Schedule output already exists; use a new private directory')
    with (output / (label + '.plist')).open('xb') as stream:
        plistlib.dump({'Label': label, 'ProgramArguments': command, 'StartInterval': 60,
                      'RunAtLoad': False}, stream)
    # systemd argument quoting differs from shell quoting; percent expands specifiers.
    if any(any(c in value for c in ('\n', '\r', '\x00')) for value in command):
        raise ValueError('Schedule paths cannot contain control characters')
    quoted = ' '.join('"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%').replace('$', '$$') + '"' for value in command)
    (output / (label + '.service')).write_text('[Unit]\nDescription=Local Claude warm-up\n[Service]\nType=oneshot\nExecStart=' + quoted + '\n')
    (output / (label + '.timer')).write_text('[Unit]\nDescription=Check local Claude warm-up slots\n[Timer]\nOnCalendar=*-*-* *:*:00\nPersistent=false\n[Install]\nWantedBy=timers.target\n')
    ps = lambda s: "'" + s.replace("'", "''") + "'"
    # Windows PowerShell writes a native Task Scheduler task only when explicitly run.
    arguments = subprocess.list2cmdline(command[1:])
    (output / (label + '.ps1')).write_text(
        '$action = New-ScheduledTaskAction -Execute ' + ps(command[0]) + ' -Argument ' + ps(arguments) + '\n'
        '$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1) -RepetitionInterval (New-TimeSpan -Minutes 1)\n'
        'Register-ScheduledTask -TaskName ' + ps(label) + ' -Action $action -Trigger $trigger\n')
    for filename in filenames:
        (output / filename).chmod(0o600)
    return {'generated': str(output), 'label': label, 'installed': False, 'billing_warning': WARNING}


def initial_profile(zone):
    return {'enabled': False, 'mode': 'observed_reset', 'timezone': zone, 'slots': ['08:00'],
            'native_default': False, 'allow_unverified_schedule': False, 'billing_mode': 'unverified',
            'billing_acknowledged_at': None, 'identity_digest': None, 'observation': None}


def setup(path, root, binary):
    """Prepare disabled settings and preview schedules; never authenticate or infer."""
    config = None
    if path.exists() or path.is_symlink():
        private_directory(path.parent, False)
        private_file(path)
        config = json.loads(path.read_text())
        root = Path(config['profile_root'])
        binary = config['claude_binary']
    names = sorted(set(list_profiles(root)) | (set(config['profiles']) if config else set()))
    print('Profiles (+default selects the existing official default login; no login is changed):')
    print('\n'.join(names))
    name = input('Select profile: ').strip()
    use_default = name == '+default'
    if use_default:
        name = input('Name for default-login binding [native-default]: ').strip() or 'native-default'
        if name in names and not (config and config['profiles'].get(name, {}).get('native_default')):
            raise ValueError('Name already belongs to an isolated profile; choose a different binding name')
        if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_-]{0,63}', name):
            raise ValueError('Invalid binding name')
    elif name not in names:
        raise ValueError('Select a listed profile')
    if config and name in config['profiles']:
        _, previous = load(path, name)
        p = dict(previous)
    else:
        p = initial_profile('UTC')
        p['native_default'] = use_default
    zone = input(f'Timezone [{p["timezone"]}]: ').strip() or p['timezone']
    ZoneInfo(zone)
    slots = input(f'Daily times, comma separated [{",".join(p["slots"])}]: ').strip()
    slots = sorted(set(s.strip() for s in slots.split(','))) if slots else p['slots']
    if not slots or any(not re.fullmatch(r'(?:[01]\d|2[0-3]):[0-5]\d', s) for s in slots):
        raise ValueError('Use daily HH:MM times')
    print('observed_reset requires a fresh manual official-UI observation; it is not automated quota verification.\n'
          'scheduled_unverified has no trustworthy reset/billing preflight and may consume paid usage credits.\n' + WARNING)
    mode = input(f'Mode [{p["mode"]}]: ').strip() or p['mode']
    if mode not in ('observed_reset', 'scheduled_unverified'):
        raise ValueError('Unknown scheduling mode')
    p.update(enabled=False, timezone=zone, slots=slots, mode=mode, allow_unverified_schedule=False)
    print('Saving will disable this profile. Before any later activation, bind the intended native identity, inspect billing in the official UI,\n'
          'then acknowledge with acknowledge-billing PROFILE --credits-disabled --acknowledge-no-overflow-guarantee.\n'
          'That acknowledgement expires after 24 hours and is not a zero-overage guarantee.')
    if input('Save disabled configuration and preview schedules? [y/N]: ').strip().casefold() != 'y':
        return {'saved': False, 'configuration': 'unchanged'}
    if config is None:
        if binary is None:
            raise ValueError('Install official Claude Code or provide --claude')
        config = {'enabled': False, 'claude_binary': str(Path(binary).expanduser().absolute()),
                  'profile_root': str(root.expanduser().absolute()), 'profiles': {}}
    config['profiles'][name] = p
    private_directory(path.parent, True)
    write_config(path, config)
    with tempfile.TemporaryDirectory(prefix='schedule-preview-', dir=path.parent) as directory:
        output = Path(directory)
        schedules(path, name, output)
        preview = {file.name: file.read_text() for file in sorted(output.iterdir())}
    return {'saved': True, 'enabled': False, 'profile': name, 'scheduler': scheduler_status(path, name),
            'schedule_preview': preview, 'dry_run': tick(path, name), 'billing_warning': WARNING}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    commands = parser.add_subparsers(dest='command', required=True)
    init = commands.add_parser('init')
    init.add_argument('profile')
    init.add_argument('--root', type=Path, required=True)
    init.add_argument('--claude', type=Path, default=shutil.which('claude'))
    init.add_argument('--timezone', default='America/Los_Angeles')
    wizard = commands.add_parser('setup', help='Interactive disabled setup; no login, inference or scheduler installation')
    wizard.add_argument('--root', type=Path, default=Path.home() / '.claude-accounts')
    wizard.add_argument('--claude', type=Path, default=shutil.which('claude'))
    for verb in ('bind', 'acknowledge-billing', 'observe', 'status', 'readiness', 'tick', 'disable', 'cancel', 'schedule'):
        sub = commands.add_parser(verb)
        sub.add_argument('profile')
        if verb == 'acknowledge-billing':
            sub.add_argument('--credits-disabled', action='store_true', required=True)
            sub.add_argument('--acknowledge-no-overflow-guarantee', action='store_true', required=True)
        if verb == 'tick':
            sub.add_argument('--execute', action='store_true')
        if verb == 'observe':
            sub.add_argument('--five-hour-reset', required=True)
            sub.add_argument('--weekly-reset', required=True)
            sub.add_argument('--five-hour-utilization', required=True, type=float)
            sub.add_argument('--weekly-utilization', required=True, type=float)
            sub.add_argument('--observed-at', required=True)
        if verb == 'schedule':
            sub.add_argument('--output', type=Path, required=True)
    args = parser.parse_args(argv)
    path = args.config.expanduser().absolute()
    now = time.time()
    try:
        if args.command == 'init':
            if path.exists() or path.is_symlink():
                raise ValueError('Config already exists; edit it explicitly')
            if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_-]{0,63}', args.profile):
                raise ValueError('Invalid profile')
            ZoneInfo(args.timezone)
            if args.claude is None:
                raise ValueError('Install official Claude Code or supply --claude absolute binary path')
            private_directory(path.parent, True)
            write_config(path, {'enabled': False, 'claude_binary': str(Path(args.claude).expanduser().absolute()), 'profile_root': str(args.root.expanduser().absolute()), 'profiles': {
                args.profile: initial_profile(args.timezone)}})
            result = {'initialized': True, 'enabled': False, 'billing_warning': WARNING}
        elif args.command == 'setup':
            result = setup(path, args.root, args.claude)
        elif args.command == 'tick':
            result = tick(path, args.profile, args.execute)
        elif args.command == 'status':
            result = status(path, args.profile, now)
        elif args.command == 'readiness':
            result = readiness(path, args.profile, now)
        elif args.command == 'schedule':
            result = schedules(path, args.profile, args.output.expanduser().absolute())
        else:
            config, p = load(path, args.profile)
            if args.command == 'bind':
                env = profile_environment(config['profile_root'], args.profile, native_default=p.get('native_default', False))
                with tempfile.TemporaryDirectory(prefix='claude-status-', dir=path.parent) as directory:
                    bound = auth(env, directory, config['claude_binary'])
                if bound != p.get('identity_digest'):
                    p.update(enabled=False, identity_digest=bound, billing_mode='unverified',
                             billing_acknowledged_at=None, billing_identity_digest=None, observation=None)
                write_config(path, config)
                result = {'identity_bound': True}
            elif args.command == 'acknowledge-billing':
                if not re.fullmatch('[0-9a-f]{64}', p.get('identity_digest') or ''):
                    raise ValueError('Bind identity first')
                p.update(billing_mode='credits_disabled_attested', billing_acknowledged_at=iso(now),
                         billing_identity_digest=p['identity_digest'])
                write_config(path, config)
                result = {'billing_attestation_recorded': True, 'billing_warning': WARNING}
            elif args.command == 'observe':
                if not re.fullmatch('[0-9a-f]{64}', p.get('identity_digest') or ''):
                    raise ValueError('Bind identity first')
                value = {'source': 'manual_official_ui', 'identity_digest': p['identity_digest'], 'observed_at': args.observed_at,
                         'five_hour_reset': args.five_hour_reset, 'weekly_reset': args.weekly_reset,
                         'five_hour_utilization': args.five_hour_utilization, 'weekly_utilization': args.weekly_utilization}
                observation(value, now)
                old = p.get('observation')
                advanced = bool(old and stamp(value['five_hour_reset']) > stamp(old['five_hour_reset']))
                p['observation'] = value
                write_config(path, config)
                result = {'reset_advancement_observed': advanced, 'attribution': 'unverified'}
            else:
                if args.command == 'disable':
                    p['enabled'] = False
                    write_config(path, config)
                db = state(path)
                key = state_key(path, args.profile)
                ensure(db, key)
                db.execute('UPDATE profiles SET cancel=cancel+1 WHERE name=?', (key,))
                db.close()
                result = {'cancel_requested': True, 'disabled': args.command == 'disable'}
        print(json.dumps(result, allow_nan=False))
        if args.command == 'readiness' and result['configuration'] != 'valid':
            return 1
        return 0
    except (OSError, ValueError, KeyError, TypeError, EOFError, sqlite3.Error, subprocess.SubprocessError) as exc:
        # Deliberately omit exception text: native output and account data must not leak.
        print(json.dumps({'error': 'Local configuration or native operation failed', 'kind': type(exc).__name__}), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
