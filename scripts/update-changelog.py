#!/usr/bin/env python3
"""Append newly pushed commits to docs/CHANGELOG.md."""

from __future__ import annotations

import subprocess
import sys
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo


MARKER = "<!-- AUTO-CHANGELOG: entries are inserted below this line. -->"
ZERO_SHA = "0" * 40


def git(*args: str) -> str:
    return subprocess.check_output(["git", *args], text=True).strip()


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: update-changelog.py <before-sha> <after-sha>", file=sys.stderr)
        return 2

    before, after = sys.argv[1:]
    revision = after if before == ZERO_SHA else f"{before}..{after}"
    output = git("log", "--reverse", "--no-merges", "--format=%H%x09%s", revision)
    changelog = Path("docs/CHANGELOG.md")
    content = changelog.read_text(encoding="utf-8")

    entries: list[str] = []
    for line in output.splitlines():
        sha, subject = line.split("\t", 1)
        if "[skip changelog]" in subject.lower():
            continue
        short_sha = sha[:7]
        if f"`{short_sha}`" not in content:
            entries.append(f"- `{short_sha}` {subject}")

    if not entries:
        print("No new changelog entries.")
        return 0
    if MARKER not in content:
        print(f"missing changelog marker: {MARKER}", file=sys.stderr)
        return 1

    timestamp = datetime.now(ZoneInfo("Asia/Shanghai")).strftime("%Y-%m-%d %H:%M")
    block = f"\n\n### {timestamp}\n\n" + "\n".join(entries)
    changelog.write_text(content.replace(MARKER, MARKER + block, 1), encoding="utf-8")
    print(f"Added {len(entries)} changelog entr{'y' if len(entries) == 1 else 'ies'}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
