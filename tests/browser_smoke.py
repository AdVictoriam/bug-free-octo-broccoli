#!/usr/bin/env python3
"""Browser + real API smoke test. No credentials, synthetic AI results or network calls.
Default uses normal browser navigation. BOLTY_BROWSER_BRIDGE=1 supports environments
whose browser administrator blocks all URLs: renders the actual local assets and
bridges only /api/* to the real localhost server. That mode does NOT test browser
navigation, cookies, CSP enforcement, or downloads; API tests cover server rules.
"""
from __future__ import annotations
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from playwright.sync_api import sync_playwright

root = Path(__file__).resolve().parents[1]
p = argparse.ArgumentParser()
p.add_argument('--binary', default='bin/bolty')
p.add_argument('--output', default='artifacts')
a = p.parse_args()
output = (root / a.output).resolve(); output.mkdir(parents=True, exist_ok=True)
with socket.socket() as sock:
    sock.bind(('127.0.0.1', 0)); port = sock.getsockname()[1]
origin = f'http://127.0.0.1:{port}'
password = secrets.token_urlsafe(32)
bridge_mode = os.environ.get('BOLTY_BROWSER_BRIDGE') == '1'
with tempfile.TemporaryDirectory() as tmp:
    env = {**os.environ, 'APP_PASSWORD': password, 'PUBLIC_URL': origin,
           'LISTEN_ADDR': f'127.0.0.1:{port}', 'DATA_DIR': tmp,
           'OPENAI_API_KEY': '', 'ANTHROPIC_API_KEY': '',
           'OPENAI_MODEL': '', 'ANTHROPIC_MODEL': '',
           'YOUTUBE_CLIENT_ID': '', 'YOUTUBE_CLIENT_SECRET': '', 'YOUTUBE_REFRESH_TOKEN': ''}
    with (output / 'server.log').open('w') as log:
        server = subprocess.Popen([str((root / a.binary).resolve())], cwd=root, env=env, stdout=log, stderr=log)
        try:
            for _ in range(100):
                try:
                    with urllib.request.urlopen(origin + '/healthz', timeout=1) as r:
                        if r.status == 200: break
                except OSError: time.sleep(.05)
            else: raise RuntimeError('Server did not become healthy')
            jar = http.cookiejar.CookieJar()
            http = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
            def bridge(path, method, body):
                assert path.startswith('/api/') and '..' not in path
                req = urllib.request.Request(origin + path, data=body.encode() if body is not None else None,
                    method=method, headers={'Origin': origin, 'Content-Type': 'application/json'})
                try:
                    with http.open(req, timeout=15) as r:
                        return {'status': r.status, 'body': r.read().decode()}
                except urllib.error.HTTPError as e:
                    return {'status': e.code, 'body': e.read().decode()}
            with sync_playwright() as playwright:
                options = {'headless': True}
                if os.environ.get('CHROMIUM_EXECUTABLE'):
                    options['executable_path'] = os.environ['CHROMIUM_EXECUTABLE']
                browser = playwright.chromium.launch(**options)
                page = browser.new_page(viewport={'width': 1512, 'height': 1100}, device_scale_factor=1)
                errors = []
                page.on('pageerror', lambda error: errors.append(str(error)))
                if bridge_mode:
                    page.expose_function('studioAPI', bridge)
                    page.set_content('<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="app"></div><div id="modal-root"></div><div id="toast" role="status"></div></body></html>')
                    page.add_style_tag(content=(root / 'web/styles.css').read_text())
                    page.add_script_tag(content="window.fetch=async(path,opts={})=>{const r=await window.studioAPI(path,opts.method||'GET',opts.body??null);return new Response(r.body,{status:r.status,headers:{'Content-Type':'application/json'}})};")
                    page.add_script_tag(type='module', content=(root / 'web/app.js').read_text())
                else:
                    page.goto(origin, wait_until='networkidle')
                page.locator('[name=password]').fill(password)
                page.locator('button[type=submit]').click()
                page.locator('.hero').wait_for()
                def width_ok():
                    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 2'), page.evaluate('[innerWidth,document.documentElement.scrollWidth]')
                def logo():
                    if bridge_mode:
                        page.locator('img[src="/mark.svg"]').evaluate_all('(els,s)=>els.forEach(e=>e.src="data:image/svg+xml,"+encodeURIComponent(s))', (root/'web/mark.svg').read_text())
                def route(name):
                    page.evaluate('(r)=>location.hash=r', name); page.wait_for_timeout(150); width_ok(); logo()
                width_ok(); logo()
                page.screenshot(path=str(output / 'studio-desktop.png'), full_page=True)
                for name in ['library', 'dna', 'quality', 'production', 'models']:
                    route(name)
                    assert page.locator('h1').count() == 1
                route('library')
                assert page.locator('.source-card').count() == 6
                page.screenshot(path=str(output / 'library-desktop.png'), full_page=True)
                # Save a source with literal markup: no executable HTML may enter the DOM.
                page.locator('[data-action=new-source]').first.click()
                page.locator('#source-form [name=title]').fill('QA <img src=x onerror=alert(1)> reference')
                page.locator('#source-form [name=transcript]').fill('An original test transcript explaining a calm reveal and a fair resolution. ' * 12)
                page.locator('#source-form [name=rights]').check()
                page.locator('#source-form [value=save]').click()
                page.locator('#source-form').wait_for(state='hidden')
                assert page.locator('.source-card').count() == 7
                assert page.locator('img[onerror]').count() == 0
                card = page.locator('.source-card').filter(has_text='QA <img')
                card.locator('[data-action=edit-source]').click()
                page.locator('#source-form [value=analyse]').click()
                page.wait_for_timeout(200)
                assert 'model ID' in page.locator('#toast').inner_text()
                # Failure retained the saved source ID. Re-saving must not duplicate it.
                page.locator('#source-form [value=save]').click()
                page.locator('#source-form').wait_for(state='hidden')
                assert page.locator('.source-card').count() == 7
                # Story direction works without a provider, but paid generation is blocked.
                route('studio')
                page.locator('[data-action=new-project]').first.click()
                page.locator('#project-form [name=title]').fill('QA fair server rules')
                page.locator('#project-form [name=premise]').fill('A moderator invents an unfair rule. The owner restores fair access after a fictional evidence review.')
                page.locator('#project-form button[type=submit]').click()
                page.locator('#project-form').wait_for(state='hidden')
                page.wait_for_timeout(200)
                assert 'QA fair server rules' in page.locator('h1').inner_text()
                width_ok()
                page.locator('[data-action=ideas]').click()
                page.wait_for_timeout(150)
                assert 'reference' in page.locator('#toast').inner_text().lower()
                # Responsive route checks.
                page.set_viewport_size({'width': 390, 'height': 844})
                for name in ['studio', 'library', 'dna', 'quality', 'production', 'models']:
                    route(name)
                route('studio')
                page.screenshot(path=str(output / 'studio-mobile.png'), full_page=True)
                assert not errors, errors
                print(json.dumps({'mode': 'real-api-bridge' if bridge_mode else 'native-browser',
                    'desktop': '1512px', 'mobile': '390px', 'routes': 6,
                    'source_save': True, 'xss_escape': True, 'no_duplicate_on_failed_analysis': True,
                    'project_create': True, 'missing_configuration_blocks_AI': True,
                    'horizontal_overflow': False, 'javascript_errors': errors}, indent=2))
                browser.close()
        finally:
            server.terminate()
            try: server.wait(timeout=20)
            except subprocess.TimeoutExpired: server.kill(); server.wait()
