import * as grpc from "@grpc/grpc-js";

import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";

type ClosableAgentCapabilityServiceClient = IAgentCapabilityServiceClient & { close(): void };

export type AgentCapabilityTransportFactory = () => ClosableAgentCapabilityServiceClient;

export interface ReconnectingAgentCapabilityTransport {
  readonly client: IAgentCapabilityServiceClient;
  close(): void;
}

// Kafka owns business-level retry. This wrapper only replaces a poisoned channel
// after an unavailable response, so the next idempotent consumer attempt re-resolves DNS.
export function createReconnectingAgentCapabilityTransport(
  createClient: AgentCapabilityTransportFactory
): ReconnectingAgentCapabilityTransport {
  let current: ClosableAgentCapabilityServiceClient = createClient();
  let closed = false;

  const reconnect = (attempt: ClosableAgentCapabilityServiceClient): void => {
    if (closed || current !== attempt) return;
    current.close();
    current = createClient();
  };

  const client = new Proxy({} as IAgentCapabilityServiceClient, {
    get(_target, property) {
      const transport = current;
      const member = Reflect.get(transport as object, property);
      if (typeof member !== "function") return member;
      return (...args: unknown[]) => {
        const callback = args.at(-1);
        if (typeof callback !== "function") {
          return Reflect.apply(member, transport, args);
        }
        const wrappedCallback = (error: grpc.ServiceError | null, ...result: unknown[]) => {
          if (error?.code === grpc.status.UNAVAILABLE) reconnect(transport);
          callback(error, ...result);
        };
        return Reflect.apply(member, transport, [...args.slice(0, -1), wrappedCallback]);
      };
    }
  });

  return {
    client,
    close: () => {
      if (closed) return;
      closed = true;
      current.close();
    }
  };
}
