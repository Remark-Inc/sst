import { describe, beforeAll, it, expect } from "vitest";
import type {} from "../../src/global.d.ts";
import * as pulumi from "@pulumi/pulumi";

// @ts-ignore
global.$app = {
  name: "app",
  stage: "test",
};
global.$util = pulumi;

pulumi.runtime.setMocks(
  {
    newResource: function (args: pulumi.runtime.MockResourceArgs): {
      id: string;
      state: any;
    } {
      return {
        id: args.inputs.name + "_id",
        state: {
          ...args.inputs,
          arn: `arn:aws:elasticloadbalancing:us-east-1:123456789:${args.type}/${args.inputs.name}`,
          dnsName: `${args.inputs.name}.us-east-1.elb.amazonaws.com`,
          zoneId: "Z1234567890",
          vpcId: "vpc-mock-id",
          securityGroups: ["sg-mock-id"],
        },
      };
    },
    call: function (args: pulumi.runtime.MockCallArgs) {
      // Mock lb.getListener — return a fake ARN so Listener.get works
      if (args.token === "aws:alb/getListener:getListener") {
        return {
          arn: `arn:aws:elasticloadbalancing:us-east-1:123456789:listener/app/mock/${args.inputs.port}`,
          ...args.inputs,
        };
      }
      return args.inputs;
    },
  },
  "project",
  "stack",
  false,
);

describe("Alb", function () {
  let Alb: typeof import("../../src/components/aws/alb").Alb;

  beforeAll(async function () {
    Alb = (await import("../../src/components/aws/alb")).Alb;
  });

  describe("#constructor", () => {
    it("creates load balancer with correct type", async () => {
      const alb = new Alb("TestAlb", {
        vpc: {
          id: "vpc-123",
          publicSubnets: ["subnet-a", "subnet-b"],
          privateSubnets: ["subnet-c", "subnet-d"],
        },
        listeners: [{ port: 80, protocol: "http" }],
      });

      const arn = await new Promise<string>((resolve) => alb.arn.apply(resolve));
      expect(arn).toBeDefined();
    });

    it("exposes url without domain", async () => {
      const alb = new Alb("TestAlbUrl", {
        vpc: {
          id: "vpc-123",
          publicSubnets: ["subnet-a", "subnet-b"],
          privateSubnets: ["subnet-c", "subnet-d"],
        },
        listeners: [{ port: 80, protocol: "http" }],
      });

      const url = await new Promise<string>((resolve) => alb.url.apply(resolve));
      expect(url).toMatch(/^http:\/\//);
    });

    it("exposes security group ID", async () => {
      const alb = new Alb("TestAlbSg", {
        vpc: {
          id: "vpc-123",
          publicSubnets: ["subnet-a", "subnet-b"],
          privateSubnets: ["subnet-c", "subnet-d"],
        },
        listeners: [{ port: 80, protocol: "http" }],
      });

      const sgId = await new Promise<string>((resolve) => alb.securityGroupId.apply(resolve));
      expect(sgId).toBeDefined();
    });

    it("exposes nodes with loadBalancer, securityGroup, listeners", async () => {
      const alb = new Alb("TestAlbNodes", {
        vpc: {
          id: "vpc-123",
          publicSubnets: ["subnet-a", "subnet-b"],
          privateSubnets: ["subnet-c", "subnet-d"],
        },
        listeners: [{ port: 80, protocol: "http" }],
      });

      expect(alb.nodes.loadBalancer).toBeDefined();
      expect(alb.nodes.securityGroup).toBeDefined();
      expect(alb.nodes.listeners).toBeDefined();
    });
  });

  describe("#getListener", () => {
    it("discovers a listener not declared on a non-ref ALB", async () => {
      const alb = new Alb("TestAlbMissing", {
        vpc: {
          id: "vpc-123",
          publicSubnets: ["subnet-a", "subnet-b"],
          privateSubnets: ["subnet-c", "subnet-d"],
        },
        listeners: [{ port: 80, protocol: "http" }],
      });

      const listener = alb.getListener("https", 443);
      const arn = await new Promise<string>((resolve) => listener.arn.apply(resolve));
      expect(arn).toContain("aws:lb/listener:Listener");
    });

    it("discovered listener resource name includes ALB name", async () => {
      const alb = new Alb("MyNamedAlb", {
        vpc: {
          id: "vpc-123",
          publicSubnets: ["subnet-a", "subnet-b"],
          privateSubnets: ["subnet-c", "subnet-d"],
        },
        listeners: [{ port: 80, protocol: "http" }],
      });

      const listener = alb.getListener("https", 443);
      const urn = await new Promise<string>((resolve) => listener.urn.apply(resolve));
      expect(urn).toContain("MyNamedAlbListenerHTTPS443");
    });
  });

  describe(".get() reference", () => {
    it("creates a referenced ALB from ARN", async () => {
      const alb = Alb.get(
        "RefAlb",
        "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/my-alb/abc123",
      );

      const arn = await new Promise<string>((resolve) => alb.arn.apply(resolve));
      expect(arn).toBeDefined();
    });

    it("lazy discovers listener via getListener on ref ALB", async () => {
      const alb = Alb.get(
        "RefAlbListener",
        "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/my-alb/abc123",
      );

      // Should not throw — lazy discovery kicks in
      const listener = alb.getListener("http", 80);
      expect(listener).toBeDefined();
    });

    it("caches discovered listeners on subsequent calls", async () => {
      const alb = Alb.get(
        "RefAlbCache",
        "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/my-alb/abc123",
      );

      const listener1 = alb.getListener("http", 80);
      const listener2 = alb.getListener("http", 80);
      expect(listener1).toBe(listener2);
    });

    it("discovers different listeners independently", async () => {
      const alb = Alb.get(
        "RefAlbMulti",
        "arn:aws:elasticloadbalancing:us-east-1:123456789:loadbalancer/app/my-alb/abc123",
      );

      const http = alb.getListener("http", 80);
      const https = alb.getListener("https", 443);
      expect(http).not.toBe(https);
    });
  });
});
