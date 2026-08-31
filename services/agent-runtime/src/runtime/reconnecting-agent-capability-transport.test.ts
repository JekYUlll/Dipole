import { describe, expect, it, vi } from "vitest";
import * as grpc from "@grpc/grpc-js";

import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import { createReconnectingAgentCapabilityTransport } from "./reconnecting-agent-capability-transport.js";

describe("createReconnectingAgentCapabilityTransport", () => {
  it("replaces a stale transport after UNAVAILABLE without replaying the failed RPC", () => {
    const firstClose = vi.fn();
    const secondClose = vi.fn();
    const firstAdmit = vi.fn((_request, _metadata, _options, callback) => {
      callback({ code: grpc.status.UNAVAILABLE, message: "connection refused" });
      return {};
    });
    const secondAdmit = vi.fn((_request, _metadata, _options, callback) => {
      callback(null, { taskId: "TASK-1" });
      return {};
    });
    const create = vi.fn()
      .mockReturnValueOnce({ admitRun: firstAdmit, close: firstClose } as unknown as IAgentCapabilityServiceClient & { close(): void })
      .mockReturnValueOnce({ admitRun: secondAdmit, close: secondClose } as unknown as IAgentCapabilityServiceClient & { close(): void });
    const transport = createReconnectingAgentCapabilityTransport(create);
    const callback = vi.fn();

    transport.client.admitRun({} as never, {} as never, {} as never, callback);

    expect(firstAdmit).toHaveBeenCalledTimes(1);
    expect(callback).toHaveBeenCalledWith(expect.objectContaining({ code: grpc.status.UNAVAILABLE }));
    expect(firstClose).toHaveBeenCalledTimes(1);
    expect(create).toHaveBeenCalledTimes(2);
    expect(secondAdmit).not.toHaveBeenCalled();

    transport.client.admitRun({} as never, {} as never, {} as never, callback);

    expect(secondAdmit).toHaveBeenCalledTimes(1);
    expect(callback).toHaveBeenLastCalledWith(null, { taskId: "TASK-1" });
    transport.close();
    expect(secondClose).toHaveBeenCalledTimes(1);
  });

  it("keeps the transport for non-retryable RPC outcomes", () => {
    const close = vi.fn();
    const admitRun = vi.fn((_request, _metadata, _options, callback) => {
      callback({ code: grpc.status.PERMISSION_DENIED, message: "denied" });
      return {};
    });
    const create = vi.fn(() => ({ admitRun, close } as unknown as IAgentCapabilityServiceClient & { close(): void }));
    const transport = createReconnectingAgentCapabilityTransport(create);

    transport.client.admitRun({} as never, {} as never, {} as never, vi.fn());

    expect(create).toHaveBeenCalledTimes(1);
    expect(close).not.toHaveBeenCalled();
  });
});
