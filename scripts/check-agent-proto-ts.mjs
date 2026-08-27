import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const temporary = mkdtempSync(resolve(tmpdir(), "dipole-agent-proto-"));

try {
  execFileSync("node", [resolve(root, "scripts/generate-agent-proto-ts.mjs")], {
    cwd: root,
    env: { ...process.env, DIPOLE_AGENT_PROTO_TS_OUTPUT: temporary },
    stdio: "inherit"
  });
  execFileSync("diff", ["-ru", resolve(root, "agent-runtime/src/generated"), temporary], {
    cwd: root,
    stdio: "inherit"
  });
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
