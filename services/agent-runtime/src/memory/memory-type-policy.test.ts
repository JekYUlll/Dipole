import { describe, expect, it } from "vitest";

import {
  agentMemoryTypes,
  getAgentMemoryTypePolicy,
  parseAgentMemoryType,
  validateMemoryTypeTransition,
} from "./memory-type-policy.js";

describe("Agent Memory type policy", () => {
  it("supports exactly the five storage types", () => {
    expect(agentMemoryTypes).toEqual(["working", "episodic", "semantic", "procedural", "observational"]);
    for (const type of agentMemoryTypes) expect(parseAgentMemoryType(type)).toBe(type);
    expect(() => parseAgentMemoryType("vector")).toThrow();
  });

  it("keeps task-scoped working memory distinct from reviewed durable types", () => {
    expect(getAgentMemoryTypePolicy("working")).toMatchObject({ durable: false, taskScoped: true, requiresReview: false });
    for (const type of ["episodic", "semantic", "procedural", "observational"] as const) {
      expect(getAgentMemoryTypePolicy(type)).toMatchObject({ durable: true, taskScoped: false, requiresReview: true });
    }
  });

  it("requires an explicit target type and only accepts observational candidates", () => {
    expect(validateMemoryTypeTransition("observational", "semantic")).toBe("semantic");
    expect(validateMemoryTypeTransition("observational", "working")).toBe("working");
    expect(() => validateMemoryTypeTransition("semantic", "semantic")).toThrow();
    expect(() => validateMemoryTypeTransition("observational", undefined)).toThrow();
    expect(() => validateMemoryTypeTransition("observational", "vector")).toThrow();
  });
});
