#!/usr/bin/env node
// Run against an actual Go-generated .sst/platform installation. Never deploys.
// Findings remain a failing security gate; logs are retained even if tests fail.
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");
const { inventory } = require("./fork-platform-inventory.cjs");
const platform = fs.realpathSync(process.argv[2] || "work/generated-platform-final/.sst/platform");
const output = path.resolve(process.argv[3] || "work/fork-platform-validation/final");
fs.mkdirSync(output, { recursive: true });
const installed = inventory(platform);
fs.writeFileSync(path.join(output, "installed-inventory.json"), JSON.stringify(installed, null, 2) + "\n");
function run(name, executable, args, options = {}) {
  const result = spawnSync(executable, args, { encoding: "utf8", maxBuffer: 20 * 1024 * 1024,
    timeout: 120_000, ...options });
  fs.writeFileSync(path.join(output, `${name}.stdout`), result.stdout || "");
  fs.writeFileSync(path.join(output, `${name}.stderr`), result.stderr || result.error?.message || "");
  return { status: result.status, error: result.error?.message };
}
const tests = run("tests", process.execPath, ["--test", path.join(__dirname, "fork-platform-tests.cjs")],
  { env: { ...process.env, SST_FORK_PLATFORM_DIR: platform } });
const audit = run("audit", process.env.SST_TEST_BUN || "bun", ["audit", "--json"], { cwd: platform });
const report = { platform, output, tests, audit, auditAdvisoryRecordsBySeverity: {},
  auditPackageGroups: [], uniqueAdvisoryURLs: [] };
try {
  const findings = JSON.parse(fs.readFileSync(path.join(output, "audit.stdout"), "utf8"));
  for (const [name, advisories] of Object.entries(findings)) {
    if (!Array.isArray(advisories)) throw new Error("Unexpected Bun audit response shape");
    report.auditPackageGroups.push(name);
    for (const advisory of advisories) {
      report.auditAdvisoryRecordsBySeverity[advisory.severity] =
        (report.auditAdvisoryRecordsBySeverity[advisory.severity] || 0) + 1;
      report.uniqueAdvisoryURLs.push(advisory.url);
    }
  }
  report.uniqueAdvisoryURLs = [...new Set(report.uniqueAdvisoryURLs)].sort();
} catch (error) { report.auditParseError = error.message; }
fs.writeFileSync(path.join(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
console.log(JSON.stringify(report, null, 2));
process.exitCode = tests.status === 0 && audit.status === 0 && !report.auditParseError ? 0 : 1;
