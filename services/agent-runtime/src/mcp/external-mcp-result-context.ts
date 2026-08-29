import type { CallToolResult } from "@modelcontextprotocol/client";

import type { ContextFragment } from "../context/context-compiler.js";

export interface ExternalMcpResultContextInput {
  readonly profileId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly invocationId: string;
  readonly result: CallToolResult;
  readonly priority?: number;
  readonly maximumBytes?: number;
}

export function externalMcpResultToContextFragment(input: ExternalMcpResultContextInput): ContextFragment {
  assertBinding(input);
  if (input.result.isError === true) throw new Error("External MCP Context requires a successful Tool result");
  const maximumBytes = input.maximumBytes ?? 64 * 1024;
  if (!Number.isSafeInteger(maximumBytes) || maximumBytes < 1024 || maximumBytes > 128 * 1024) {
    throw new Error("External MCP Context size limit must be between 1 KiB and 128 KiB");
  }
  const priority = input.priority ?? 50;
  if (!Number.isSafeInteger(priority) || priority < -100 || priority > 100) {
    throw new Error("External MCP Context priority must be between -100 and 100");
  }

  let encoded: string;
  try {
    encoded = JSON.stringify(input.result);
  } catch {
    throw new Error("External MCP Context result must be valid JSON");
  }
  if (encoded === undefined || Buffer.byteLength(encoded, "utf8") > maximumBytes) {
    throw new Error("External MCP Context result exceeds its size limit");
  }
  const snapshot = JSON.parse(encoded) as CallToolResult;
  const contentTypes = [...new Set(snapshot.content.map(item => item.type))].sort();
  const sourceId = `${input.serverId}:${input.toolName}:${input.invocationId}`;

  return {
    id: `external-mcp:${input.invocationId}`,
    section: "evidence",
    trust: "untrusted",
    content: encoded,
    compactContent: JSON.stringify({
      externalMcpResult: true,
      serverId: input.serverId,
      toolName: input.toolName,
      contentTypes,
      hasStructuredContent: snapshot.structuredContent !== undefined
    }),
    priority,
    required: false,
    provenance: {
      sourceType: "external_mcp_tool",
      sourceId,
      uri: `dipole://external-mcp/${input.profileId}/${input.serverId}/${input.toolName}/${input.invocationId}`
    }
  };
}

function assertBinding(input: ExternalMcpResultContextInput): void {
  const identifier = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/;
  const toolName = /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/;
  const invocationId = /^[A-Z][A-Z0-9-]{15,127}$/;
  if (
    !identifier.test(input.profileId)
    || !identifier.test(input.serverId)
    || !toolName.test(input.toolName)
    || !invocationId.test(input.invocationId)
  ) {
    throw new Error("External MCP Context binding is invalid");
  }
}
