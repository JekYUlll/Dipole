import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import { agentElicitationSchemaVersion, validateElicitationForm, validateElicitationResponse } from "./agent-elicitation.js";

const form = {
  schemaVersion: "dipole.agent.elicitation.v1" as const,
  fields: [
    { id: "scope", label: "Scope", type: "select" as const, required: true, options: ["today", "week"] },
    { id: "notes", label: "Notes", type: "text" as const, required: false, maxLength: 100 },
    { id: "notify", label: "Notify me", type: "boolean" as const, required: true }
  ]
};

describe("Agent Elicitation v1", () => {
  it("keeps the language-neutral contract aligned with the runtime version", async () => {
    const path = new URL("../../../contracts/agent-elicitation/v1/schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      properties: { schemaVersion: { const: string }; fields: { minItems: number; maxItems: number } };
    };
    expect(schema.$id).toMatch(/agent-elicitation\/v1\/schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(agentElicitationSchemaVersion);
    expect(schema.properties.schemaVersion.const).toBe(agentElicitationSchemaVersion);
    expect(schema.properties.fields).toMatchObject({ minItems: 1, maxItems: 16 });
  });

  it("accepts a bounded deterministic form and exact response", () => {
    expect(validateElicitationForm(form)).toEqual(form);
    expect(validateElicitationResponse(form, { scope: "today", notify: true })).toEqual({ scope: "today", notify: true });
  });

  it("rejects unknown fields, missing required values, and invalid options", () => {
    expect(() => validateElicitationResponse(form, { scope: "month", notify: true })).toThrow(/scope/);
    expect(() => validateElicitationResponse(form, { scope: "today" })).toThrow(/notify/);
    expect(() => validateElicitationResponse(form, { scope: "today", notify: true, principalUserId: "U999" })).toThrow(/principalUserId/);
  });

  it("rejects duplicate field IDs and malformed type-specific options", () => {
    expect(() => validateElicitationForm({ ...form, fields: [form.fields[0], form.fields[0]] })).toThrow(/unique/);
    expect(() => validateElicitationForm({
      schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "bad", label: "Bad", type: "select", required: true, options: [] }]
    })).toThrow(/options/);
  });
});
