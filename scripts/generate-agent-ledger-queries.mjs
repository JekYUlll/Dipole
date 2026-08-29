import { readFile, writeFile } from "node:fs/promises";

const sourceUrl = new URL("../db/queries/agent_event_ledger.sql", import.meta.url);
const outputUrl = new URL("../services/agent-runtime/src/events/mysql-event-ledger-queries.ts", import.meta.url);
const source = await readFile(sourceUrl, "utf8");
const entries = [...source.matchAll(/^-- name: (\w+) :\w+\n([\s\S]*?)(?=^-- name:|(?![\s\S]))/gm)];

if (entries.length !== 6) {
  throw new Error(`expected 6 Agent EventLedger queries, found ${entries.length}`);
}

const output = [
  "// Code generated from db/queries/agent_event_ledger.sql; DO NOT EDIT.",
  ""
];
for (const [, name, sql] of entries) {
  const constant = name.replaceAll(/([a-z0-9])([A-Z])/g, "$1_$2").toUpperCase();
  output.push(`export const ${constant} = ${JSON.stringify(sql.trim().replace(/;$/, ""))};`, "");
}
await writeFile(outputUrl, `${output.join("\n").trimEnd()}\n`);
