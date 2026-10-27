# October 27 security candidate validation

This supplements the historical October 8 evidence; it does not rewrite that
release's results. Candidate toolchains: Go 1.26.10, Bun 1.3.6, Node 26.5.1, with
generated tracing/resource controls also run on Forum Node 24.18.1. Pulumi Go
pkg/SDK, engine and generated Node SDK are coordinated at 3.272.0.

Local macOS arm64 checks passed:

- `scripts/fork-build --allow-dirty`: actual CLI and embedded platform build.
- `go vet ./...` and `go test ./...` with the pinned Go toolchain.
- `bun run typecheck:platform`; platform tests: 111 passed, 15 existing todo.
- `scripts/fork-go-security.test.py`: four controls using the real govulncheck
  1.9.0 converter. The captured reachable GO-2026-6809 report fails with exit 3
  despite a successful JSON-producing scan. Clean, malformed and failed-scan
  controls distinguish success from errors.
- `scripts/fork-go-security`: zero reachable or imported-package vulnerabilities
  in the candidate. One module-only finding is reported without a reachable
  call path; it is not a blanket clean-module claim.
- `scripts/fork-integration-check`: clean production Go-generated install,
  frozen-lock rejection/recovery, checksum-verified real 3.272.0 engine and
  deliberately conflicting language-host preview control, then all 15 installed
  tracing/baggage/resource/secret tests. Local backend only; no cloud resources.
- `scripts/fork-platform-validate.cjs`: exact generated inventory recorded;
  all 15 tests passed and Bun audit returned zero advisory records.
- The same installed platform tests passed on Node 24.18.1.
- Publishing guards and offline installer overwrite/upgrade/failure controls.

Pulumi upstream tests were also run against the downloaded 3.272.0 modules:
`TestSerializedOutputValueSecretsRedacted`, `TestRedactSecretsOutputValues`,
`TestIsSecretValue`, `TestImportCheckFailureRedactsSecrets`,
`TestInvalidInputDiagnosticsRedactSecrets`, and `TestValidationFailures` passed.
These cover nested secret input diagnostics, import/check/construct paths,
serialized secret outputs, public-value controls and explicit `--show-secrets`.
Additional upstream test dependencies were resolved in a disposable copy of the
Go module files, leaving the fork's production graph unchanged. All markers are
synthetic; no actual Forum secret disclosure was established.

The clean committed artifact build, repeat-byte comparison and native CI results
are recorded in the update PR after its final commit. Do not infer those from the
dirty review build above. Linux/macOS and Node 24 CI, review and exact merged-source
artifacts remain required before publication. No release, live SST deployment,
disposable S3/SSM rehearsal or production rollout was performed.

See [upstream assessment and remaining divergence](../fork-upstream-status.md).
