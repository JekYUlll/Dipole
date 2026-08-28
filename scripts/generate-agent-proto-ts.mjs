import { execFileSync } from "node:child_process";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = process.env.DIPOLE_AGENT_PROTO_TS_OUTPUT === undefined
  ? resolve(root, "agent-runtime/src/generated")
  : resolve(process.env.DIPOLE_AGENT_PROTO_TS_OUTPUT);
execFileSync("protoc", [
  `--proto_path=${resolve(root, "api/proto")}`,
  `--plugin=protoc-gen-ts=${resolve(root, "agent-runtime/node_modules/.bin/protoc-gen-ts")}`,
  `--ts_out=${output}`,
  "--ts_opt=client_grpc1,long_type_bigint,ts_nocheck",
  resolve(root, "api/proto/dipole/common/v1/context.proto"),
  resolve(root, "api/proto/dipole/message/v1/message.proto"),
  "/usr/include/google/protobuf/timestamp.proto",
  resolve(root, "api/proto/dipole/agent/v1/agent.proto")
], { cwd: root, stdio: "inherit" });

for (const file of await sourceFiles(output)) {
  const source = await readFile(file, "utf8");
  const esm = source.replace(/from "(\.{1,2}\/[^\"]+)";/g, (_match, specifier) =>
    `from "${specifier.endsWith(".js") ? specifier : `${specifier}.js`}";`
  );
  await writeFile(file, esm);
}

async function sourceFiles(directory) {
  const result = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) result.push(...await sourceFiles(path));
    else if (entry.name.endsWith(".ts")) result.push(path);
  }
  return result;
}
