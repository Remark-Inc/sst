# Remark SST fork maintenance

This is a temporary, narrowly scoped fork for Forum AI. Source: `anomalyco/sst`,
base **v4.17.2**, commit `e49ac31411f63ab4ed0d802b7e15d95f6b827e47`.
The application runtime SDK package remains upstream `sst`, pinned to the exact
base release **4.17.2** in every workspace package. Lambda bundling accepts that
base SDK with the full-commit fork build metadata; older SDK patches, prereleases,
and unrelated metadata still fail the version check. Invoking the npm CLI would
bypass this fork. Invoke the versioned fork executable explicitly.

## Patch inventory and boundaries

- Coordinate Pulumi Go SDK/pkg, engine and Node SDK at **3.272.0**, including the
  secrets provider context API migration and generated-platform dependency policy.
  Provider package versions are intentionally unchanged. AWS JavaScript clients
  and build/test dependencies receive targeted security updates; retry semantics
  are preserved by mocked compatibility tests. Generated installations omit
  repository test tools while retaining TypeScript declarations and runtime Vite
  for Remix config loading.
- Apply a version/source-checked Pulumi Node tracing fix for gRPC metadata propagation,
  with idempotence, malformed-input/control and mocked secret/resource tests.
- Select the language host deterministically from the matching engine installation;
  test a conflicting executable earlier on `PATH` and reinstall/upgrade behavior.
- Patch the Pulumi 3.272.0 outgoing gRPC tracing setter with an embedded,
  version/source-checked installer; pin a single SDK and compatible grpc-js in the
  generated platform. See [validation evidence](fork-validation/README.md).
- Build the actual Go CLI with its embedded platform, distinct commit identity,
  no npm-local CLI forwarding and no upstream self-upgrade.
- Disable upstream npm, Rust, Python, GitHub release, Homebrew, AUR and bridge image
  publishing. Existing upstream release/snapshot scripts fail closed. CI has only
  read permissions and retains review artifacts; it cannot release or deploy.

References, not cherry-pick instructions: [official PR 6995](https://github.com/anomalyco/sst/pull/6995)
(3.261.0 and `secrets.Provider.OfType` context) and
[community PR 18](https://github.com/sst-community/sst/pull/18) (3.267.0 and reported
AWS deployment/rollback). Their deployment evidence does not validate this fork.

## Supported builds and artifact identity

`./scripts/fork-toolchain.env` pins Go **1.26.10**, Bun **1.3.6** and Node **26.5.1**.
Use these exact versions on Linux CI and macOS. Generated installation rejects
npm fallback (`NO_BUN` / `SST_NO_BUN`) and a Bun executable with another version.
Node 22 or newer is required by the tracing patch; validated runtime targets are
Node 26.5.1 and Forum's Node 24 line. The separate Linux Node 24 CI job records the
resolved patch version and runs the clean frozen install, real engine/host
regression, tracing/resource/secret tests and generated dependency audit.
Existing Python packaging tests use
uv 0.9.28; CI installs that tested version before running the Go suite. Go module resolution is readonly;
Bun installations use committed frozen root/platform/runtime lockfiles. Python 3 is used
only to package archives and copy files. Docker is unnecessary for the CLI build.
The platform builder fails on `DOCKER_PUSH=true` and skips bridge image builds by
default; an explicit local build uses a local image name and cannot push.

The native build command is:

```sh
./scripts/fork-check-safety
./scripts/fork-build
```

The tree must be committed for distributable output. `--allow-dirty` produces a
clearly marked local review build that the installer refuses. Supported targets:
Linux amd64/arm64 and macOS amd64/arm64; CI validates native Linux amd64 and macOS
arm64. Set `GOOS`/`GOARCH` to cross-compile, but execute regression tests natively
before approving an additional target. A build is never a deployment approval.

Outputs in `dist/fork/` include `sst-forum-4.17.2+forum.<full-commit>-<os>-<arch>.tar.gz`,
`manifest.json` and `checksums.txt`. The archive contains the actual CLI, embedded
platform, dependency lockfiles, original MIT license and third-party notices.
Fork identity uses SemVer build metadata, so base-version constraints such as
`app.version: "4.17.2"` and `">= 4.17.2"` accept it without opting into prereleases.
The archive fixes timestamps, ordering, modes and ownership; CI builds twice and
compares checksums on each OS. Record compiler, OS/architecture, manifest and
checksums with the review. Equal cross-OS/cross-architecture bytes are not expected.
Checksums prove integrity only when acquired through a trusted review channel.
CI artifacts expire after 30 days and are review evidence, not a permanent release
channel. A future publication needs separate approval, a Remark-owned destination,
and a policy preventing replacement of an existing commit/version artifact.

After downloading the reviewed archive and its trusted checksum, verify before
extracting (`shasum -a 256 -c checksums.txt` on macOS, `sha256sum -c checksums.txt` on
Linux). Extract to an empty directory and run its included installer:

```sh
./install /absolute/path/tools/sst-forum/4.17.2+forum.FULL_COMMIT
/absolute/path/tools/sst-forum/4.17.2+forum.FULL_COMMIT/sst-forum version
```

The installer refuses an existing destination; upgrades install side by side.
Pin CI and developer commands to that absolute wrapper path. The wrapper exports
`SST_SKIP_LOCAL=true`; the fork itself also prevents npm-local CLI forwarding.
Never use `npx sst`, `bunx sst`, an unqualified `sst`, `latest`, or `sst upgrade` to
select this fork. `SST_PULUMI_PATH` selects a complete engine installation; this fork validates
its engine and sibling Node host versions. It is not a Node SDK fix.
Root workspace overrides also do not remediate `.sst/platform` generated installs.

`platform/runtime/package.json` and `platform/runtime/bun.lockb` describe the
reviewed deployment dependencies, excluding test tools. Regenerate them only in a
reviewed dependency update with `python3 scripts/fork-runtime-lock.py --update`;
builds check manifest consistency and actual generated installs use
`bun install --frozen-lockfile`. An unexpected dependency change fails installation
and removes its completion receipt. Provider overrides outside the embedded
package versions are rejected with an explicit update instruction. Forum's
current default AWS configuration is supported; new providers or provider version
overrides require a fork dependency/lock update and the same security checks.

Offline archive installation stages files beside the destination and promotes
them only after the wrapper is complete; failures clean staging and permit retry.
Pulumi repair retains the previous directory until promotion succeeds, restores
it on promotion failure, and reports retained recovery paths if rollback fails.

The ECS Task dev image remains upstream `ghcr.io/anomalyco/sst/bridge-task:latest`
in the component source. Building this archive does **not** create or publish a
fork bridge image or pin that deployment dependency. ECS Task dev use requires a
separately reviewed immutable image plan; it is outside this fork acceptance scope.

## Required update evidence

Before approval, record commands, tool versions, exact commit and results:

1. Go build, vet and tests; platform build, typecheck and mocked resource/secret
   output tests. Preserve existing focused tests and explain failures.
2. Install the generated platform in a clean directory through the same package
   generation path used by SST. Record the exact installed Pulumi SDK and all
   tracing/OpenTelemetry dependencies, dependency graph and current security scan.
   A root workspace scan alone is insufficient. Document residual findings. The
   CI generated-platform audit and separate pinned `govulncheck` job are failing
   security gates when findings are present. The Go gate converts its JSON report
   with `govulncheck -mode=convert`; JSON output alone returns success even with
   reachable findings. A recorded reachable finding and clean/error controls test
   the actual converter and shell gate.
   Build/reproducibility artifacts are retained before the JS security gate runs.
3. Prove trace parent/baggage propagation, malformed baggage rejection and valid
   controls where relevant to the selected tracing implementation. Include secret
   output serialization and masking cases without printing real secrets.
4. Engine/language host regression with an intentionally conflicting executable
   earlier on `PATH`, including reuse, upgrade/reinstall and an explicit engine
   override. Confirm generated-platform Node SDK and engine versions agree.
5. Run `scripts/fork-integration-check /absolute/fresh/evidence-directory` for the
   clean generated installation, real checksum-verified Pulumi local-backend host
   regression and tracing/mock suite. This uses a preview with no cloud resources.
6. Build archive twice, compare checksums, install at a new versioned path, run
   `version`, and prove an existing installation cannot be overwritten. Confirm
   publishing guards and that the embedded platform is from the reviewed source.

## Integration acceptance: NOT RUN / deployment approval required

No local `sst deploy`, live infrastructure mutation, release or Forum workflow
change is authorized by this implementation. The following is a future coordinated
acceptance plan, not a claim of passing evidence:

| Gate | Required evidence |
| --- | --- |
| Disposable stack | A reviewed minimal config and test account/stage; create, update, no-change, delete; expected resources/outputs and no leaks. Include secret outputs and trace/baggage controls. |
| Existing-stack upgrade | Backup/export state and record engine/provider versions; run a no-change preview, upgrade, update and no-change with the fork; compare outputs/resource identities and state compatibility. |
| Rollback rehearsal | Restore prior versioned executable plus prior generated platform dependencies; preview first, then approved rollback only if state/schema compatibility permits. Demonstrate recovery in disposable state before any shared stack. |
| Forum staging six-check suite | Run the current `scripts/staging-smoke/run.ts` suite: browser setup; browser navigation and asset smoke; landing form writes and HQ verification; labeling create/browser submit/results/archive (including tree); response-gen one-prompt run; neutrality judge one-response batch. Record each status and evidence. PR 2250 is merged in Forum; preserve it and validate the separate generated graph. |
| Memory/performance | Compare peak RSS, deployment duration, process count and no-change duration against the same baseline stack; investigate significant regressions before rollout. |
| Browser acceptance | Verify HQ, landing, Sanity content, response flow and judge/accounting using the approved staging test accounts and canonical suite. Capture expected outputs without exposing credentials. |
| Runtime errors | Query the agreed CloudWatch groups/window for new errors, failed invocations, unhandled rejections and tracing/baggage failures. Correlate deployment and test timestamps. |

The current Forum workflow command is
`pnpm sst shell --stage staging -- pnpm exec tsx scripts/staging-smoke/run.ts`.
A separately reviewed Forum change should substitute the absolute approved fork
CLI for `pnpm sst`, keeping the same suite. Do not run it under this task's
local-only authorization.

Before switching Forum, inventory all CLI consumers and route each through one
absolute, checksum-verified fork executable variable. Required call sites include:

- Native CodeBuild deployment, refresh and migration commands in
  `.github/scripts/native-deploy.sh`.
- Staging/production GitHub and CodeBuild deployment workflows and the standalone
  smoke workflow, including their shell commands.
- Nested helpers `read-staging-outputs.sh`, `verify-staging.sh`, and
  `wait-for-ecs.mjs`; the latter directly spawns `pnpm sst`.

Search Forum scripts, workflows and package scripts for `pnpm sst`, `npx sst`,
`bunx sst`, and direct/spawned SST commands. Verify the actual executable/version
used by each deployment and validation path, so nested smoke/refresh helpers
cannot silently invoke upstream SST. These substitutions belong in the Forum
integration PR, alongside an approved permanent artifact channel; this fork PR
does not perform them or publish a release.

Forum Adam workaround PR 392 currently removes `/usr/local/bin/pulumi-language-nodejs`
in staging/production/smoke. **Retain it.** The conflicting-host regression is a
prerequisite to proposing removal; actual Forum staging acceptance and a separate
reviewed Forum change are still required. This task does not modify Forum AI.

See [current upstream assessment](fork-upstream-status.md) for release dates,
remaining divergence and the security update awaiting immutable publication.

## Upkeep, conflicts and exit

The existing “Maintain Forum SST fork” weekly heartbeat (Mondays at 09:00
America/New_York) may inspect upstream release notes, security reports, the
relevant dependency changes and the patch inventory, and prepare reviewed updates.
It must never auto-merge, release, publish, deploy or contact upstream maintainers.
Unchanged/non-actionable checks should stay quiet. This document does not itself
schedule a heartbeat.

For each update, fetch upstream read-only, select an explicit reviewed base,
compare the patch inventory, and update Go SDK/pkg, derived engine and generated
Node SDK/locks together. Avoid unrelated provider upgrades. Resolve conflicts
semantically against the tests, especially package generation, host selection and
release guards; never accept an upstream workflow wholesale. Repeat the evidence
above and open a `codex/` PR against `Remark-Inc/sst`. Keep immutable prior binaries,
checksums, generated dependency inventories and state exports for rollback.

Binary rollback is not guaranteed to reverse resource changes, state migrations or
provider schema changes. Do not blindly reuse newer state with an older engine;
review compatibility and preview in disposable stacks first. Deleting generated
platform caches requires reinstalling the pinned prior dependency set.

Exit when an official version includes equivalent fixes and passes the same clean
install, host conflict, security and staging acceptance gates. Pin that official
version, review a separate Forum migration, retain rollback artifacts, and archive
this patch inventory only after the agreed observation window. Do not delete the
host workaround or infer workspace remediation solely because official versions moved.
