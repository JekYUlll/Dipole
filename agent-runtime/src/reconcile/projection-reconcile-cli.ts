import { Client, Connection } from "@temporalio/client";

import { createAgentCapabilityRPC, loadShadowRuntimeConfig } from "../runtime/shadow-runtime.js";
import { loadTemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import { TemporalTaskWorkflowInspector } from "../temporal/temporal-task-client.js";
import { AgentTaskProjectionReconciler } from "./agent-task-projection-reconciler.js";

const pageSize = argument("--page-size", 100);
const maxExamples = argument("--max-examples", 100);
const shadow = loadShadowRuntimeConfig(process.env);
const temporal = loadTemporalRuntimeConfig(process.env);
if (!shadow.capabilityRpc.enabled || !temporal.enabled) {
  throw new Error("Projection reconciliation requires Agent Capability RPC and Temporal");
}

const rpc = createAgentCapabilityRPC(shadow);
const connection = await Connection.connect({ address: temporal.address });
try {
  const client = new Client({ connection, namespace: temporal.namespace });
  const reconciler = new AgentTaskProjectionReconciler({
    list: (afterTaskId, limit) => rpc.client.listTaskWorkflowProjectionSnapshots(afterTaskId, limit)
  }, new TemporalTaskWorkflowInspector(client.workflow));
  const report = await reconciler.run({ pageSize, maxExamples });
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
  if (!report.consistent) process.exitCode = 2;
} finally {
  rpc.close();
  await connection.close();
}

function argument(name: string, fallback: number): number {
  const prefix = `${name}=`;
  const raw = process.argv.slice(2).find((value) => value.startsWith(prefix))?.slice(prefix.length);
  return raw === undefined ? fallback : Number(raw);
}
