#!/usr/bin/env python3
"""Load the simple .env format and run the locally built Go application."""
from pathlib import Path
import os

root = Path(__file__).resolve().parents[1]
env = root / '.env'
if not env.exists():
    raise SystemExit('Run python3 tools/setup.py, then configure .env first.')
for raw in env.read_text().splitlines():
    line = raw.strip()
    if line and not line.startswith('#') and '=' in line:
        k, v = line.split('=', 1)
        os.environ.setdefault(k.strip(), v.strip().strip('"').strip("'"))
binary = root / 'bin' / 'bolty'
if not binary.exists():
    raise SystemExit('Run make build first.')
os.chdir(root)
os.execve(str(binary), [str(binary)], os.environ)
