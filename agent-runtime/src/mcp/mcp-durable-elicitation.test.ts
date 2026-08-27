import { describe, expect, it } from "vitest";

import { McpDurableElicitationAdapter } from "./mcp-durable-elicitation.js";

describe("MCP durable Elicitation adapter", () => {
  const adapter = new McpDurableElicitationAdapter(() => 1_000);

  it("maps a bounded form request to a lineage-bound durable wait", () => {
    const wait = adapter.request({
      request: formRequest(), requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", expiresAtUnixMs: 2_000
    });

    expect(wait.directive).toEqual({
      kind: "wait_input", requestId: "INPUT-1", prompt: "Choose event settings", expiresAtUnixMs: 2_000,
      form: { schemaVersion: "dipole.agent.elicitation.v1", fields: [
        { id: "title", label: "Event title", type: "text", required: true, maxLength: 120 },
        { id: "visibility", label: "Visibility", type: "select", required: true, options: ["team", "private"] },
        { id: "notify", label: "Notify attendees", type: "boolean", required: false },
        { id: "labels", label: "Labels", type: "multiselect", required: false, options: ["release", "incident"], maxSelections: 1 }
      ] }, checkpoint: wait.checkpoint
    });
    expect(wait.checkpoint).toMatchObject({
      schemaVersion: "dipole.mcp.elicitation-checkpoint.v1", requestId: "INPUT-1", serverId: "calendar.example",
      toolName: "calendar.create", invocationId: "INV-1", trust: "untrusted"
    });
    expect(wait.checkpoint.bindingSha256).toMatch(/^[a-f0-9]{64}$/);
  });

  it("returns accepted content only for an exact durable resume", () => {
    const wait = adapter.request({
      request: formRequest(), requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", expiresAtUnixMs: 2_000
    });
    const result = adapter.result(wait.checkpoint, {
      action: "accept", resume: { kind: "input", requestId: "INPUT-1", value: { title: "Release", visibility: "team", labels: ["release"] } }
    });
    expect(result).toEqual({ action: "accept", content: { title: "Release", visibility: "team", labels: ["release"] } });

    expect(() => adapter.result(wait.checkpoint, {
      action: "accept", resume: { kind: "input", requestId: "INPUT-2", value: { title: "Release", visibility: "team" } }
    })).toThrow(/binding/i);
    expect(() => adapter.result({ ...wait.checkpoint, toolName: "calendar.delete" }, {
      action: "cancel", requestId: "INPUT-1"
    })).toThrow(/checkpoint/i);
  });

  it("supports explicit decline and cancel without accepting content", () => {
    const checkpoint = adapter.request({
      request: formRequest(), requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", expiresAtUnixMs: 2_000
    }).checkpoint;
    expect(adapter.result(checkpoint, { action: "decline", requestId: "INPUT-1" })).toEqual({ action: "decline" });
    expect(adapter.result(checkpoint, { action: "cancel", requestId: "INPUT-1" })).toEqual({ action: "cancel" });
    expect(() => new McpDurableElicitationAdapter(() => 2_000).result(checkpoint, {
      action: "cancel", requestId: "INPUT-1"
    })).toThrow(/expired/i);
  });

  it("rejects unsupported or authority-bearing request shapes", () => {
    const input = { requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create", invocationId: "INV-1", expiresAtUnixMs: 2_000 };
    expect(() => adapter.request({ ...input, request: { method: "elicitation/create", params: { mode: "url", message: "Authorize", url: "https://example.com" } } })).toThrow(/form/i);
    expect(() => adapter.request({ ...input, request: propertyRequest("count", { type: "integer" }) })).toThrow(/unsupported/i);
    expect(() => adapter.request({ ...input, request: propertyRequest("token", { type: "string", default: "secret" }) })).toThrow(/unsupported/i);
    expect(() => adapter.request({ ...input, request: { ...formRequest(), params: { ...formRequest().params, taskId: "TASK-OTHER" } } })).toThrow(/field/i);
    expect(() => adapter.request({ ...input, expiresAtUnixMs: 1_000, request: formRequest() })).toThrow(/deadline/i);
  });
});

function formRequest() {
  return {
    method: "elicitation/create" as const,
    params: {
      mode: "form" as const,
      message: "Choose event settings",
      requestedSchema: {
        type: "object" as const,
        properties: {
          title: { type: "string" as const, title: "Event title", maxLength: 120 },
          visibility: { type: "string" as const, title: "Visibility", enum: ["team", "private"] },
          notify: { type: "boolean" as const, title: "Notify attendees" },
          labels: { type: "array" as const, title: "Labels", items: { type: "string" as const, enum: ["release", "incident"] }, maxItems: 1 }
        },
        required: ["title", "visibility"]
      }
    }
  };
}

function propertyRequest(id: string, schema: Record<string, unknown>) {
  return {
    method: "elicitation/create" as const,
    params: { mode: "form" as const, message: "Input", requestedSchema: { type: "object" as const, properties: { [id]: schema } } }
  };
}
