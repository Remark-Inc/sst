# Upstream assessment — October 29, 2026

Official SST stable remains [v4.17.2](https://github.com/anomalyco/sst/releases/tag/v4.17.2),
published October 8 at 15:14 UTC. Its tag and current `upstream/dev` both resolve to
`e49ac31411f63ab4ed0d802b7e15d95f6b827e47`, the fork's existing base. There are no
new official SST commits to absorb. [Pulumi upgrade PR 6995](https://github.com/anomalyco/sst/pull/6995)
is still open. No Forum patch has become redundant in official SST. Community
fork releases are optional evidence, not official upstream or automatic updates.

Forum integration #2279 still pins the previously published immutable fork
`36ef23545d9596c4f181ef79e60fb8cc60914d9e`. This security-update source is a review
candidate, not a replacement release. Do not change Forum's checksums until a
reviewed merged commit has new verified immutable artifacts.

## October 29 refresh

Official SST is unchanged. Pulumi stable 3.273.0, published October 28 at
19:51:35 UTC, is a justified coordinated security refresh: its actual Linux x64
and macOS arm64 engine archives contain x/net 0.62.0, fixing GO-2026-6860 and
GO-2026-6866. It also fixes secret map/array leakage in JSON preview/update
output ([upstream #25400](https://github.com/pulumi/pulumi/pull/25400)). Synthetic
controls fail on 3.272.0 and pass on 3.273.0; upstream regression tests pass.
Engine, Go pkg/SDK, generated Node SDK, locks and checksums move together.
The tracing setter is still needed; provider package versions remain unchanged.

The release is recent. No configured release-age policy exists in this checkout;
this focused security candidate does not bypass any age check. Publication and
live acceptance remain separately gated. The release removes deprecated bare
`pulumi logs` and `pulumi watch`; the inspected SST command dispatch does not use
those commands. Read/existence and generated Automation API changes still need
review in acceptance.

### Corrected engine/OpenPGP evidence

The October 28 description of affected symbols being present was too strong.
`govulncheck -mode=extract` recovers zero package symbols from the official
3.272.0 macOS and 3.273.0 macOS/Linux binaries. In govulncheck 1.9.0, that selects
a documented fallback that synthesizes all known vulnerable symbols from the
module graph. Binary exit 3 for GO-2026-5932 is not proof of imported OpenPGP.
Do not suppress, rewrite or describe that raw scan as passed.

The 3.273.0 source revision recorded in the official binary is
`874c77a73039cf013d6a685a15293843de2b0e01`. Analysis with Go 1.27.3, CGO disabled
and release tags `osusergo,grpcnotrace` finds no reachable or imported-package
findings; OpenPGP remains module-only. All 255 dependency versions recorded in
the macOS binary match the exact source module graph, and its imported package
list has no OpenPGP. Retain this as evidence for an explicit risk disposition,
not a release approval or a blanket security claim. See the October 29 validation
record and PR for Linux confirmation and final committed CI.

## Historical October 27 security update

- Go 1.26.10 fixes [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809), reachable
  from the prior CLI's websocket TLS path. The exact toolchain and module minimum
  are raised together. No exploitation in Forum has been established.
- The Go gate preserves JSON evidence and converts it to enforce failure for
  reachable findings. Regression controls include the prior real finding, a
  clean scan, scanner failure and malformed output. A green JSON-only scan from
  the previous release is not valid clean-security evidence.
- Pulumi Go pkg/SDK, engine, generated Node SDK, all three Bun locks and native
  engine checksums move together to stable **3.272.0**, published October 22 at
  21:29 UTC. It contains upstream fixes for [invalid-input diagnostic secrets](https://github.com/pulumi/pulumi/pull/24995)
  and [secret output logging](https://github.com/pulumi/pulumi/pull/25092).
  Provider package versions remain unchanged. Required Go transitive changes
  come from the coordinated Pulumi module requirements.
- The earlier fixing release 3.271.0 was tested first, but its published pkg
  references `apitype.PolicyException` and `PolicyExceptionMatch`, absent from
  its matching SDK. Stable 3.272.0 supplies those types; using a mismatched SDK
  or maintaining a second Pulumi source fork would add unnecessary divergence.
- The original OpenTelemetry `propagation.inject(..., carrier)` call is still
  present in Pulumi 3.272.0. Retain the source/version-checked setter patch and
  rerun tracing, malformed baggage and mocked secret/resource controls.

This checkout has no configured minimum-release-age/cooldown policy in its
instructions or package-manager configuration as of this assessment. Versions
and publication dates are recorded explicitly; no existing age check is disabled
or bypassed. Recheck any policy introduced later before publication or adoption.

Pulumi's intervening releases change read/preview behavior, unknown output
handling, policy exceptions and the AwaitError provider protocol (exit code 10).
Compilation and local-backend preview tests do not establish cloud compatibility
or rollback safety. The disposable lifecycle and refreshed Forum acceptance must
cover the selected engine before rollout.

## Retained divergence and removal conditions

| Delta | Why retained | Remove when |
| --- | --- | --- |
| Coordinated Pulumi/Go security update and frozen generated graph | Official SST still pins Pulumi 3.215.0; workspace overrides cannot fix generated installs | Official stable supplies the patched compatible graph and passes the same installed security and acceptance tests |
| gRPC tracing setter | Source still lacks the required setter | Upstream supplies equivalent behavior and trace/baggage tests pass without the patch |
| Matching engine/host selection and atomic installs | Official SST has not absorbed the fork changes | Upstream passes host-conflict and interrupted-install controls; separately review Adam's workaround |
| Fork identity, SDK base compatibility, checksum distribution and publishing guards | Forum must run reviewed CLI bytes while retaining upstream SST runtime SDK 4.17.2 | Return to a validated official release, with an explicit migration and rollback plan |
| Targeted AWS/build dependency fixes | The graph is consumed by CLI building or generated deployment | Upstream incorporates equivalent fixes and compatibility controls pass |

Inherited example and website manifests are unchanged. Their alert totals are
not a production-exposure conclusion; root builds still install the website/SDK
workspace graph. Audit the actual build and generated graphs separately. No alert
is dismissed and no scan is disabled to reduce counts.

## Next update and approval boundaries

Finish the refreshed security-update PR's native Linux/macOS, Node 24, generated install,
security, host selection, tracing and reproducibility evidence. Then obtain the
separate fork merge/publication decision, build the exact merged source, and
prepare Forum's new pin. Keep #2279 draft until its security hold and live
acceptance requirements are satisfied. Neither this document nor green CI grants
permission for the disposable S3/SSM rehearsal, shared staging deployment, Forum
merge or production rollout.

The weekly upstream check now belongs to the Forum SST integration owner. It
compares stable tags and source commits, checks applicable security fixes, and
prepares only justified focused changes. It stays quiet when unchanged and reuses
existing work rather than opening duplicate updates.
