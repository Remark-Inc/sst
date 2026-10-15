import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // Pulumi closure serialization loads Node builtins, including trace_events,
    // which Node does not support inside worker threads.
    pool: "forks",
  },
});
