import { describe, expect, it } from "vitest";

import { buildServer } from "./server.js";

describe("agent runtime health", () => {
  it("separates liveness from readiness", async () => {
    let ready = false;
    const server = buildServer({ isReady: () => ready });

    expect((await server.inject({ method: "GET", url: "/livez" })).statusCode).toBe(200);
    expect((await server.inject({ method: "GET", url: "/readyz" })).statusCode).toBe(503);
    ready = true;
    expect((await server.inject({ method: "GET", url: "/readyz" })).statusCode).toBe(200);
    await server.close();
  });
});
