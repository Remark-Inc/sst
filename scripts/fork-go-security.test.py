#!/usr/bin/env python3
"""Exercise the CI gate with recorded scan output and the real pinned converter."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class SecurityGate(unittest.TestCase):
    def run_gate(self, report, scan_exit=0):
        scanner = shutil.which('govulncheck')
        self.assertIsNotNone(scanner, 'Install the pinned govulncheck before running this test')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'input.json').write_text(report)
            wrapper = root / 'govulncheck'
            wrapper.write_text('#!/usr/bin/env bash\n'
                               'if [ "$1" = -json ]; then cat "$TEST_REPORT"; exit "$TEST_SCAN_EXIT"; fi\n'
                               'exec "$TEST_SCANNER" "$@"\n')
            wrapper.chmod(0o700)
            env = dict(os.environ, PATH=tmp + os.pathsep + os.environ['PATH'],
                       TEST_REPORT=str(root / 'input.json'), TEST_SCANNER=scanner,
                       TEST_SCAN_EXIT=str(scan_exit))
            result = subprocess.run(['bash', str(ROOT / 'scripts/fork-go-security'),
                                     str(root / 'evidence')], env=env, capture_output=True, text=True)
            self.assertEqual((root / 'evidence/govulncheck.json').read_text(), report)
            return result

    def test_reachable_finding_fails_even_when_json_scan_succeeds(self):
        report = (ROOT / 'scripts/testdata/govulncheck-reachable.json').read_text()
        result = self.run_gate(report)
        self.assertEqual(result.returncode, 3, result.stderr)
        self.assertIn('GO-2026-6809', result.stdout)

    def test_clean_symbol_scan_passes(self):
        result = self.run_gate(json.dumps({'config': {
            'protocol_version': 'v1.0.0', 'scanner_name': 'govulncheck',
            'scanner_version': 'v1.9.0', 'scan_level': 'symbol', 'scan_mode': 'source'}}) + '\n')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_scanner_failure_is_not_hidden_by_conversion(self):
        self.assertEqual(self.run_gate('', scan_exit=2).returncode, 2)

    def test_malformed_report_fails(self):
        self.assertNotEqual(self.run_gate('not JSON').returncode, 0)


if __name__ == '__main__':
    unittest.main()
