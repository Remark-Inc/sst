# October 29 security candidate validation

Pulumi engine, Go pkg/SDK and generated Node SDK are coordinated at 3.273.0.
Go remains 1.26.10 for the SST CLI; provider package versions and inherited
example/website manifests are unchanged. Official SST base remains 4.17.2.

Local macOS arm64 validation passed: dirty review build; Go vet/tests; platform
typecheck and 111 tests (15 existing todo); Go vulnerability gate (zero reachable
or imported-package findings, OpenPGP module-only); four gate regression controls;
fresh frozen generated install; actual engine/host-conflict controls; all 15
installed tracing/resource/secret tests on Node 26.5.1 and 24.18.1; generated
audit with zero advisory records; installer and publishing guards.

Synthetic secret-map and secret-array controls against exported Pulumi
`display.MassageSecrets` fail on 3.272.0 and pass on 3.273.0 while retaining
public values. The exact upstream 3.273.0 tests
`TestMassageSecretsRedactsContainers` and
`TestPreviewMetadataStepRedactsSecretMaps` pass, including nested inputs/outputs,
invalid UTF-8 and explicit show-secrets controls. No real secrets are used.

Official 3.273.0 engine archives for macOS arm64 and Linux x64 were verified
against the upstream checksums. Both contain x/net 0.62.0 and clear the two
October 28 HTML findings. Raw binary scans still exit 3 for OpenPGP. Extraction
returns no package symbols; govulncheck falls back to module precision and
synthesizes affected symbol names. This corrects the earlier symbol-presence
interpretation without changing or suppressing the scan. Exact-release source
analysis uses Go 1.27.3 and the release build tags; the source dependency versions
match all 255 dependencies in the macOS binary. No imported OpenPGP package or
reachable finding is reported. Keep the raw binary/source distinction explicit.

Committed repeat-byte artifacts, Linux/macOS/Node 24 CI and refreshed review
results are recorded in the PR after commit. These cannot be inferred from the
local dirty build. Forum's immutable pin is unchanged. Publication, adoption,
lifecycle/rollback rehearsal and staging/production remain separate decisions.
