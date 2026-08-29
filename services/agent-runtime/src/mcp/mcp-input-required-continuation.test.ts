import { describe, expect, it } from "vitest";

import { McpInputRequiredContinuation } from "./mcp-input-required-continuation.js";

describe("MCP input-required continuation", () => {
  const continuation = new McpInputRequiredContinuation(() => 1_000);

  it("turns one manual MRTR form round into a durable wait and exact retry", () => {
    const wait = continuation.begin({
      result: inputRequired(), requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", arguments: { calendarId: "CAL-1", timezone: "Asia/Shanghai" },
      expiresAtUnixMs: 2_000
    });

    expect(wait.directive).toMatchObject({
      kind: "wait_input", requestId: "INPUT-1",
      source: { kind: "mcp", serverId: "calendar.example", toolName: "calendar.create", invocationId: "INV-1", trust: "untrusted" },
      checkpoint: wait.checkpoint
    });
    expect(wait.checkpoint).toMatchObject({
      schemaVersion: "dipole.mcp.input-required-checkpoint.v1", inputRequestKey: "event-settings",
      requestState: "opaque-state-1", arguments: { calendarId: "CAL-1", timezone: "Asia/Shanghai" }
    });

    expect(continuation.retry(wait.checkpoint, {
      action: "accept", resume: {
        kind: "input", requestId: "INPUT-1",
        value: { title: "Release review", visibility: "team", labels: ["release"] }
      }
    })).toEqual({
      name: "calendar.create",
      arguments: { calendarId: "CAL-1", timezone: "Asia/Shanghai" },
      inputResponses: {
        "event-settings": {
          action: "accept", content: { title: "Release review", visibility: "team", labels: ["release"] }
        }
      },
      requestState: "opaque-state-1"
    });
  });

  it("supports decline and cancel while preserving the exact input key", () => {
    const checkpoint = continuation.begin({
      result: inputRequired(), requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", arguments: { calendarId: "CAL-1" }, expiresAtUnixMs: 2_000
    }).checkpoint;

    expect(continuation.retry(checkpoint, { action: "decline", requestId: "INPUT-1" }).inputResponses)
      .toEqual({ "event-settings": { action: "decline" } });
    expect(continuation.retry(checkpoint, { action: "cancel", requestId: "INPUT-1" }).inputResponses)
      .toEqual({ "event-settings": { action: "cancel" } });
  });

  it("rejects unsupported rounds and any checkpoint or argument drift", () => {
    const base = {
      requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", arguments: { calendarId: "CAL-1" }, expiresAtUnixMs: 2_000
    };
    expect(() => continuation.begin({ ...base, result: {
      ...inputRequired(), inputRequests: { ...inputRequired().inputRequests, second: formRequest() }
    } })).toThrow(/one.*input/i);
    expect(() => continuation.begin({ ...base, result: {
      resultType: "input_required", inputRequests: { sample: { method: "sampling/createMessage", params: {} } }
    } })).toThrow(/elicitation/i);
    expect(() => continuation.begin({ ...base, result: {
      resultType: "input_required", requestState: "opaque-only"
    } })).toThrow(/one.*input/i);

    const checkpoint = continuation.begin({ ...base, result: inputRequired() }).checkpoint;
    expect(() => continuation.retry({ ...checkpoint, requestState: "tampered" }, {
      action: "cancel", requestId: "INPUT-1"
    })).toThrow(/integrity/i);
    expect(() => continuation.retry({ ...checkpoint, arguments: { calendarId: "CAL-2" } }, {
      action: "cancel", requestId: "INPUT-1"
    })).toThrow(/integrity/i);
  });

  it("rejects credential-bearing arguments and oversized opaque state", () => {
    const base = {
      result: inputRequired(), requestId: "INPUT-1", serverId: "calendar.example", toolName: "calendar.create",
      invocationId: "INV-1", expiresAtUnixMs: 2_000
    };
    expect(() => continuation.begin({ ...base, arguments: { apiToken: "hidden" } })).toThrow(/credential/i);
    expect(() => continuation.begin({ ...base, arguments: { calendarId: "CAL-1" }, result: {
      ...inputRequired(), requestState: "x".repeat(8_193)
    } })).toThrow(/requestState/i);
  });
});

function inputRequired() {
  return {
    resultType: "input_required" as const,
    inputRequests: { "event-settings": formRequest() },
    requestState: "opaque-state-1"
  };
}

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
          labels: { type: "array" as const, title: "Labels", items: { type: "string" as const, enum: ["release", "incident"] }, maxItems: 1 }
        },
        required: ["title", "visibility"]
      }
    }
  };
}
