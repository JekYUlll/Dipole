import { describe, expect, it, vi } from "vitest";
import { GetWeatherCapability } from "./get-weather.js";
import { CapabilityRegistry } from "./registry.js";
import { executeShadowPlan } from "../events/shadow-processor.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U1", agentUuid: "AI", taskId: "T1", runId: "R1", mode: "active",
  permissions: ["weather.read"], resourceScopes: [{ resourceType: "weather", resourceId: "*", actions: ["read"] }], approvedCapabilities: []
};
const location = { name: "Beijing", country: "China", latitude: 39.9, longitude: 116.4 };
const current = { time: "2026-09-15T12:00", temperature_2m: 25, apparent_temperature: 26,
  relative_humidity_2m: 40, precipitation: 0, weather_code: 0, wind_speed_10m: 8 };
function fixture(...results: unknown[]) {
  const fetcher = vi.fn(async () => Response.json(results.shift()));
  const registry = new CapabilityRegistry();
  registry.register(new GetWeatherCapability(fetcher as typeof fetch));
  return { fetcher, registry };
}

describe("get_weather", () => {
  it("returns bounded current weather with matched location, units and source", async () => {
    const { registry, fetcher } = fixture({ results: [location] }, { timezone: "Asia/Shanghai", current });
    expect(await registry.execute("get_weather", { city: "Beijing", countryCode: "cn" }, context)).toMatchObject({
      found: true, location, current, source: "Open-Meteo", units: { temperature: "C" }
    });
    const calls = fetcher.mock.calls as unknown as [URL, RequestInit][];
    expect(calls[0]![0].searchParams.get("countryCode")).toBe("CN");
    expect(calls[1]![0].hostname).toBe("api.open-meteo.com");
    expect(calls[0]![1].signal).toBeInstanceOf(AbortSignal);
    expect(calls[0]![1].redirect).toBe("error");
  });
  it("does not invent weather for unknown cities", async () => {
    const { registry, fetcher } = fixture({});
    expect(await registry.execute("get_weather", { city: "unknown" }, context)).toMatchObject({ found: false });
    expect(fetcher).toHaveBeenCalledOnce();
  });
  it("rejects authority and URL arguments and requires a weather grant", async () => {
    const { registry, fetcher } = fixture();
    await expect(registry.execute("get_weather", { city: "Beijing", url: "http://localhost" }, context)).rejects.toThrow();
    await expect(registry.execute("get_weather", { city: "Beijing" }, { ...context, permissions: [] })).rejects.toThrow(/permission/);
    await expect(registry.execute("get_weather", { city: "Beijing" }, { ...context, resourceScopes: [] })).rejects.toThrow(/scope/);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("propagates provider failures and rejects malformed or oversized data", async () => {
    const capability = new GetWeatherCapability(vi.fn(async () => new Response("unavailable", { status: 503 })) as typeof fetch);
    await expect(capability.execute({ city: "Beijing" })).rejects.toThrow(/503/);
    const oversized = new GetWeatherCapability(vi.fn(async () => new Response("x".repeat(65537))) as typeof fetch);
    await expect(oversized.execute({ city: "Beijing" })).rejects.toThrow(/too large/);
    const { registry } = fixture({ results: [location] }, { current: { temperature_2m: null } });
    await expect(registry.execute("get_weather", { city: "Beijing" }, context)).rejects.toThrow();
  });
  it("feeds weather evidence back into the same Agent task answer", async () => {
    const { registry } = fixture({ results: [location] }, { timezone: "Asia/Shanghai", current });
    const answer = vi.fn(async (_event, ctx, evidence) => {
      expect(ctx.taskId).toBe("T1");
      expect(evidence[0]).toMatchObject({ capabilityId: "get_weather", output: { current } });
      return "Beijing is 25 C (Open-Meteo).";
    });
    const result = await executeShadowPlan({ eventId: "E1", aggregateId: "M1", eventType: "message.direct.created",
      occurredAt: "2026-09-15T00:00:00Z", payload: { content: "Beijing weather?" } }, context, {
      planner: { plan: async () => ({ summary: "checking", steps: [{ capabilityId: "get_weather", input: { city: "Beijing" } }] }), answer },
      registry, audit: { append: async () => undefined }, stepLeaseMs: 30_000,
      trajectory: { append: async () => undefined, claimStep: async () => ({ outcome: "claimed", token: "lease" }),
        completeStep: async () => undefined, failStep: async () => undefined }
    });
    expect(result.summary).toBe("Beijing is 25 C (Open-Meteo).");
    expect(answer).toHaveBeenCalledOnce();
  });

  it("keeps the task reply path available when a model proposes invalid weather input", async () => {
    const { registry } = fixture({ results: [location] }, { timezone: "Asia/Shanghai", current });
    const answer = vi.fn(async (_event, _ctx, evidence) => {
      expect(evidence).toEqual([{ capabilityId: "get_weather", error: "invalid_input" }]);
      return "我可以查询天气，请告诉我城市。";
    });
    const failStep = vi.fn(async () => undefined);
    const result = await executeShadowPlan({ eventId: "E2", aggregateId: "M2", eventType: "message.direct.created",
      occurredAt: "2026-09-15T00:00:00Z", payload: { content: "你有什么能力" } }, context, {
      planner: { plan: async () => ({ summary: "checking", steps: [{ capabilityId: "get_weather", input: { location: "current location" } }] }), answer },
      registry, audit: { append: async () => undefined }, stepLeaseMs: 30_000,
      trajectory: { append: async () => undefined, claimStep: async () => ({ outcome: "claimed", token: "lease" }),
        completeStep: async () => undefined, failStep }
    });
    expect(result.summary).toBe("我可以查询天气，请告诉我城市。");
    expect(failStep).toHaveBeenCalledOnce();
  });
});
