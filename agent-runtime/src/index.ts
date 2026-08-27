import { buildServer } from "./server.js";

const port = Number.parseInt(process.env.DIPOLE_AGENT_PORT ?? "8091", 10);
const host = process.env.DIPOLE_AGENT_HOST?.trim() || "0.0.0.0";
let ready = false;

const server = buildServer({ isReady: () => ready });

try {
  await server.listen({ host, port });
  ready = true;
} catch (error) {
  server.log.error(error);
  process.exitCode = 1;
}

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.once(signal, () => {
    ready = false;
    void server.close();
  });
}
