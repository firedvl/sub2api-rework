#!/usr/bin/env python3
"""Launch official Claude Code with an explicitly selected private profile."""

import argparse
import os
from pathlib import Path
import re
import stat
import subprocess
import sys


BLOCKED_PREFIXES = (
    "ANTHROPIC_", "CLAUDE_", "AWS_", "BEDROCK_", "VERTEX_", "GOOGLE_",
    "GCLOUD_", "CLOUD_ML_", "AZURE_", "OPENAI_",
)


def private_directory(path, create):
    if path.is_symlink():
        raise ValueError(f"Refusing symlink directory: {path}")
    if create:
        path.mkdir(mode=0o700, parents=False, exist_ok=True)
    mode = path.stat().st_mode
    if not stat.S_ISDIR(mode):
        raise ValueError(f"Not a directory: {path}")
    if os.name == "posix":
        if mode & 0o077:
            raise ValueError(f"Directory must be private (chmod 700): {path}")
        if path.stat().st_uid != os.getuid():
            raise ValueError(f"Directory must belong to the current user: {path}")


def main(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__,
        epilog=(
            "Uses official subscription authentication; this helper never reads or "
            "copies credentials. Each launch retains your real HOME and current project "
            "directory. Profile user settings and managed policy remain trusted; this "
            "is not a sandbox or a guarantee against paid usage. Configure billing in "
            "your Claude account. Automatic warm-up is unsupported."
        ),
    )
    parser.add_argument("--root", type=Path, default=Path.home() / ".claude-accounts",
                        help="Existing parent location for private profile directories (default: ~/.claude-accounts)")
    parser.add_argument("command", choices=("init", "login", "status", "run", "warmup"))
    parser.add_argument("profile", help="Explicit profile name: letters, digits, underscores or hyphens")
    args = parser.parse_args(argv)
    if args.command == "warmup":
        print("Automatic warm-up is disabled: no supported request-scoped guarantee "
              "against paid overflow is available. No request was sent.", file=sys.stderr)
        return 2
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,63}", args.profile):
        parser.error("profile must be a safe identifier of 1–64 letters, digits, underscores or hyphens")
    root = args.root.expanduser().absolute()
    profile = root / args.profile
    try:
        if root.resolve() == (Path.home() / ".claude").resolve():
            raise ValueError("The shared ~/.claude directory cannot be used as the profile root")
        create = args.command in ("init", "login")
        private_directory(root, create)
        private_directory(profile, create)
        anthropic_config = profile / "anthropic"
        private_directory(anthropic_config, create)
        env = {key: value for key, value in os.environ.items()
               if not key.upper().startswith(BLOCKED_PREFIXES)}
        env["CLAUDE_CONFIG_DIR"] = str(profile)
        env["ANTHROPIC_CONFIG_DIR"] = str(anthropic_config)
        command = ["claude", "--settings", '{"forceLoginMethod":"claudeai"}',
                   "--setting-sources", "user"]
        if args.command in ("init", "login"):
            command.extend(("auth", "login", "--claudeai"))
        elif args.command == "status":
            command.extend(("auth", "status", "--text"))
        print(f"Profile: {args.profile}\nConfig directory: {profile}\n"
              "Requested login: Claude subscription (claude.ai); confirm identity with /status", flush=True)
        return subprocess.run(command, env=env, check=False).returncode
    except (OSError, ValueError) as exc:
        print(f"claude-accounts: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    sys.exit(main())
