#!/usr/bin/env node

const baseURL = new URL(required("DIPOLE_MCP_BASE_URL"));
const password = process.env.DIPOLE_MCP_SMOKE_PASSWORD ?? "mcp-smoke-pass-123";
const telephone = `139${String(Date.now()).slice(-8)}`;

const registered = await request("/api/v1/auth/register", {
  method: "POST",
  body: { nickname: "MCP Smoke", telephone, password }
});
if (registered.response.status !== 200) fail(`registration returned ${registered.response.status}`);

const login = await request("/api/v1/auth/login", {
  method: "POST",
  body: { telephone, password }
});
const sessionToken = login.body?.data?.token;
if (login.response.status !== 200 || typeof sessionToken !== "string") fail(`login returned ${login.response.status}`);
const sessionHeaders = { authorization: `Bearer ${sessionToken}` };

const definition = await request("/api/v1/agent/definitions", { method: "POST", headers: sessionHeaders });
if (definition.response.status !== 201) fail(`Definition create returned ${definition.response.status}`);

const task = await request("/api/v1/agent/tasks", {
  method: "POST", headers: sessionHeaders,
  body: { client_request_id: `mcp-${Date.now()}`, goal: "List my conversations" }
});
const taskID = task.body?.taskId;
if (task.response.status !== 202 || typeof taskID !== "string") fail(`Task create returned ${task.response.status}`);

const runID = await waitForMCPRun(taskID, sessionHeaders);
const grant = await request("/api/v1/auth/agent-mcp/token", {
  method: "POST", headers: sessionHeaders,
  body: { resource: "https://dipole.local/api/v1/agent/mcp", scopes: ["dipole.agent.mcp.read"], consent: true }
});
const mcpToken = grant.body?.data?.access_token;
if (grant.response.status !== 200 || typeof mcpToken !== "string") fail(`MCP consent returned ${grant.response.status}`);

const endpoint = `/api/v1/agent/tasks/${encodeURIComponent(taskID)}/runs/${encodeURIComponent(runID)}/mcp`;
let sessionID = "";
const initialize = await rpc(endpoint, mcpToken, 1, "initialize", {
  protocolVersion: "2025-03-26", capabilities: {}, clientInfo: { name: "dipole-mcp-smoke", version: "1.0.0" }
});
await notify(endpoint, mcpToken, "notifications/initialized", sessionID);
const tools = await rpc(endpoint, mcpToken, 2, "tools/list");
if (!tools?.tools?.some((tool) => tool.name === "dipole_conversation_list")) fail("tools/list did not expose dipole_conversation_list");
const toolResult = await rpc(endpoint, mcpToken, 3, "tools/call", { name: "dipole_conversation_list", arguments: { limit: 20 } });

console.log(JSON.stringify({
  taskAccepted: true,
  mcpRunBound: true,
  initialized: typeof initialize?.serverInfo?.name === "string",
  sessionBound: sessionID.length > 0,
  tools: tools.tools.map((tool) => tool.name),
  toolCall: "dipole_conversation_list",
  toolResult: Array.isArray(toolResult?.content) ? "received" : "missing"
}));

async function waitForMCPRun(taskID, headers) {
  for (let attempt = 0; attempt < 80; attempt += 1) {
    const state = await request(`/api/v1/agent/tasks/${encodeURIComponent(taskID)}`, { headers });
    if (state.response.status === 200 && typeof state.body?.mcpRunId === "string" && state.body.mcpRunId.length > 0) return state.body.mcpRunId;
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  fail("Task did not expose an executable mcpRunId");
}

async function rpc(path, token, id, method, params) {
  const result = await request(path, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, accept: "application/json, text/event-stream" },
    body: { jsonrpc: "2.0", id, method, ...(params === undefined ? {} : { params }) }
  });
  if (!result.response.ok) fail(`${method} returned ${result.response.status}`);
  sessionID ||= result.response.headers.get("mcp-session-id") ?? "";
  const body = result.body;
  if (body?.error !== undefined) fail(`${method} failed: ${body.error.message ?? "unknown error"}`);
  return body?.result;
}

async function notify(path, token, method, mcpSessionID) {
  const result = await request(path, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, accept: "application/json, text/event-stream", ...(mcpSessionID === "" ? {} : { "mcp-session-id": mcpSessionID }) },
    body: { jsonrpc: "2.0", method }
  });
  if (!result.response.ok) fail(`${method} returned ${result.response.status}`);
}

async function request(path, { method = "GET", headers = {}, body } = {}) {
  const response = await fetch(new URL(path, baseURL), {
    method,
    headers: { ...(body === undefined ? {} : { "content-type": "application/json" }), ...headers },
    ...(body === undefined ? {} : { body: JSON.stringify(body) })
  });
  const text = await response.text();
  return { response, body: parseResponse(text) };
}

function parseResponse(text) {
  const data = text.split(/\r?\n/).filter((line) => line.startsWith("data:")).map((line) => line.slice(5).trim()).join("");
  try { return JSON.parse(data || text); } catch { return undefined; }
}

function required(name) {
  const value = process.env[name]?.trim();
  if (value === undefined || value === "") fail(`${name} is required`);
  return value;
}

function fail(message) {
  throw new Error(`Agent MCP smoke: ${message}`);
}
