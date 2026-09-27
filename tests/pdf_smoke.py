#!/usr/bin/env python3
"""Structural PDF test. The fixture is synthetic, never an editorial example."""
from pathlib import Path
import io
import json
import os
import re
import subprocess
import tempfile
import zipfile
import fitz

root = Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory() as tmp:
    fixture = Path(tmp) / 'fixture.json'
    subprocess.run(['go', 'test', './internal/studio', '-run', '^TestEmitFormatFixture$', '-count=1'],
                   cwd=root, env={**os.environ, 'BOLTY_FORMAT_FIXTURE': str(fixture)}, check=True)
    script = json.loads(fixture.read_text())
    with zipfile.ZipFile(str(fixture) + '.production.zip') as handoff:
        assert set(handoff.namelist()) == {'voice-actor.md', 'editor.md', 'voice-actor.pdf', 'editor.pdf'}
        assert b'%PDF-' == handoff.read('voice-actor.pdf')[:5]
        assert b'%PDF-' == handoff.read('editor.pdf')[:5]
        assert b'L01' in handoff.read('voice-actor.md') and b'L80' in handoff.read('editor.md')
        print('Authenticated API handoff: exactly four valid expected files')
    result = subprocess.run([os.environ.get('PYTHON_BIN', 'python3'), 'tools/render.py'],
                            input=fixture.read_bytes(), cwd=root, check=True, capture_output=True)
    with zipfile.ZipFile(io.BytesIO(result.stdout)) as z:
        assert set(z.namelist()) == {'voice-actor.pdf', 'editor.pdf'}
        for name in z.namelist():
            with fitz.open(stream=z.read(name), filetype='pdf') as doc:
                full = ''.join(page.get_text() for page in doc)
                actual_ids = [int(v) for v in re.findall(r'\bL(\d+)\b', full)]
                assert actual_ids == [line['id'] for line in script['lines']], (name, actual_ids)
                colours = set()
                for page in doc:
                    for block in page.get_text('dict')['blocks']:
                        for line in block.get('lines', []):
                            for span in line['spans']:
                                colours.add(span['color'])
                                x0, y0, x1, y1 = span['bbox']
                                assert 0 <= x0 <= x1 <= page.rect.width + 0.5, (name, span)
                                assert 0 <= y0 <= y1 <= page.rect.height + 0.5, (name, span)
                if name.startswith('voice'):
                    assert int('8242BB', 16) in colours, colours
                    assert 'Gameplay / general:' not in full and 'Additional non-spoken hold:' not in full
                    assert 'YO GUYS! Welcome back to the channel!' in full
                else:
                    assert int('2465A7', 16) in colours and int('B43B40', 16) in colours
                    assert full.index('Thumbnail') < full.index('Line-synced edit')
                print(f'{name}: {len(doc)} pages; all 80 line IDs, colours and page bounds verified')
print('PDF structural checks passed. This does not measure script quality.')
