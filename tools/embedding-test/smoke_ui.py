#!/usr/bin/env python3
"""Opt-in browser proof of the portable memory controls (no GPU inference).

Requires Playwright/Chromium in a development Python. Uses disposable fixtures,
never an existing installation. Example: python smoke_ui.py --app APP --report UI.json
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
from urllib.request import urlopen

from playwright.sync_api import sync_playwright, expect


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--app', required=True, type=Path)
    ap.add_argument('--report', required=True, type=Path)
    ap.add_argument('--screenshots', type=Path)
    args = ap.parse_args()
    args.report.parent.mkdir(parents=True, exist_ok=True)
    base = 'http://127.0.0.1:18090'
    with socket.socket() as probe:
        if probe.connect_ex(('127.0.0.1', 18090)) == 0:
            raise RuntimeError('Port 18090 already used; no existing process touched.')
    results, errors = {}, []
    with tempfile.TemporaryDirectory(prefix='ajean-ui-proof-') as work:
        work = Path(work)
        app = work/args.app.name
        shutil.copy2(args.app, app)
        data = work/'data'
        for folder in ['presets', 'models/embeddinggemma-2-full', 'backends/fixture', 'embedding-python']:
            (data/folder).mkdir(parents=True, exist_ok=True)
        (data/'models'/'proof.gguf').write_bytes(b'fixture, never loaded')
        (data/'models'/'embeddinggemma-2-full'/'config.json').write_text('{}')
        runtime = data/'backends'/'fixture'/'llama-server'
        runtime.write_text('#!/bin/sh\nprintf "Available devices:\\n  ROCm0: UI fixture\\n"\n')
        runtime.chmod(0o755)
        (data/'embedding-python'/'python.exe').write_bytes(b'fixture, never executed')
        (data/'presets'/'Memory-UI.env').write_text(
            '# NAME=Memory UI\nBIN=backends/fixture/llama-server\nMODEL=proof.gguf\nMEMORY_SYSTEM=classic\n', encoding='utf-8')
        (data/'presets'/'Remote-UI.env').write_text(
            '# NAME=Remote UI\nEXTERNAL=1\nEXTERNAL_URL=http://127.0.0.1:9/v1\nEXTERNAL_MODEL=fixture\n', encoding='utf-8')
        with (work/'app.log').open('w') as log:
            process = subprocess.Popen([str(app), 'app'], cwd=work,
                                       env=dict(os.environ, AJEAN_HOME=str(work/'decoy')),
                                       stdout=log, stderr=subprocess.STDOUT)
            try:
                deadline = time.monotonic()+30
                while True:
                    try:
                        with urlopen(base+'/api/embeddings', timeout=2) as response:
                            status = json.load(response)
                        break
                    except OSError:
                        if process.poll() is not None or time.monotonic()>deadline:
                            raise RuntimeError((work/'app.log').read_text())
                        time.sleep(.1)
                assert status['version'].endswith('-embedding-test.5'), status['version']
                assert Path(status['executable']) == app
                assert Path(status['home']) == data
                results['running_version'] = status['version']
                with sync_playwright() as pw:
                    browser = pw.chromium.launch(headless=True)
                    context = browser.new_context(viewport={'width': 1280, 'height': 1050}, locale='fr-FR')
                    page = context.new_page()
                    page.on('pageerror', lambda e: errors.append(str(e)))
                    page.goto(base+'/?memory-test='+status['version'], wait_until='domcontentloaded')
                    expect(page.locator('#memory-test-build')).to_be_visible()
                    expect(page.locator('#memory-test-build')).to_contain_text('TEST mémoire · R5')
                    results['served_revision_badge'] = 'PASS'

                    page.locator('summary:has([data-i18n="settings.section_title"])').click()
                    expect(page.locator('#memory-system')).to_be_visible()
                    page.locator('#memory-system').select_option('hybrid')
                    page.wait_for_function("async () => (await (await fetch('/api/embeddings')).json()).defaults.enabled === true")
                    row = page.locator('#memory-system').locator('xpath=../..')
                    row.locator('.help').click()
                    expect(row.locator('.tip')).to_contain_text('Classique')
                    expect(row.locator('.tip')).to_contain_text('Hybride')
                    row.locator('.help').click()
                    results['general_mode_and_help'] = 'PASS'
                    if args.screenshots:
                        args.screenshots.mkdir(parents=True, exist_ok=True)
                        page.screenshot(path=str(args.screenshots/'general.png'))

                    page.locator('summary:has([data-i18n="presets.title"])').click()
                    local_edit = page.locator('#presets [data-id="Memory-UI"] .preset-edit')
                    local_edit.click()
                    page.wait_for_selector('#modal:not(.loading) #m-memory-block', state='visible')
                    assert page.locator('#m-memory-block').evaluate("el => el.previousElementSibling.id") == 'm-memory-local-anchor'
                    assert page.locator('#m-memory-local-anchor').evaluate("el => el.previousElementSibling.id") == 's-mmproj-cpu-row'
                    page.locator('#m-memory-mode').select_option('hybrid')
                    for modality in ['image', 'audio', 'video']:
                        page.locator('#m-memory-'+modality).check()
                    expect(page.locator('#m-memory-provider')).to_have_value('transformers')
                    page.locator('#m-memory-device').select_option('gpu')
                    expect(page.locator('#m-memory-gpu')).to_be_visible()
                    expect(page.locator('#m-memory-gpu')).to_have_value('cuda:0')
                    page.locator('#m-memory-dimensions').select_option('256')
                    page.locator('#m-memory-threads').fill('6')
                    page.locator('#m-memory-threads').blur()
                    if args.screenshots:
                        page.locator('#m-memory-block').screenshot(path=str(args.screenshots/'preset.png'))
                    page.locator('#modal .pe-btn.primary').click()
                    expect(page.locator('#modal')).not_to_be_visible(timeout=15000)
                    text = (data/'presets'/'Memory-UI.env').read_text()
                    for marker in ['MEMORY_SYSTEM=hybrid', 'MEMORY_EMBED_PROVIDER=transformers', 'MEMORY_EMBED_DEVICE=gpu',
                                   'MEMORY_EMBED_MODALITIES=text,image,audio,video', 'MEMORY_EMBED_DIMENSIONS=256', 'MEMORY_EMBED_THREADS=6']:
                        assert marker in text, marker
                    local_edit.click()
                    page.wait_for_selector('#modal:not(.loading) #m-memory-block', state='visible')
                    expect(page.locator('#m-memory-device')).to_have_value('gpu')
                    expect(page.locator('#m-memory-dimensions')).to_have_value('256')
                    for modality in ['image', 'audio', 'video']:
                        expect(page.locator('#m-memory-'+modality)).to_be_checked()
                    page.locator('#m-memory-device').select_option('cpu')
                    expect(page.locator('#m-memory-gpu')).not_to_be_visible()
                    page.locator('#modal .pe-btn.primary').click()
                    expect(page.locator('#modal')).not_to_be_visible()
                    assert 'MEMORY_EMBED_DEVICE=cpu' in (data/'presets'/'Memory-UI.env').read_text()
                    results['preset_placement_cpu_gpu_modalities_save_reload'] = 'PASS'

                    page.locator('#presets [data-id="Remote-UI"] .preset-edit').click()
                    page.wait_for_selector('#modal:not(.loading) #m-memory-block', state='visible')
                    assert page.locator('#m-memory-block').evaluate("el => el.previousElementSibling.id") == 'm-memory-remote-anchor'
                    page.locator('#modal .modal-x').click()
                    results['external_preset_controls'] = 'PASS'

                    page.locator('.preset-add').click()
                    page.wait_for_selector('#modal:not(.loading) #m-memory-block', state='visible')
                    expect(page.locator('#m-memory-mode')).to_have_value('')
                    page.locator('#modal .modal-x').click()
                    results['new_preset_controls'] = 'PASS'
                    page.reload(wait_until='domcontentloaded')
                    page.locator('summary:has([data-i18n="settings.section_title"])').click()
                    expect(page.locator('#memory-system')).to_have_value('hybrid')
                    page.locator('#memory-system').select_option('classic')
                    page.wait_for_function("async () => (await (await fetch('/api/embeddings')).json()).defaults.enabled === false")
                    results['general_persistence_and_classic_switch'] = 'PASS'
                    context.close()
                    browser.close()
                assert not errors, errors
                results['javascript_errors'] = errors
            finally:
                process.terminate()
                process.wait(timeout=15)
    args.report.write_text(json.dumps(results, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    print(json.dumps(results, ensure_ascii=False))


if __name__ == '__main__':
    main()
