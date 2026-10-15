#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const { createHash } = require("node:crypto");
const { createRequire } = require("node:module");

function inventory(directory) {
  const platform = fs.realpathSync(directory);
  const packages = [];
  function visitModules(modules, ancestors = new Set()) {
    if (!fs.existsSync(modules)) return;
    const real = fs.realpathSync(modules);
    if (ancestors.has(real)) throw new Error(`Cyclic node_modules path: ${modules}`);
    const next = new Set([...ancestors, real]);
    for (const name of fs.readdirSync(modules).sort()) {
      if (name.startsWith(".")) continue;
      const entry = path.join(modules, name);
      if (name.startsWith("@")) { visitModules(entry, next); continue; }
      const manifest = path.join(entry, "package.json");
      if (!fs.existsSync(manifest)) continue;
      const bytes = fs.readFileSync(manifest);
      const pkg = JSON.parse(bytes);
      packages.push({ name: pkg.name, version: pkg.version,
        path: path.relative(platform, entry), license: pkg.license,
        manifestSHA256: createHash("sha256").update(bytes).digest("hex") });
      visitModules(path.join(entry, "node_modules"), next);
    }
  }
  visitModules(path.join(platform, "node_modules"));
  packages.sort((a, b) => a.name.localeCompare(b.name) || a.path.localeCompare(b.path));
  const platformRequire = createRequire(path.join(platform, "package.json"));
  const sdkManifest = platformRequire.resolve("@pulumi/pulumi/package.json");
  const sdkRequire = createRequire(sdkManifest);
  const sdk = JSON.parse(fs.readFileSync(sdkManifest));
  const tracingResolution = {};
  for (const name of Object.keys(sdk.dependencies).filter((name) => name.startsWith("@opentelemetry/") || name === "@grpc/grpc-js").sort()) {
    let location = path.dirname(sdkRequire.resolve(name));
    for (;;) {
      const manifest = path.join(location, "package.json");
      if (fs.existsSync(manifest)) {
        const pkg = JSON.parse(fs.readFileSync(manifest));
        if (pkg.name === name) {
          tracingResolution[name] = { version: pkg.version, path: path.relative(platform, location) };
          break;
        }
      }
      const parent = path.dirname(location);
      if (parent === location) throw new Error(`Cannot locate manifest for ${name}`);
      location = parent;
    }
  }
  const hashes = {};
  for (const name of ["package.json", "bun.lockb", "bun.lock", "package-lock.json"]) {
    const file = path.join(platform, name);
    if (fs.existsSync(file)) hashes[name] = createHash("sha256").update(fs.readFileSync(file)).digest("hex");
  }
  return { node: process.version, platform: process.platform, architecture: process.arch,
    manifestAndLockSHA256: hashes, sdkVersion: sdk.version, tracingResolution, packages };
}

if (require.main === module) console.log(JSON.stringify(inventory(process.argv[2]), null, 2));
module.exports = { inventory };
