export interface ActiveReadProfileSurface {
  readonly controlEnabled: boolean;
  readonly mcpServerEnabled: boolean;
  readonly externalMcpEnabled: boolean;
  readonly memoryEnabled: boolean;
  readonly retrievalEnabled: boolean;
  readonly retrievalContextEnabled: boolean;
  readonly subscriptionShadowEnabled: boolean;
}

export function assertActiveReadProfileSurface(
  runtimeMode: "shadow" | "active",
  surface: ActiveReadProfileSurface
): void {
  if (runtimeMode !== "active") return;
  const enabled = [
    ["MCP Server", surface.mcpServerEnabled],
    ["External MCP", surface.externalMcpEnabled],
    ["Memory", surface.memoryEnabled],
    ["retrieval", surface.retrievalEnabled],
    ["retrieval Context", surface.retrievalContextEnabled],
    ["subscription Shadow", surface.subscriptionShadowEnabled]
  ].filter(([, isEnabled]) => isEnabled).map(([name]) => name);
  if (enabled.length > 0) {
    throw new Error(`Active Agent Runtime read profile forbids: ${enabled.join(", ")}`);
  }
}
