#!/usr/bin/env python3
"""Review-time generation of the runtime manifest/lock; builds only check them."""
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

root = Path(__file__).resolve().parent.parent
platform = root / 'platform'
runtime = platform / 'runtime'
manifest = json.loads((platform / 'package.json').read_text())
for name, version in manifest.pop('devDependencies', {}).items():
    if name.startswith('@types/'):
        manifest['dependencies'][name] = version

if sys.argv[1:] == ['--update']:
    if subprocess.check_output(['bun', '--version'], text=True).strip() != '1.3.6':
        raise SystemExit('Require Bun 1.3.6')
    runtime.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='sst-runtime-lock-') as tmp:
        stage = Path(tmp)
        (stage / 'package.json').write_text(json.dumps(manifest, indent=2) + '\n')
        seed = runtime / 'bun.lockb'
        shutil.copy2(seed if seed.exists() else platform / 'bun.lockb', stage / 'bun.lockb')
        subprocess.run(['bun', 'install', '--lockfile-only', '--ignore-scripts'], cwd=stage, check=True)
        for name in ('package.json', 'bun.lockb'):
            shutil.copy2(stage / name, runtime / name)
elif sys.argv[1:]:
    raise SystemExit('Usage: scripts/fork-runtime-lock.py [--update]')
else:
    if json.loads((runtime / 'package.json').read_text()) != manifest:
        raise SystemExit('Runtime manifest drift: update and review scripts/fork-runtime-lock.py --update')
    if not (runtime / 'bun.lockb').is_file():
        raise SystemExit('Missing reviewed runtime lock')
