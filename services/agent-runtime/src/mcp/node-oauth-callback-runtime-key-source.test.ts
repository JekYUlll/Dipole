import { generateKeyPairSync } from "node:crypto";
import { chmod, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { createEncryptedFileOAuthCallbackRuntimeKeySource } from "./node-oauth-callback-runtime-key-source.js";

let directory = "";
let keyPath = "";

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), "dipole-oauth-key-"));
  keyPath = join(directory, "runtime-key.pem");
  const { privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
  await writeFile(keyPath, privateKey.export({ type: "pkcs8", format: "pem" }), { mode: 0o600 });
  await chmod(keyPath, 0o600);
});
afterAll(async () => { await rm(directory, { recursive: true, force: true }); });

describe("OAuth callback Runtime key source", () => {
  it("opens only an explicitly configured key for one callback", async () => {
    const source = createEncryptedFileOAuthCallbackRuntimeKeySource({ keys: { "oauth-runtime-2026-08": keyPath } });
    await expect(source.use("oauth-runtime-2026-08", key => key.length > 256 && key.subarray(0, 27).toString("ascii") === "-----BEGIN PRIVATE KEY-----")).resolves.toBe(true);
    await expect(source.use("unknown", () => true)).rejects.toThrow(/^OAuth callback Runtime key is unavailable$/);
  });

  it("rejects writable private-key files and malformed configuration", async () => {
    const source = createEncryptedFileOAuthCallbackRuntimeKeySource({ keys: { "oauth-runtime-2026-08": keyPath } });
    await chmod(keyPath, 0o644);
    await expect(source.use("oauth-runtime-2026-08", () => true)).rejects.toThrow(/^OAuth callback Runtime key is unavailable$/);
    await chmod(keyPath, 0o600);
    expect(() => createEncryptedFileOAuthCallbackRuntimeKeySource({ keys: { bad: "relative.pem" } })).toThrow(/configuration/i);
  });
});
