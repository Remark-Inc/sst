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
    'runtimePackageSHA256': hashlib.sha256(Path('platform/runtime/package.json').read_bytes()).hexdigest(),
    'runtimeLockSHA256': hashlib.sha256(Path('platform/runtime/bun.lockb').read_bytes()).hexdigest(),
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
# Collect only modules in the binaries we actually ship. Listing all modules
# and skipping uncached directories made notices depend on prior test runs.
modules = {}
for package, build_env in [
    ('./cmd/sst', os.environ.copy()),
    ('./platform/functions/bridge', {**os.environ, 'GOOS': 'linux', 'GOARCH': 'amd64'}),
]:
    raw = subprocess.check_output(['go', 'list', '-mod=readonly', '-deps', '-json', package],
                                  text=True, env=build_env)
    decoder = json.JSONDecoder()
    while raw.strip():
        entry, offset = decoder.raw_decode(raw.lstrip())
        raw = raw.lstrip()[offset:]
        module = entry.get('Module')
        if module and not module.get('Main'):
            modules[module['Path']] = module
for module in sorted(modules.values(), key=lambda item: item['Path']):
    directory = module.get('Dir')
    if not directory:
        raise RuntimeError(f"Compiled module {module['Path']} is missing its source directory")
    for src in Path(directory).iterdir():
        if src.is_file() and src.name.lower().startswith(('license', 'copying', 'notice')):
            dest = notices / 'go' / module['Path'] / src.name
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(src.read_bytes())
# Include the standard library/runtime license as well as third-party modules.
goroot = Path(subprocess.check_output(['go', 'env', 'GOROOT'], text=True).strip())
(notices / 'go-runtime').mkdir()
(notices / 'go-runtime' / 'LICENSE').write_bytes((goroot / 'LICENSE').read_bytes())
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

# Keep per-file diagnostics outside the archive for repeat-build comparisons.
files = {str(p.relative_to(stage)): hashlib.sha256(p.read_bytes()).hexdigest()
         for p in sorted(stage.rglob('*')) if p.is_file()}
Path('dist/fork/files.json').write_text(json.dumps(files, indent=2, sort_keys=True) + '\n')
