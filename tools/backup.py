#!/usr/bin/env python3
"""Consistent SQLite backup, including committed WAL data. No raw-file live copy."""
from pathlib import Path
import argparse
import os
import sqlite3

p = argparse.ArgumentParser()
p.add_argument('database', type=Path)
p.add_argument('destination', type=Path)
a = p.parse_args()
src = a.database.resolve()
dst = a.destination.resolve()
if not src.is_file():
    raise SystemExit('Source database does not exist.')
if src == dst or dst.exists():
    raise SystemExit('Choose a new destination file, not the source or an existing backup.')
dst.parent.mkdir(parents=True, exist_ok=True)
fd = os.open(dst, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
os.close(fd)
try:
    with sqlite3.connect(src.as_uri() + '?mode=ro', uri=True) as source:
        with sqlite3.connect(dst) as target:
            source.backup(target)
            result = target.execute('PRAGMA integrity_check').fetchone()[0]
            if result != 'ok':
                raise RuntimeError('Backup failed SQLite integrity check: ' + result)
except BaseException:
    dst.unlink(missing_ok=True)
    raise
print('Verified backup:', dst)
