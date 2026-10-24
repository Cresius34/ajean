"""Run the pinned upstream setup inside Ajean's portable installation only.

No START-HERE/venv launcher, cross-install scan, system installer or build fallback.
The engine and model preparation itself remain upstream's implementation.
"""
from __future__ import annotations
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sys


def configure(upstream, manifest: dict, root: Path, data: Path):
    root, data = root.resolve(), data.resolve()
    if not data.is_relative_to(root.parent):
        raise ValueError('The model folder must stay inside this Ajean MoE folder.')
    data.mkdir(parents=True, exist_ok=True)
    upstream.settings_path = lambda: root/'ajean-settings.json'
    upstream.other_installs = lambda _settings: []
    # The upstream helper normally merges/moves data from other installations.
    # This proof owns one explicit folder, so there is nothing to discover/move.
    # 0.2.4 calls it as data_folder(requested, move, yes, set_default).
    upstream.data_folder = lambda requested, *_rest: (data, [])
    upstream.prebuilt_bases = lambda requested: [requested]
    upstream.hf_unpinned = lambda url: url
    # --yes upstream may choose two NVIDIA cards. This prototype offers one
    # card per model, consistently with Ajean's displayed hardware selection.
    upstream.choose_gpus = lambda _args, found: [
        sorted((g for g in found if upstream.gpu_problem(g) is None),
               key=lambda g: (-round(g['vram_gb']), g['index']))[0]['index']
    ]

    def no_build(*_args, **_kwargs):
        raise RuntimeError('The verified prebuilt engine cannot run on this GPU. '
                           'No system compiler or SDK will be installed by this portable prototype.')
    upstream.build_engine = no_build
    upstream.build_engine_hip = no_build

    assets = {a['name']: a for a in manifest['engines'].values()}
    def verify(archive: Path, asset: str, _base: str):
        expected = assets.get(asset)
        if expected is None:
            raise RuntimeError('Engine archive not in the pinned Ajean manifest: '+asset)
        with Path(archive).open('rb') as stream:
            actual = hashlib.file_digest(stream, 'sha256').hexdigest()
        if actual != expected['sha256'] or Path(archive).stat().st_size != expected['size']:
            raise RuntimeError('Engine archive hash/size mismatch: '+asset)
    upstream.verify_engine_archive = verify
    upstream.PREBUILT_URL = 'https://github.com/Niko1221/Strata/releases/download/v'+manifest['version']+'/'
    # setup's CUDA/HIP runtime wheels are installed into the embedded Python
    # (sys.prefix), with all pip/HF caches kept under the portable root.
    os.environ['PIP_CACHE_DIR'] = str(root.parent/'cache'/'pip')
    os.environ['PIP_CONFIG_FILE'] = os.devnull
    os.environ['PYTHONNOUSERSITE'] = '1'
    os.environ['HF_HOME'] = str(root.parent/'cache'/'huggingface')
    os.environ['STRATA_SOURCE'] = 'huggingface'


def main():
    ap = argparse.ArgumentParser(add_help=False)
    ap.add_argument('--ajean-manifest', required=True, type=Path)
    opts, args = ap.parse_known_args()
    root = Path(__file__).resolve().parent
    manifest = json.loads(opts.ajean_manifest.read_text(encoding='utf-8-sig'))
    if '--data-dir' not in args:
        raise ValueError('An explicit portable --data-dir is required.')
    data = Path(args[args.index('--data-dir')+1])
    spec = importlib.util.spec_from_file_location('ajean_strata_setup', root/'setup.py')
    upstream = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = upstream
    spec.loader.exec_module(upstream)
    configure(upstream, manifest, root, data)
    sys.argv = [str(root/'setup.py'), *args]
    return upstream.main()


if __name__ == '__main__':
    sys.exit(main())
