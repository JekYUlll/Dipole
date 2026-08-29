import { readFile, writeFile } from "node:fs/promises";

const sourceUrl = new URL("../db/queries/agent_memory_derived_lineage.sql", import.meta.url);
const outputUrl = new URL("../services/agent-runtime/src/memory/mysql-memory-derived-lineage-queries.ts", import.meta.url);
const source = await readFile(sourceUrl, "utf8");
const entries = [...source.matchAll(/^-- name: (\w+) :\w+\n([\s\S]*?)(?=^-- name:|(?![\s\S]))/gm)];
if (entries.length !== 1) throw new Error(`expected 1 Agent Memory derived-lineage query, found ${entries.length}`);
const [, name, sql] = entries[0];
const constant = name.replaceAll(/([a-z0-9])([A-Z])/g, "$1_$2").toUpperCase();
await writeFile(outputUrl, `// Code generated from db/queries/agent_memory_derived_lineage.sql; DO NOT EDIT.\n\nexport const ${constant} = ${JSON.stringify(sql.trim().replace(/;$/, ""))};\n`);
