# Fork validation — 2026-10-15

This is local macOS arm64 evidence for the source/lock hashes in
`source-hashes.json`, using Go 1.26.9, Bun 1.3.6 and Node 26.5.1.
It is not deployment approval. Provider package versions remain unchanged.

## Passing local checks

- `go build ./...`, `go vet ./...`, `go test -vet=all ./...`;
  `go mod verify`, `go mod tidy -diff` (no changes).
- Embedded platform build with `SST_SKIP_BRIDGE_IMAGE=true`; platform typecheck.
- Existing platform component suite plus retry regressions: 111 passed, 15 todo;
  11 passed files / 1 skipped, no unhandled errors. Vitest 4 reports existing todo
  tests separately; they are not claimed as executed coverage.
  Vitest uses processes because Pulumi closure serialization enumerates Node
  builtins, including `trace_events` which is unavailable in worker threads.
  Stale existing component fixtures/expectations were corrected without changing
  component implementation.
- Root and standalone platform frozen installs; generated-platform typecheck.
- Four local AWS client regressions preserve retry quotas, 10,000-attempt bound,
  capped backoff and permanent-error handling, and verify signed S3 XML requests
  with a mocked request handler (no sockets or credentials).
- Actual generated installation through `Project.CopyPlatform`,
  `writePackageJson`, and `fetchDeps` (`TestGeneratedPlatformInstall`).
- 15 generated-install checks: runtime Vite loading without app-local Vite; mocked resource/secret output propagation;
  installed SDK/runtime inventory; valid trace parent reaching a local gRPC
  monitor; disabled and malformed trace controls; malformed/oversized baggage
  and valid controls; tracing patch idempotence and changed-version/source refusal.
- Real Pulumi 3.267.0 local-backend **preview only**, no provider resources:
  conflicting `pulumi-language-nodejs` first on PATH is ignored; negative control
  with ambient selection enabled invokes it and fails. Versioned installation,
  mixed/missing hosts, repair/checksum refusal, symlink/relative overrides, lock
  cancellation and generated-platform SDK/reinstall checks also pass.
- Publishing guards and offline installer side-by-side/overwrite/dirty-build tests.
  Forced copy/wrapper failures leave no partial destination and permit retry.
  Pulumi promotion tests cover backup/promotion/rollback failures, preserve old
  and validated replacement files, and exercise retry after a transient failure.

No local `sst deploy`, AWS mutation, release or package publication was run.
Linux CI and final committed archive reproducibility/install results must be read
from the PR checks and artifact manifest; this document does not pre-claim them.

## Exact installed SDK/tracing inventory

`installed-inventory.json` lists every installed package path/version/license and
manifest hash, plus the generated manifest/lock hashes and direct tracing resolution.
The generated tree has **one Pulumi SDK, 3.267.0**, with grpc-js **1.14.6** and no
Jaeger packages. OpenTelemetry API is 1.9.1; the direct stable tracing family is
2.12.0. Experimental OTLP/instrumentation packages are 0.220.0 and retain nested
patched core/resources 2.9.0. Tests inspect all installed core copies.

An initial candidate with only the direct SDK upgrade retained nested Pulumi 3.98.0
through the old TLS provider lock, including obsolete tracing packages. Exact
SDK overrides in the **embedded platform** and regenerated standalone lock remove
that copy without changing provider versions. Root overrides alone are insufficient.

Pulumi 3.267.0's actual `runtime/settings.js` passes a carrier with a `set()` method
to OpenTelemetry's default object-property setter, silently losing outgoing gRPC
trace metadata. The embedded, exact-version/source-checked postinstall patch adds
an explicit setter. It preserves the SDK license. SST reapplies it explicitly if
package-manager lifecycle scripts are disabled, and only marks installation
complete afterward. Review/remove the patch on every SDK update. The unpatched
positive test failed while disabled/malformed controls passed; patched tests pass.

## Generated-platform security gate: PASS

Current Bun audit of the actual clean generated installation returns **zero
findings** (`{}`, exit 0). The standalone platform build/test installation also
returns zero findings, including its development tools. The package inventory and
lock hashes are retained alongside the scan. Registry audit does not establish
absence of every vulnerability or replace dependency review.

Remediation keeps all Pulumi provider versions unchanged: AWS JavaScript clients
move within v3, obsolete AWS retry middleware moves to its maintained Smithy
implementation with behavior tests, Glob/esbuild and Vitest/Vite are updated,
and compatible transitive ranges are refreshed in the standalone lock. Generated
platforms retain TypeScript declaration packages and runtime Vite but omit repository test tools.
SST rejects npm fallback and non-pinned Bun installers so unvalidated lockless
resolution cannot silently replace the reviewed generated tree. A separate embedded
runtime manifest/lock is installed with `--frozen-lockfile`; an actual generated
install regression changes a dependency requirement and proves installation fails
without a completion receipt. Provider overrides outside the reviewed versions
are rejected. Linux CI also tests Forum's Node 24 runtime; consult its current
check result rather than assuming the local Node 26 result establishes compatibility. CI fails on any
new generated-platform finding and preserves the raw evidence.

Scope: this scan covers the generated deployment platform and standalone platform
build/test dependencies. A separate audit of the entire upstream monorepo still
reports legacy website/SDK workspace dependencies; those packages are not the
runtime tree this fork ships or a claim of being remediated here. Root lock
changes are limited to the platform update. The original SST runtime npm package
is not published by this fork.

Go `govulncheck` v1.9.0 (database last modified 2026-10-08) has **no reachable
vulnerable package/symbol** after targeted x/net, go-git, Lambda SDK and chi updates.
It still reports module-level GO-2026-5932 for unmaintained x/crypto/openpgp with no
fixed version; see `go-vulnerability-summary.json`. This is static analysis, not
a guarantee of absence of vulnerabilities.

## Reproduce

With the pinned toolchain, build the embedded platform, then run:

```sh
./scripts/fork-integration-check /absolute/fresh/evidence-directory
node scripts/fork-platform-validate.cjs \
  /absolute/fresh/evidence-directory/generated-platform/.sst/platform \
  /absolute/security-evidence-directory
```

The validation command exits nonzero on any security findings while
preserving tests, exact inventory and raw audit data; the recorded run exits zero. Never replace it with a
workspace-only scan. `docs/fork-maintenance.md` records required unrun disposable
and existing-stack lifecycle/rollback acceptance, the Forum six-check suite,
browser flows, memory/performance and CloudWatch inspection. Forum PR 2250 remains
draft; the Adam workaround remains in staging, production and smoke workflows.
