import { describe, expect, it } from "vitest";
import { OAuthCallbackHandoffExecutor } from "./oauth-callback-handoff-executor.js";

const input = { handoffId: "a".repeat(22), leaseOwner: "runtime-worker-1" };
const handoff = Object.freeze({ ...input, transactionId: "b".repeat(22), ownerUserId: "c".repeat(22), issuer: "https://auth.example.com/tenant", redirectUri: "https://dipole.example.com/oauth/callback", authorizationCodeSHA256: "d".repeat(64), sealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped-dek", runtimeKeyId: "runtime-key-1", expiresAt: new Date(Date.now() + 300_000), leaseExpiresAt: new Date(Date.now() + 30_000) });

describe("OAuthCallbackHandoffExecutor", () => {
  it("claims, opens with the recovered owner binding, then completes", async () => {
    const calls: string[] = [];
    const executor = build(calls, async () => "completed");
    await expect(executor.execute(input)).resolves.toBe("completed");
    expect(calls).toEqual(["claim", "key:runtime-key-1", `open:${handoff.ownerUserId}`, "process:code", "complete"]);
  });

  it("releases only pre-effect failures and declared retryable results", async () => {
    const retryCalls: string[] = [];
    await expect(build(retryCalls, async () => "retryable_failure").execute(input)).resolves.toBe("released");
    expect(retryCalls.at(-1)).toBe("release");
    const ambiguousCalls: string[] = [];
    await expect(build(ambiguousCalls, async () => { throw new Error("outcome unknown"); }).execute(input)).rejects.toThrow("outcome unknown");
    expect(ambiguousCalls).not.toContain("release");
  });

  it("keeps the lease when the terminal completion outcome is unavailable", async () => {
    const calls: string[] = [];
    const executor = build(calls, async () => "completed", { async complete() { calls.push("complete"); throw new Error("terminal unavailable"); } });

    await expect(executor.execute(input)).rejects.toThrow("terminal unavailable");
    expect(calls).not.toContain("release");
  });

  it("releases without opening an expired handoff", async () => {
    const calls: string[] = [];
    const expired = Object.freeze({ ...handoff, leaseExpiresAt: new Date(Date.now() - 1) });
    const executor = build(calls, async () => "completed", undefined, expired);

    await expect(executor.execute(input)).rejects.toThrow("lease expired");
    expect(calls).toEqual(["claim", "release"]);
  });

  it("releases if the lease expires while opening the envelope", async () => {
    const calls: string[] = [];
    const times = [new Date(Date.now()), new Date(Date.now() + 60_000)];
    const executor = build(calls, async () => "completed", undefined, handoff, () => times.shift()!);

    await expect(executor.execute(input)).rejects.toThrow("lease expired");
    expect(calls).toEqual(["claim", "key:runtime-key-1", `open:${handoff.ownerUserId}`, "release"]);
  });
});

function build(
  calls: string[], process: () => Promise<"completed" | "retryable_failure">,
  terminalOverride?: Partial<{ complete(): Promise<void>; release(): Promise<void> }>,
  claimed = handoff,
  now?: () => Date
): OAuthCallbackHandoffExecutor {
  const claims = { async claim() { calls.push("claim"); return claimed; } };
  const terminal = { async complete() { calls.push("complete"); }, async release() { calls.push("release"); }, ...terminalOverride };
  const keys = { async use<T>(id: string, operation: (key: Buffer) => Promise<T> | T): Promise<T> { calls.push(`key:${id}`); return operation(Buffer.from("key")); } };
  return new OAuthCallbackHandoffExecutor(claims as never, terminal as never, keys, { async process({ authorizationCode }) { calls.push(`process:${authorizationCode}`); return process(); } }, (_envelope, bound) => { calls.push(`open:${bound.ownerUserId}`); return "code"; }, now);
}
