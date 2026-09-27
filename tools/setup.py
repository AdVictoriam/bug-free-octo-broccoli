#!/usr/bin/env python3
"""Create a private .env exactly once; never print API credentials."""
from pathlib import Path
import os
import secrets

root = Path(__file__).resolve().parents[1]
target = root / '.env'
if target.exists():
    raise SystemExit('.env already exists. Left unchanged; edit it directly.')
content = (root / '.env.example').read_text()
content = content.replace('APP_PASSWORD=\n', 'APP_PASSWORD=' + secrets.token_urlsafe(32) + '\n', 1)
fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as f:
    f.write(content)
print('Created .env with a random studio password. Open it locally to retrieve the password,')
print('add provider keys and exact model IDs, then run: docker compose up --build -d')
