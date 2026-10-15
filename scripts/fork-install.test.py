#!/usr/bin/env python3
"""Offline acceptance checks for immutable, explicitly invoked installations."""
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

installer = Path(__file__).resolve().with_name('fork-install')
with tempfile.TemporaryDirectory(prefix='sst forum installer ') as temp:
    root = Path(temp)
    archive = root / 'archive'
    archive.mkdir()
    shutil.copy2(installer, archive / 'install')
    (archive / 'sst').write_text('#!/usr/bin/env bash\nprintf "%s:%s\\n" "$SST_SKIP_LOCAL" "$1"\n')
    (archive / 'sst').chmod(0o755)
    (archive / 'LICENSE').write_text('preserved license\n')
    manifest = archive / 'manifest.json'
    manifest.write_text(json.dumps({'version': '4.17.2-forum.first', 'distributable': True}))
    first = root / 'installs' / 'first version'
    subprocess.run([str(archive / 'install'), str(first)], check=True)
    result = subprocess.check_output([str(first / 'sst-forum'), 'version'], text=True)
    assert result == 'true:version\n', result
    assert (first / 'LICENSE').read_text() == 'preserved license\n'
    assert subprocess.run([str(archive / 'install'), str(first)], capture_output=True).returncode != 0
    manifest.write_text(json.dumps({'version': '4.17.2-forum.second', 'distributable': True}))
    second = root / 'installs' / 'second version'
    subprocess.run([str(archive / 'install'), str(second)], check=True)
    assert json.loads((first / 'manifest.json').read_text())['version'] == '4.17.2-forum.first'
    assert json.loads((second / 'manifest.json').read_text())['version'] == '4.17.2-forum.second'
    manifest.write_text(json.dumps({'version': '4.17.2-forum.first-dirty', 'distributable': False}))
    dirty = root / 'installs' / 'dirty'
    assert subprocess.run([str(archive / 'install'), str(dirty)], capture_output=True).returncode != 0
    assert not dirty.exists()
    assert subprocess.run([str(archive / 'install'), 'relative-path'], capture_output=True).returncode != 0
print('Installer checks passed: wrapper identity, preserved license, immutable reinstall, side-by-side upgrade, dirty rejection')
