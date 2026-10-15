#!/usr/bin/env python3
"""Offline acceptance checks for immutable, explicitly invoked installations."""
import json
import os
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
    manifest.write_text(json.dumps({'version': '4.17.2+forum.first', 'distributable': True}))
    first = root / 'installs' / 'first version'
    subprocess.run([str(archive / 'install'), str(first)], check=True)
    result = subprocess.check_output([str(first / 'sst-forum'), 'version'], text=True)
    assert result == 'true:version\n', result
    assert (first / 'LICENSE').read_text() == 'preserved license\n'
    assert subprocess.run([str(archive / 'install'), str(first)], capture_output=True).returncode != 0
    manifest.write_text(json.dumps({'version': '4.17.2+forum.second', 'distributable': True}))
    second = root / 'installs' / 'second version'
    subprocess.run([str(archive / 'install'), str(second)], check=True)
    assert json.loads((first / 'manifest.json').read_text())['version'] == '4.17.2+forum.first'
    assert json.loads((second / 'manifest.json').read_text())['version'] == '4.17.2+forum.second'
    # Inject filesystem failures in the installer's Python subprocess. Neither a
    # partial copy nor a failed wrapper write may reserve an immutable version.
    hooks = root / 'fault hooks'
    hooks.mkdir()
    (hooks / 'sitecustomize.py').write_text('''
import os, pathlib, shutil
original_copy = shutil.copy2
original_write = pathlib.Path.write_text
def copy(src, dst, *args, **kwargs):
    result = original_copy(src, dst, *args, **kwargs)
    if os.environ.get('INSTALL_FAULT') == 'copy':
        raise OSError('injected partial copy failure')
    return result
def write(self, *args, **kwargs):
    if os.environ.get('INSTALL_FAULT') == 'wrapper' and self.name == 'sst-forum':
        raise OSError('injected wrapper write failure')
    return original_write(self, *args, **kwargs)
shutil.copy2 = copy
pathlib.Path.write_text = write
''')
    for failure in ('copy', 'wrapper'):
        dest = root / 'installs' / ('retry-' + failure)
        env = dict(os.environ, PYTHONPATH=str(hooks), INSTALL_FAULT=failure)
        result = subprocess.run([str(archive / 'install'), str(dest)], env=env, capture_output=True, text=True)
        assert result.returncode != 0 and 'injected' in result.stderr, result
        assert not dest.exists(), 'failed installation left a partial destination'
        assert not dest.with_name(dest.name + '.install-lock').exists()
        assert not list(dest.parent.glob('.' + dest.name + '.install-*'))
        subprocess.run([str(archive / 'install'), str(dest)], check=True)
        assert subprocess.check_output([str(dest / 'sst-forum'), 'version'], text=True) == 'true:version\n'
    empty = root / 'installs' / 'existing empty'
    empty.mkdir()
    assert subprocess.run([str(archive / 'install'), str(empty)], capture_output=True).returncode != 0
    assert list(empty.iterdir()) == []
    manifest.write_text(json.dumps({'version': '4.17.2+forum.first-dirty', 'distributable': False}))
    dirty = root / 'installs' / 'dirty'
    assert subprocess.run([str(archive / 'install'), str(dirty)], capture_output=True).returncode != 0
    assert not dirty.exists()
    assert subprocess.run([str(archive / 'install'), 'relative-path'], capture_output=True).returncode != 0
print('Installer checks passed: wrapper identity, preserved license, immutable reinstall, side-by-side upgrade, dirty rejection')
