import { buildServer } from "./server.js";
import { createKafkaShadowRuntime, loadShadowRuntimeConfig } from "./runtime/shadow-runtime.js";

const port = Number.parseInt(process.env.DIPOLE_AGENT_PORT ?? "8091", 10);
const host = process.env.DIPOLE_AGENT_HOST?.trim() || "0.0.0.0";
let ready = false;
const shadowConfig = loadShadowRuntimeConfig(process.env);
const shadowRuntime = shadowConfig.enabled ? createKafkaShadowRuntime(shadowConfig) : undefined;

const server = buildServer({ isReady: () => ready });

try {
  await server.listen({ host, port });
  if (shadowRuntime !== undefined) {
    await shadowRuntime.start();
  }
  ready = true;
} catch (error) {
  process.stderr.write(`${String(error)}\n`);
  await server.close();
  process.exitCode = 1;
}

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.once(signal, () => {
    ready = false;
    void (async () => {
      if (shadowRuntime !== undefined) {
        await shadowRuntime.stop();
      }
      await server.close();
    })();
  });
}
