// Child process for fork-platform-tests.cjs. Only loopback gRPC is used.
const assert = require("node:assert/strict");
const path = require("node:path");
const { createRequire } = require("node:module");
const platformRequire = createRequire(path.join(process.argv[2], "package.json"));
const sdkRequire = createRequire(platformRequire.resolve("@pulumi/pulumi/package.json"));
const mode = process.argv[3];
const parent = "00-1234567890abcdef1234567890abcdef-1234567890abcdef-01";
delete process.env.PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT;
delete process.env.TRACEPARENT;
if (mode === "enabled") process.env.TRACEPARENT = parent;
if (mode === "malformed") process.env.TRACEPARENT = "not-a-traceparent";
// Pulumi requires this import before grpc-js or any Pulumi module that uses it.
const instrumentation = sdkRequire("./cmd/run/instrumentation.js");
const api = sdkRequire("@opentelemetry/api");
const grpc = sdkRequire("@grpc/grpc-js");
const { ResourceMonitorService, ResourceMonitorClient } = sdkRequire("./proto/resource_grpc_pb.js");
const { SupportsFeatureRequest, SupportsFeatureResponse } = sdkRequire("./proto/resource_pb.js");
const settings = sdkRequire("./runtime/settings.js");

(async () => {
  const server = new grpc.Server();
  let received;
  let client;
  server.addService(ResourceMonitorService, {
    supportsFeature(call, callback) {
      received = call.metadata.get("traceparent");
      const response = new SupportsFeatureResponse();
      response.setHassupport(true);
      callback(null, response);
    },
  });
  try {
    const port = await new Promise((resolve, reject) => server.bindAsync(
      "127.0.0.1:0", grpc.ServerCredentials.createInsecure(),
      (error, value) => error ? reject(error) : resolve(value)));
    client = new ResourceMonitorClient(`127.0.0.1:${port}`,
      grpc.credentials.createInsecure(), settings.grpcClientOptions);
    await instrumentation.withRootContext(async () => {
      await new Promise((resolve) => setImmediate(resolve));
      const span = api.trace.getSpanContext(api.context.active());
      if (mode === "enabled") assert.equal(span?.traceId, parent.split("-")[1]);
      else assert.equal(span, undefined);
      const request = new SupportsFeatureRequest();
      request.setId("secrets");
      const response = await new Promise((resolve, reject) => client.supportsFeature(request,
        { deadline: Date.now() + 5_000 }, (error, result) => error ? reject(error) : resolve(result)));
      assert.equal(response.getHassupport(), true);
    });
    assert.deepEqual(received, mode === "enabled" ? [parent] : [],
      "traceparent must reach the mock monitor through the installed SDK's real gRPC interceptor");
  } finally {
    client?.close();
    server.forceShutdown();
    await instrumentation.shutdownTracing();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
