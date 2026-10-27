// Fork patch for @pulumi/pulumi 3.272.0. Preserve the SDK's original license.
// The SDK passes a carrier with a set() method to propagation.inject(), but the
// default OpenTelemetry setter assigns object properties instead of calling it.
// Supply the setter so traceparent reaches gRPC Metadata. Fail closed on updates.
const fs = require("node:fs");
const path = require("node:path");
const { createRequire } = require("node:module");
const { randomUUID } = require("node:crypto");

function patchPulumiTracing(sdkRoot) {
  if (Number(process.versions.node.split(".")[0]) < 22) {
    throw new Error(`The SST fork requires Node.js 22 or newer; found ${process.version}`);
  }
  const version = JSON.parse(fs.readFileSync(path.join(sdkRoot, "package.json"), "utf8")).version;
  if (version !== "3.272.0") {
    throw new Error(`Review the SST fork tracing patch before using Pulumi ${version}`);
  }
  const target = path.join(sdkRoot, "runtime", "settings.js");
  const original = "opentelemetry.propagation.inject(opentelemetry.context.active(), carrier);";
  const replacement = "opentelemetry.propagation.inject(opentelemetry.context.active(), carrier, { set: (target, key, value) => target.set(key, value) });";
  const source = fs.readFileSync(target, "utf8");
  const oldCount = source.split(original).length - 1;
  const newCount = source.split(replacement).length - 1;
  if (oldCount === 0 && newCount === 1) return false;
  if (oldCount !== 1 || newCount !== 0) {
    throw new Error("Pulumi tracing source changed; review the SST fork patch before installing");
  }
  // Rename a new file rather than mutating a package-manager cache hardlink.
  const temporary = `${target}.${randomUUID()}.tmp`;
  try {
    fs.writeFileSync(temporary, source.replace(original, replacement), {
      flag: "wx", mode: fs.statSync(target).mode,
    });
    fs.renameSync(temporary, target);
  } finally {
    fs.rmSync(temporary, { force: true });
  }
  return true;
}

if (require.main === module) {
  const platformRequire = createRequire(path.join(__dirname, "..", "package.json"));
  const sdkRoot = path.dirname(platformRequire.resolve("@pulumi/pulumi/package.json"));
  const changed = patchPulumiTracing(sdkRoot);
  console.log(`SST fork Pulumi trace propagation patch: ${changed ? "applied" : "already applied"}`);
}

module.exports = { patchPulumiTracing };
