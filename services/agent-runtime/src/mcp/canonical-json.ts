export function canonicalMcpJSON(value: unknown, ancestors = new Set<object>()): string {
  if (value === null) return "null";
  if (typeof value === "string" || typeof value === "boolean") return JSON.stringify(value);
  if (typeof value === "number") {
    if (!Number.isFinite(value)) throw new Error("MCP value must contain finite JSON numbers");
    return JSON.stringify(value);
  }
  if (typeof value !== "object") throw new Error("MCP value must be JSON serializable");
  if (ancestors.has(value)) throw new Error("MCP value cannot contain cycles");
  ancestors.add(value);
  try {
    if (Array.isArray(value)) return `[${value.map(item => canonicalMcpJSON(item, ancestors)).join(",")}]`;
    const prototype = Object.getPrototypeOf(value);
    if (prototype !== Object.prototype && prototype !== null) throw new Error("MCP value requires plain JSON objects");
    return `{${Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) =>
      `${JSON.stringify(key)}:${canonicalMcpJSON(item, ancestors)}`
    ).join(",")}}`;
  } finally {
    ancestors.delete(value);
  }
}
