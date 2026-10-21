#!/usr/bin/env python3
"""Opt-in CPU end-to-end proof: import, native controls, embeddings, folder move.

Run against the portable build, an official CPU llama-server and the text GGUF.
All fixtures are created in a new temporary folder. No existing Ajean is used.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--app', required=True, type=Path)
    ap.add_argument('--llama', required=True, type=Path)
    ap.add_argument('--model', required=True, type=Path)
    ap.add_argument('--report', required=True, type=Path)
    args = ap.parse_args()
    work = Path(tempfile.mkdtemp(prefix='ajean-portable-proof-'))
    before, after, source = work/'before', work/'after', work/'source'
    for d in [before, source/'presets', source/'models', source/'backends']:
        d.mkdir(parents=True, exist_ok=True)
    suffix = '.exe' if os.name == 'nt' else ''
    app_name, llama_name = 'ajean-memoire-test'+suffix, 'llama-server'+suffix
    shutil.copy2(args.app, before/app_name)
    shutil.copytree(args.llama.parent, source/'backends'/'cpu')
    shutil.copytree(args.llama.parent, before/'encoder')
    source_model = source/'models'/'embeddinggemma-2-Q8_0.gguf'
    try:
        os.link(args.model, source_model)
    except OSError:
        shutil.copy2(args.model, source_model)
    preset = ('# NAME=CPU portable management proof\n'
              f'BIN={source/"backends"/"cpu"/llama_name}\n'
              'MODEL=embeddinggemma-2-Q8_0.gguf\nHOST=0.0.0.0\nPORT=8080\n'
              'NGL=0\nCTX=8192\nBATCH=8192\nUBATCH=8192\nTHREADS=2\n'
              'EXTRA_ARGS=--device none --embeddings --pooling mean --port=8080 --host 0.0.0.0\n')
    source_preset = source/'presets'/'CPU.env'
    source_preset.write_text(preset, encoding='utf-8')
    env = dict(os.environ, AJEAN_HOME=str(source))  # must be ignored by this build
    import_result = subprocess.run([str(before/app_name), 'import-presets', str(source/'presets')],
                                   cwd=work, env=env, capture_output=True, text=True, timeout=180)
    assert import_result.returncode == 0, import_result.stderr+import_result.stdout
    assert source_preset.read_text(encoding='utf-8') == preset
    imported = before/'data'/'presets'/'CPU.env'
    imported_text = imported.read_text(encoding='utf-8')
    assert str(source) not in imported_text and 'PORT=18080' in imported_text
    assert (before/'data'/'backends'/'cpu'/llama_name).is_file()
    notes = before/'data'/'memory'/'generale'
    notes.mkdir(parents=True, exist_ok=True)
    (notes/'resistance.md').write_text('# Tenue de la structure\nLes ancrages fixent les clusters inférieurs de la GeometryCollection. Les seuils de rupture définissent la résistance aux impacts. Le socle reste immobile pendant la simulation Chaos.\n', encoding='utf-8')
    (notes/'apparence.md').write_text('# Apparence\nLe Material Instance règle les textures, la couleur et la rugosité des murs.\n', encoding='utf-8')
    source.rename(work/'source-disabled')  # copied runtimes/weights must suffice

    class BorrowedEngine(BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(b'{"data":[{"id":"borrowed-engine-marker"}]}')
        def log_message(self, *_):
            pass

    borrowed = ThreadingHTTPServer(('127.0.0.1', 8080), BorrowedEngine)
    threading.Thread(target=borrowed.serve_forever, daemon=True).start()
    process, log = None, None
    base = 'http://127.0.0.1:18090'

    def api(path, body=None):
        req = Request(base+path, data=None if body is None else json.dumps(body).encode(),
                      headers={} if body is None else {'Content-Type': 'application/json'})
        with urlopen(req, timeout=20) as response:
            return json.load(response)

    def wait_for(fn, description, seconds=45):
        deadline = time.monotonic()+seconds
        while time.monotonic() < deadline:
            try:
                value = fn()
                if value:
                    return value
            except (OSError, URLError, HTTPError, KeyError):
                pass
            time.sleep(.1)
        try:
            detail = api('/api/embeddings')['job'].get('error', '')
        except Exception:
            detail = ''
        raise AssertionError(description+'; '+detail+'; logs in '+str(work))

    def start(folder):
        nonlocal process, log
        log = (work/(folder.name+'.log')).open('w')
        process = subprocess.Popen([str(folder/app_name), 'app'], cwd=work, env=env,
                                   stdout=log, stderr=subprocess.STDOUT)
        return wait_for(lambda: api('/api/embeddings'), 'UI did not start')

    def stop():
        nonlocal process, log
        if process is not None:
            process.terminate()
            process.wait(timeout=15)
            process = None
        if log:
            log.close()
            log = None

    def assert_borrowed_alive():
        with urlopen('http://127.0.0.1:8080/v1/models', timeout=2) as r:
            assert json.load(r)['data'][0]['id'] == 'borrowed-engine-marker'

    report = {'work': str(work), 'source_hidden': True}
    try:
        status = start(before)
        assert status['home'] == str(before/'data') and status['parallel']
        assert any(p['id'] == 'CPU' for p in api('/api/presets'))
        # Regression from Windows: choosing an internal runtime while MODEL
        # still holds an absolute source path must save a portable preset.
        edited = api('/api/preset?id=CPU')['content']
        edited += '\nMODEL="'+str(work/'source-disabled'/'models'/source_model.name)+'"\n'
        assert api('/api/preset/save', {'id': 'CPU', 'name': 'CPU', 'content': edited})['ok']
        assert str(work/'source-disabled') not in api('/api/preset?id=CPU')['content']
        (work/'source-disabled').rename(work/'source-unavailable')
        report['native_preset_external_model_save'] = 'PASS'
        assert api('/api/switch', {'id': 'CPU'})['ok']
        wait_for(lambda: api('/api/status')['health'], 'local CPU engine not ready')
        assert api('/api/restart', {})['ok']
        wait_for(lambda: api('/api/status')['health'], 'restart failed')
        assert api('/api/stop', {})['ok']
        assert not api('/api/status')['active']
        assert_borrowed_alive()
        report['native_presets_start_restart_stop'] = 'PASS'
        cfg = api('/api/embeddings')['config']
        cfg.update(enabled=True, threads=2)
        api('/api/embeddings', cfg)
        wait_for(lambda: (not api('/api/embeddings')['job']['running']) and api('/api/embeddings')['chunks'] > 0, 'initial index failed')
        query = {'query': 'Comment empêcher un bâtiment de s’écrouler après un choc ?', 'limit': 3}
        result = api('/api/embeddings/search', query)['result']
        assert result['mode'] == 'hybrid' and result['hits'][0]['File'] == 'resistance.md', result
        report['real_gguf_search'] = 'PASS'
        stop()
        before.rename(after)
        status = start(after)
        assert status['home'] == str(after/'data')
        assert str(after) in status['config']['bin'] and str(before) not in status['config']['bin']
        assert str(after) in api('/api/config')['BIN']
        assert api('/api/start', {})['ok']
        wait_for(lambda: api('/api/status')['health'], 'local engine failed after folder move')
        wait_for(lambda: (not api('/api/embeddings')['job']['running']) and api('/api/embeddings')['chunks'] > 0, 'index failed after folder move')
        result = api('/api/embeddings/search', query)['result']
        assert result['mode'] == 'hybrid' and result['hits'][0]['File'] == 'resistance.md', result
        assert api('/api/stop', {})['ok']
        assert_borrowed_alive()
        report['folder_move_engine_and_embeddings'] = 'PASS'
        report['borrowed_port_8080_unchanged'] = 'PASS'
        for path in ['/api/update/apply', '/api/network', '/api/link/start']:
            try:
                api(path, {})
                raise AssertionError(path+' unexpectedly allowed')
            except HTTPError as exc:
                assert exc.code == 403
        report['global_controls_blocked'] = 'PASS'
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2, ensure_ascii=False)+'\n', encoding='utf-8')
        print(json.dumps(report, ensure_ascii=False), flush=True)
    finally:
        stop()
        borrowed.shutdown()
        borrowed.server_close()


if __name__ == '__main__':
    main()
