import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createAgentCapabilityRPC, loadShadowRuntimeConfig } from "../runtime/shadow-runtime.js";
import {
  PromotionEvidencePublisher, type PromotionEvidencePublicationInput, type PromotionEvidenceReceipt
} from "./promotion-evidence-publisher.js";

interface Writable {
  write(value: string): unknown;
}

interface PublisherHandle {
  publish(input: PromotionEvidencePublicationInput): Promise<PromotionEvidenceReceipt>;
  close(): void;
}

interface PromotionEvidencePublishCLIDependencies {
  openPublisher(): PublisherHandle;
}

export async function runPromotionEvidencePublishCLI(
  args: string[], stdout: Writable, stderr: Writable,
  dependencies: PromotionEvidencePublishCLIDependencies = defaultDependencies()
): Promise<number> {
  const inputArgs = args.filter(argument => argument.startsWith("--input="));
  if (args.length !== 1 || inputArgs.length !== 1 || inputArgs[0]!.slice("--input=".length).trim() === "") {
    stderr.write("promotion evidence publication requires exactly one --input=<path> argument\n");
    return 1;
  }
  let handle: PublisherHandle | undefined;
  try {
    const input = JSON.parse(await readFile(inputArgs[0]!.slice("--input=".length), "utf8")) as PromotionEvidencePublicationInput;
    handle = dependencies.openPublisher();
    const receipt = await handle.publish(input);
    stdout.write(`${JSON.stringify(receipt, null, 2)}\n`);
    return 0;
  } catch (error) {
    stderr.write(`promotion evidence publication failed closed: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  } finally {
    handle?.close();
  }
}

function defaultDependencies(): PromotionEvidencePublishCLIDependencies {
  return {
    openPublisher: () => {
      const config = loadShadowRuntimeConfig(process.env);
      if (!config.capabilityRpc.enabled) throw new Error("Agent Capability RPC must be enabled for evidence publication");
      const rpc = createAgentCapabilityRPC(config);
      const publisher = new PromotionEvidencePublisher(rpc.client);
      return { publish: input => publisher.publish(input), close: rpc.close };
    }
  };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runPromotionEvidencePublishCLI(process.argv.slice(2), process.stdout, process.stderr);
}
