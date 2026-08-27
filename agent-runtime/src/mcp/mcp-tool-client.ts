import { Client, type CallToolResult, type Tool, type Transport } from "@modelcontextprotocol/client";

export class AllowlistedMcpToolClient {
  readonly #client: Client;
  readonly #allowedTools: ReadonlySet<string>;
  readonly #serverId: string;
  #discovered = new Set<string>();

  constructor(serverId: string, allowedServerIds: readonly string[], allowedTools: readonly string[]) {
    const normalizedServerId = serverId.trim();
    if (!allowedServerIds.includes(normalizedServerId)) throw new Error(`MCP Server ${normalizedServerId} is not allowlisted`);
    const normalizedTools = allowedTools.map((name) => name.trim());
    if (normalizedTools.length === 0 || normalizedTools.some((name) => !/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(name)) || new Set(normalizedTools).size !== normalizedTools.length) {
      throw new Error("MCP Tool allowlist is empty, invalid, or duplicated");
    }
    this.#allowedTools = new Set(normalizedTools);
    this.#serverId = normalizedServerId;
    this.#client = new Client({ name: "dipole-agent", version: "0.1.0" });
  }

  async connect(transport: Transport): Promise<readonly Tool[]> {
    await this.#client.connect(transport);
    const server = this.#client.getServerVersion();
    if (server?.name !== this.#serverId) {
      await this.#client.close();
      throw new Error(`MCP Server identity mismatch: expected ${this.#serverId}, received ${server?.name ?? "unknown"}`);
    }
    const listed = await this.#client.listTools();
    if (listed.tools.length > 256) {
      await this.#client.close();
      throw new Error("MCP Server advertised more than 256 Tools");
    }
    this.#discovered = new Set(listed.tools.filter((tool) => this.#allowedTools.has(tool.name)).map((tool) => tool.name));
    return listed.tools.filter((tool) => this.#discovered.has(tool.name));
  }

  async callTool(name: string, arguments_: Readonly<Record<string, unknown>>): Promise<CallToolResult> {
    if (!this.#allowedTools.has(name) || !this.#discovered.has(name)) throw new Error(`MCP Tool ${name} is not allowlisted and discovered`);
    const result = await this.#client.callTool({ name, arguments: arguments_ });
    const bytes = new TextEncoder().encode(JSON.stringify(result)).length;
    if (bytes > 128 * 1024) throw new Error(`MCP Tool ${name} response exceeds 128 KiB`);
    if (result.isError === true) throw new Error(`MCP Tool ${name} returned an error`);
    return result;
  }

  close(): Promise<void> {
    return this.#client.close();
  }
}
