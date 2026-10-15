#!/usr/bin/env python3
"""Deterministic archive metadata and retained third-party license texts."""
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile

stage, version, commit, target_os, target_arch = sys.argv[1:]
stage = Path(stage)
epoch = int(subprocess.check_output(['git', 'show', '-s', '--format=%ct', 'HEAD']))
metadata = {
    'version': version, 'commit': commit, 'source': 'https://github.com/Remark-Inc/sst',
    'target': f'{target_os}/{target_arch}', 'distributable': not version.endswith('-dirty'),
    'go': subprocess.check_output(['go', 'version'], text=True).strip(),
    'bun': subprocess.check_output(['bun', '--version'], text=True).strip(),
    'node': subprocess.check_output(['node', '--version'], text=True).strip(),
    'platformPackageSHA256': hashlib.sha256(Path('platform/package.json').read_bytes()).hexdigest(),
    'platformLockSHA256': hashlib.sha256(Path('platform/bun.lockb').read_bytes()).hexdigest(),
}
(stage / 'manifest.json').write_text(json.dumps(metadata, indent=2, sort_keys=True) + '\n')
notices = stage / 'third-party-notices'
notices.mkdir()
# Preserve licenses/notices for installed JS modules, including bundled helpers.
seen = set()
for root in (Path('node_modules'), Path('platform/node_modules')):
    for folder, dirs, files in os.walk(root, followlinks=True):
        dirs.sort()
        files.sort()
        real = os.path.realpath(folder)
        if real in seen:
            dirs[:] = []
            continue
        seen.add(real)
        for name in files:
            if name.lower().startswith(('license', 'copying', 'notice')):
                src = Path(folder) / name
                if src.is_file():
                    dest = notices / 'js' / src
                    dest.parent.mkdir(parents=True, exist_ok=True)
                    dest.write_bytes(src.read_bytes())
# Retain direct and transitive Go module notices without shipping module source.
raw = subprocess.check_output(['go', 'list', '-mod=readonly', '-m', '-json', 'all'], text=True)
decoder = json.JSONDecoder()
while raw.strip():
    module, offset = decoder.raw_decode(raw.lstrip())
    raw = raw.lstrip()[offset:]
    directory = module.get('Dir')
    if not directory or module.get('Main'):
        continue
    for src in Path(directory).iterdir():
        if src.is_file() and src.name.lower().startswith(('license', 'copying', 'notice')):
            dest = notices / 'go' / module['Path'] / src.name
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(src.read_bytes())
archive_name = f'sst-forum-{version}-{target_os}-{target_arch}.tar.gz'
archive_path = Path('dist/fork') / archive_name
# Explicit timestamps, ownership, modes and ordering avoid host filesystem metadata.
with archive_path.open('wb') as out, gzip.GzipFile(filename='', fileobj=out, mode='wb', mtime=epoch) as gz:
    with tarfile.open(fileobj=gz, mode='w', format=tarfile.PAX_FORMAT) as tar:
        for path in sorted(stage.rglob('*')):
            if not path.is_file():
                continue
            data = path.read_bytes()
            info = tarfile.TarInfo(str(path.relative_to(stage)))
            info.size, info.mtime = len(data), epoch
            info.mode = 0o755 if path.name in ('sst', 'install') else 0o644
            tar.addfile(info, io.BytesIO(data))
digest = hashlib.sha256(archive_path.read_bytes()).hexdigest()
Path('dist/fork/checksums.txt').write_text(f'{digest}  {archive_name}\n')
Path('dist/fork/manifest.json').write_text((stage / 'manifest.json').read_text())
