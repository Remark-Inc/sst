import { afterEach, describe, expect, it, vi } from "vitest";
import { ListBucketsCommand, S3Client } from "@aws-sdk/client-s3";
import { useClient } from "../../../src/components/aws/helpers/client";

class CapturedClient {
  constructor(public config: any) {}
}

const request = () => ({
  request: {
    method: "GET",
    protocol: "https:",
    hostname: "example.invalid",
    path: "/",
    headers: {},
    query: {},
  },
});
const success = () => ({ response: {}, output: { $metadata: {} } });

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
});

describe("AWS retry strategy compatibility", () => {
  it("preserves custom errors, unlimited retry quota, exponential backoff and its cap", async () => {
    const delays: number[] = [];
    vi.spyOn(globalThis, "setTimeout").mockImplementation(((
      callback: () => void,
      delay: number,
    ) => {
      delays.push(delay);
      callback();
      return 0;
    }) as any);
    const client = useClient(CapturedClient, {
      retrableErrors: ["CustomTransient"],
    });
    let attempts = 0;
    const result = await client.config.retryStrategy.retry(async () => {
      if (++attempts <= 80)
        throw Object.assign(new Error("retry"), { name: "CustomTransient" });
      return success();
    }, request());
    expect(attempts).toBe(81); // Beyond the stock retry-token quota.
    expect(delays.slice(0, 3)).toEqual([150, 225, 337.5]);
    expect(delays.at(-1)).toBe(5000);
    expect(result.output.$metadata.attempts).toBe(81);
  });

  it("retains the 10,000-attempt bound", async () => {
    vi.spyOn(globalThis, "setTimeout").mockImplementation(((
      callback: () => void,
    ) => {
      callback();
      return 0;
    }) as any);
    const client = useClient(CapturedClient);
    let attempts = 0;
    const error = Object.assign(new Error("throttled"), {
      name: "ThrottlingException",
    });
    await expect(
      client.config.retryStrategy.retry(async () => {
        attempts++;
        throw error;
      }, request()),
    ).rejects.toBe(error);
    expect(attempts).toBe(10000);
  });

  it("does not retry a permanent error", async () => {
    const client = useClient(CapturedClient);
    const error = Object.assign(new Error("denied"), {
      name: "AccessDeniedException",
    });
    const next = vi.fn(async () => {
      throw error;
    });
    await expect(
      client.config.retryStrategy.retry(next, request()),
    ).rejects.toBe(error);
    expect(next).toHaveBeenCalledTimes(1);
  });

  it("sends and parses S3 XML using the upgraded SDK and the existing strategy", async () => {
    vi.stubEnv("SST_AWS_ACCESS_KEY_ID", "AKIDEXAMPLE");
    vi.stubEnv("SST_AWS_SECRET_ACCESS_KEY", "test-secret");
    vi.stubEnv("SST_AWS_REGION", "us-east-1");
    const client = useClient(S3Client);
    const handle = vi.fn(async () => ({
      response: {
        statusCode: 200,
        headers: {
          "content-type": "application/xml",
          "x-amz-request-id": "local-test",
        },
        body: new TextEncoder().encode(
          '<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Buckets><Bucket><Name>mock-bucket</Name><CreationDate>2026-01-01T00:00:00Z</CreationDate></Bucket></Buckets><Owner><ID>mock-owner</ID></Owner></ListAllMyBucketsResult>',
        ),
      },
    }));
    // A local request handler prevents sockets, credential lookups, or AWS calls.
    client.config.requestHandler = { handle } as any;
    const result = await client.send(new ListBucketsCommand({}));
    expect(result.Buckets?.[0].Name).toBe("mock-bucket");
    expect(result.Owner?.ID).toBe("mock-owner");
    expect(result.$metadata.attempts).toBe(1);
    expect(handle).toHaveBeenCalledTimes(1);
    const sent = handle.mock.calls[0] as any;
    expect(sent[0].headers.authorization).toContain("Credential=AKIDEXAMPLE/");
    client.destroy();
  });
});
