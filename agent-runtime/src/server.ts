import Fastify, { type FastifyInstance } from "fastify";

export interface RuntimeReadiness {
  isReady(): boolean;
}

export function buildServer(readiness: RuntimeReadiness): FastifyInstance {
  const server = Fastify({ logger: false });

  server.get("/livez", async () => ({ status: "ok", service: "dipole-agent" }));
  server.get("/readyz", async (_request, reply) => {
    if (!readiness.isReady()) {
      return reply.code(503).send({ status: "not_ready", service: "dipole-agent" });
    }
    return { status: "ready", service: "dipole-agent" };
  });

  return server;
}
