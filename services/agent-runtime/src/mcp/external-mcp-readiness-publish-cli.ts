import { pathToFileURL } from "node:url";

import { createAgentCapabilityRPC, loadShadowRuntimeConfig } from "../runtime/shadow-runtime.js";
import { loadExternalMcpConfig } from "./external-mcp-profile.js";
import { loadExternalMcpProductionIoManifest } from "./external-mcp-production-io-manifest.js";
import { createExternalMcpProductionIoRuntime } from "./external-mcp-production-io.js";
import {
  createExternalMcpReadinessPublication,
  type ExternalMcpReadinessPublication,
  type ExternalMcpReadinessPublicationInput,
  validateExternalMcpReadinessPublicationInput
} from "./external-mcp-readiness-publication.js";

interface Writable {
  write(value: string): unknown;
}

interface PublicationHandle {
  publish: ExternalMcpReadinessPublication;
  close(): void;
}

interface ExternalMcpReadinessPublishCLIDependencies {
  openPublication(): Promise<PublicationHandle>;
}

export async function runExternalMcpReadinessPublishCLI(
  args: string[],
  stdout: Writable,
  stderr: Writable,
  dependencies: ExternalMcpReadinessPublishCLIDependencies = defaultDependencies()
): Promise<number> {
  const input = parseInput(args);
  if (input === undefined) {
    stderr.write("external MCP readiness publication requires tenant, profile, validity, request and trace arguments\n");
    return 1;
  }
  let handle: PublicationHandle | undefined;
  try {
    handle = await dependencies.openPublication();
    const receipt = await handle.publish(input);
    stdout.write(`${JSON.stringify(receipt, null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("external MCP readiness publication failed closed\n");
    return 1;
  } finally {
    handle?.close();
  }
}

function parseInput(args: string[]): ExternalMcpReadinessPublicationInput | undefined {
  const names = ["tenant", "profile", "valid-for-seconds", "request-id", "trace-id"] as const;
  if (args.length !== names.length) return undefined;
  const values = new Map<string, string>();
  for (const argument of args) {
    const separator = argument.indexOf("=");
    if (!argument.startsWith("--") || separator < 3) return undefined;
    const name = argument.slice(2, separator);
    if (!names.includes(name as typeof names[number]) || values.has(name)) return undefined;
    values.set(name, argument.slice(separator + 1));
  }
  const seconds = Number(values.get("valid-for-seconds"));
  if (!Number.isSafeInteger(seconds)) return undefined;
  const input = {
    tenantId: values.get("tenant") ?? "",
    profileId: values.get("profile") ?? "",
    validForMs: seconds * 1_000,
    requestId: values.get("request-id") ?? "",
    traceId: values.get("trace-id") ?? ""
  };
  try {
    validateExternalMcpReadinessPublicationInput(input);
    return input;
  } catch {
    return undefined;
  }
}

function defaultDependencies(): ExternalMcpReadinessPublishCLIDependencies {
  return {
    openPublication: async () => {
      const externalConfig = loadExternalMcpConfig(process.env);
      if (!externalConfig.enabled) throw new Error("External MCP must be enabled");
      const loaded = await loadExternalMcpProductionIoManifest(externalConfig, process.env);
      if (loaded === undefined) throw new Error("External MCP production I/O must be configured");
      const shadowConfig = loadShadowRuntimeConfig(process.env);
      if (!shadowConfig.capabilityRpc.enabled) throw new Error("Agent Capability RPC must be enabled");
      const runtime = createExternalMcpProductionIoRuntime(externalConfig, loaded.io, loaded.options);
      const rpc = createAgentCapabilityRPC(shadowConfig);
      return {
        publish: createExternalMcpReadinessPublication({
          collect: runtime.readinessEvidence,
          publishMcpReadinessEvidence: (tenantId, evidence, expiresAt, context) =>
            rpc.client.publishMcpReadinessEvidence(tenantId, evidence, expiresAt, context)
        }),
        close: rpc.close
      };
    }
  };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runExternalMcpReadinessPublishCLI(process.argv.slice(2), process.stdout, process.stderr);
}
