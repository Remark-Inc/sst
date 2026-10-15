#!/usr/bin/env node
// No cloud credentials, providers, or deployments are used by this suite.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { createRequire } = require("node:module");
const { spawnSync } = require("node:child_process");
const { test } = require("node:test");
const os = require("node:os");
const { inventory } = require("./fork-platform-inventory.cjs");

const platform = fs.realpathSync(process.env.SST_FORK_PLATFORM_DIR || "platform");
const platformRequire = createRequire(path.join(platform, "package.json"));
const sdkPath = platformRequire.resolve("@pulumi/pulumi/package.json");
const sdkRequire = createRequire(sdkPath);
const expected = require("../platform/package.json").dependencies["@pulumi/pulumi"];

test("the installed SDK is the fork's exact version and comes from the target installation", () => {
  assert.equal(sdkRequire("./package.json").version, expected);
  assert.ok(fs.realpathSync(sdkPath).startsWith(platform + path.sep),
    `SDK resolved outside target installation: ${sdkPath}`);
});

test("providers cannot retain a legacy Pulumi SDK or vulnerable tracing generation", () => {
  const installed = inventory(platform).packages;
  const sdks = installed.filter((pkg) => pkg.name === "@pulumi/pulumi");
  assert.ok(sdks.length > 0);
  for (const pkg of sdks) assert.equal(pkg.version, expected, pkg.path);
  for (const pkg of installed.filter((pkg) => pkg.name === "@opentelemetry/core" || pkg.name === "@opentelemetry/propagator-jaeger")) {
    const [major, minor] = pkg.version.split(".").map(Number);
    const minimumMinor = pkg.name === "@opentelemetry/core" ? 8 : 9;
    assert.ok(major > 2 || (major === 2 && minor >= minimumMinor),
      `${pkg.path} retains known vulnerable generation ${pkg.version}`);
  }
});

test("tracing patch is idempotent and refuses changed versions or source", () => {
  const { patchPulumiTracing } = require("../platform/scripts/patch-pulumi-tracing.cjs");
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "sst-tracing-patch-"));
  try {
    fs.mkdirSync(path.join(directory, "runtime"));
    const manifest = path.join(directory, "package.json");
    const target = path.join(directory, "runtime", "settings.js");
    fs.writeFileSync(manifest, JSON.stringify({ version: expected }));
    const original = "opentelemetry.propagation.inject(opentelemetry.context.active(), carrier);";
    const replacement = "opentelemetry.propagation.inject(opentelemetry.context.active(), carrier, { set: (target, key, value) => target.set(key, value) });";
    const source = fs.readFileSync(path.join(path.dirname(sdkPath), "runtime", "settings.js"), "utf8")
      .replace(replacement, original);
    fs.writeFileSync(target, source);
    const cachedCopy = path.join(directory, "package-manager-cache.js");
    fs.linkSync(target, cachedCopy);
    assert.equal(patchPulumiTracing(directory), true);
    assert.equal(fs.readFileSync(cachedCopy, "utf8"), source, "must not mutate package-manager cache hardlinks");
    const patched = fs.readFileSync(target, "utf8");
    assert.equal(patchPulumiTracing(directory), false);
    assert.equal(fs.readFileSync(target, "utf8"), patched);
    fs.writeFileSync(manifest, JSON.stringify({ version: "3.268.0" }));
    assert.throws(() => patchPulumiTracing(directory), /Review the SST fork tracing patch/);
    assert.equal(fs.readFileSync(target, "utf8"), patched);
    fs.writeFileSync(manifest, JSON.stringify({ version: expected }));
    fs.writeFileSync(target, source.replace(original, "changedSource();"));
    assert.throws(() => patchPulumiTracing(directory), /tracing source changed/);
    fs.writeFileSync(target, source + "\n" + original);
    assert.throws(() => patchPulumiTracing(directory), /tracing source changed/);
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

test("mocked resource outputs preserve values and secret taint", async () => {
  const pulumi = sdkRequire("./index.js");
  const seen = [];
  await pulumi.runtime.setMocks({
    newResource(args) {
      seen.push(args);
      return { id: `${args.name}-id`, state: { ...args.inputs, endpoint: pulumi.secret("https://mock.invalid") } };
    },
    call(args) { return args.inputs; },
  }, "fork-validation", "offline", false);
  class FixtureResource extends pulumi.CustomResource {
    constructor() {
      super("fork:test:Resource", "fixture", {
        plain: "public-value",
        token: pulumi.secret("mock-secret-only"),
        endpoint: undefined,
      });
    }
  }
  const resource = new FixtureResource();
  const value = (output) => new Promise((resolve, reject) => output.apply(resolve).promise().catch(reject));
  assert.equal(await value(resource.id), "fixture-id");
  assert.equal(await value(resource.plain), "public-value");
  assert.equal(await value(resource.token), "mock-secret-only");
  assert.equal(await value(resource.endpoint), "https://mock.invalid");
  assert.equal(await pulumi.isSecret(resource.plain), false);
  assert.equal(await pulumi.isSecret(resource.token), true);
  assert.equal(await pulumi.isSecret(resource.endpoint), true);
  const derived = pulumi.all([resource.plain, resource.token]).apply(([a, b]) => `${a}:${b}`);
  assert.equal(await value(derived), "public-value:mock-secret-only");
  assert.equal(await pulumi.isSecret(derived), true);
  assert.equal(seen.filter((args) => args.type === "fork:test:Resource").length, 1);
});

test("installed OpenTelemetry baggage parser accepts the valid control", () => {
  const api = sdkRequire("@opentelemetry/api");
  const { W3CBaggagePropagator } = sdkRequire("@opentelemetry/core");
  const propagator = new W3CBaggagePropagator();
  const context = propagator.extract(api.ROOT_CONTEXT,
    { baggage: "tenant=forum,space=hello%20world" }, api.defaultTextMapGetter);
  const baggage = api.propagation.getBaggage(context);
  assert.equal(baggage.getEntry("tenant").value, "forum");
  assert.equal(baggage.getEntry("space").value, "hello world");
  const carrier = {};
  propagator.inject(context, carrier, api.defaultTextMapSetter);
  assert.equal(carrier.baggage, "tenant=forum,space=hello%20world");
});

test("installed OpenTelemetry bounds baggage entries and bytes", () => {
  const api = sdkRequire("@opentelemetry/api");
  const { W3CBaggagePropagator } = sdkRequire("@opentelemetry/core");
  const propagator = new W3CBaggagePropagator();
  for (const header of [
    Array.from({ length: 10_000 }, (_, i) => `key${i}=value`).join(","),
    Array.from({ length: 100 }, (_, i) => `key${i}=${"a".repeat(1024)}`).join(","),
    `oversized=${"a".repeat(1024 * 1024)},control=valid`,
  ]) {
    const context = propagator.extract(api.ROOT_CONTEXT, { baggage: header }, api.defaultTextMapGetter);
    const entries = api.propagation.getBaggage(context)?.getAllEntries() || [];
    assert.ok(entries.length <= 180, `retained ${entries.length} baggage entries`);
    assert.ok(entries.every(([key, entry]) => key.length + entry.value.length <= 4096));
    assert.ok(entries.reduce((size, [key, entry]) => size + key.length + entry.value.length, 0) <= 8192);
    const carrier = {};
    propagator.inject(context, carrier, api.defaultTextMapSetter);
    assert.ok((carrier.baggage || "").length <= 8192);
  }
});

for (const malformed of ["broken=%", "broken=%G1", "broken=%E0%A4%A", "%ZZ=value", "missing-equals"]) {
  test(`installed OpenTelemetry safely ignores malformed baggage: ${malformed}`, () => {
    const api = sdkRequire("@opentelemetry/api");
    const { W3CBaggagePropagator } = sdkRequire("@opentelemetry/core");
    const propagator = new W3CBaggagePropagator();
    let context;
    assert.doesNotThrow(() => {
      context = propagator.extract(api.ROOT_CONTEXT,
        { baggage: `before=valid,${malformed},after=valid` }, api.defaultTextMapGetter);
    });
    const baggage = api.propagation.getBaggage(context);
    assert.deepEqual(baggage.getAllEntries().map(([key, entry]) => [key, entry.value]),
      [["before", "valid"], ["after", "valid"]]);
  });
}

for (const mode of ["enabled", "disabled", "malformed"]) {
  test(`Pulumi SDK tracing ${mode}: root context and outgoing gRPC metadata`, () => {
    // Isolated processes prevent global telemetry registrations leaking between controls.
    const result = spawnSync(process.execPath,
      [path.join(__dirname, "fork-platform-trace.cjs"), platform, mode],
      { encoding: "utf8", timeout: 20_000, env: { ...process.env } });
    assert.equal(result.error, undefined, result.error?.message);
    assert.equal(result.status, 0, result.stdout + result.stderr);
  });
}
