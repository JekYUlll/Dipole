import { createCipheriv, randomBytes } from "node:crypto";
import { chmod, link, mkdtemp, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";
import {
  createEncryptedFileExternalMcpSecretProvider,
  externalMcpEncryptedSecretEnvelopeMagic
} from "./node-external-mcp-encrypted-secret-provider.js";

const binding: ExternalMcpCredentialBinding = {
  tenantId: "dipole",
  credentialRef: "CRED-0123456789ABCDEF",
  credentialVersion: 3,
  providerId: "local-aes-gcm",
  providerSecretRef: "SECRET-0123456789ABCDEF"
};
const keyRef = "KEY-0123456789ABCDEF";
let directory = "";
let keyPath = "";
let envelopePath = "";
let key = Buffer.alloc(0);

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), "dipole-mcp-secret-"));
  keyPath = join(directory, "key.bin");
  envelopePath = join(directory, "secret.bin");
  key = randomBytes(32);
  await writeFile(keyPath, key, { mode: 0o600 });
  await writeEnvelope(envelopePath, "token-first", key, binding, keyRef);
});

afterAll(async () => {
  key.fill(0);
  await rm(directory, { recursive: true, force: true });
});

describe("Node external MCP encrypted Secret Provider", () => {
  it("decrypts fresh bytes and observes atomic secret rotation on every read", async () => {
    const livePath = join(directory, "live-secret.bin");
    await writeFile(livePath, await readFile(envelopePath), { mode: 0o600 });
    const provider = providerFor(livePath, keyPath);
    const first = await provider.read(binding, new AbortController().signal);
    expect(Buffer.from(first).toString("utf8")).toBe("token-first");

    await writeEnvelope(livePath, "token-second", key, binding, keyRef);
    const second = await provider.read(binding, new AbortController().signal);
    expect(Buffer.from(second).toString("utf8")).toBe("token-second");
    expect(first).not.toBe(second);
    first.fill(0);
    second.fill(0);
  });

  it("supports versioned key rotation and removes old secret refs without fallback", async () => {
    const nextBinding: ExternalMcpCredentialBinding = {
      ...binding,
      credentialVersion: 4,
      providerSecretRef: "SECRET-FEDCBA9876543210"
    };
    const nextKeyRef = "KEY-FEDCBA9876543210";
    const nextKey = randomBytes(32);
    const nextKeyPath = join(directory, "next-key.bin");
    const nextEnvelopePath = join(directory, "next-secret.bin");
    await writeFile(nextKeyPath, nextKey, { mode: 0o600 });
    await writeEnvelope(nextEnvelopePath, "token-v4", nextKey, nextBinding, nextKeyRef);

    const rotating = createEncryptedFileExternalMcpSecretProvider({
      providerId: binding.providerId,
      keys: { [keyRef]: keyPath, [nextKeyRef]: nextKeyPath },
      secrets: {
        [binding.providerSecretRef]: { keyRef, path: envelopePath },
        [nextBinding.providerSecretRef]: { keyRef: nextKeyRef, path: nextEnvelopePath }
      }
    });
    await expect(readText(rotating, binding)).resolves.toBe("token-first");
    await expect(readText(rotating, nextBinding)).resolves.toBe("token-v4");

    const afterRevocation = createEncryptedFileExternalMcpSecretProvider({
      providerId: binding.providerId,
      keys: { [nextKeyRef]: nextKeyPath },
      secrets: { [nextBinding.providerSecretRef]: { keyRef: nextKeyRef, path: nextEnvelopePath } }
    });
    await expect(afterRevocation.read(binding, new AbortController().signal))
      .rejects.toThrow(/^External MCP encrypted secret is unavailable$/);
    nextKey.fill(0);
  });

  it.each([
    ["tenant", { tenantId: "other" }],
    ["credential", { credentialRef: "CRED-FEDCBA9876543210" }],
    ["version", { credentialVersion: 4 }],
    ["provider", { providerId: "other-provider" }]
  ])("authenticates the complete %s binding as AAD", async (_name, override) => {
    const provider = providerFor(envelopePath, keyPath);
    const changed = { ...binding, ...override };
    const error = await provider.read(changed, new AbortController().signal).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(Error);
    expect(String(error)).not.toContain(changed.tenantId);
    expect(String(error)).not.toContain(changed.providerSecretRef);
  });

  it("rejects corrupted, truncated and wrong-key envelopes without leaking cryptographic details", async () => {
    const corruptedPath = join(directory, "corrupted.bin");
    const corrupted = Buffer.from(await readFile(envelopePath));
    corrupted[corrupted.length - 1] = corrupted[corrupted.length - 1]! ^ 0xff;
    await writeFile(corruptedPath, corrupted, { mode: 0o600 });

    const truncatedPath = join(directory, "truncated.bin");
    await writeFile(truncatedPath, externalMcpEncryptedSecretEnvelopeMagic, { mode: 0o600 });

    const wrongKeyPath = join(directory, "wrong-key.bin");
    await writeFile(wrongKeyPath, randomBytes(32), { mode: 0o600 });

    for (const provider of [
      providerFor(corruptedPath, keyPath),
      providerFor(truncatedPath, keyPath),
      providerFor(envelopePath, wrongKeyPath)
    ]) {
      await expect(provider.read(binding, new AbortController().signal))
        .rejects.toThrow(/^External MCP encrypted secret is unavailable$/);
    }
  });

  it("rejects symlinks, hard links and unsafe key or envelope permissions", async () => {
    const keyLink = join(directory, "key-link.bin");
    await symlink(keyPath, keyLink);
    await expect(providerFor(envelopePath, keyLink).read(binding, new AbortController().signal)).rejects.toThrow();
    await rm(keyLink);

    const envelopeLink = join(directory, "envelope-link.bin");
    await link(envelopePath, envelopeLink);
    await expect(providerFor(envelopeLink, keyPath).read(binding, new AbortController().signal)).rejects.toThrow();
    await rm(envelopeLink);

    const publicKeyPath = join(directory, "public-key.bin");
    await writeFile(publicKeyPath, key, { mode: 0o644 });
    await chmod(publicKeyPath, 0o644);
    await expect(providerFor(envelopePath, publicKeyPath).read(binding, new AbortController().signal)).rejects.toThrow();

    const writableEnvelopePath = join(directory, "writable-secret.bin");
    await writeFile(writableEnvelopePath, await readFile(envelopePath), { mode: 0o666 });
    await chmod(writableEnvelopePath, 0o666);
    await expect(providerFor(writableEnvelopePath, keyPath).read(binding, new AbortController().signal)).rejects.toThrow();
  });

  it("does not fall back for unknown secret refs and honors pre-cancellation", async () => {
    const provider = providerFor(envelopePath, keyPath);
    await expect(provider.read({
      ...binding, providerSecretRef: "SECRET-FEDCBA9876543210"
    }, new AbortController().signal)).rejects.toThrow(/^External MCP encrypted secret is unavailable$/);

    const controller = new AbortController();
    controller.abort(new Error("cancelled before secret access"));
    await expect(provider.read(binding, controller.signal)).rejects.toThrow(/cancelled before secret access/i);
  });

  it("rejects invalid or aliased provider configuration before file access", () => {
    expect(() => createEncryptedFileExternalMcpSecretProvider({
      providerId: binding.providerId,
      keys: { [keyRef]: keyPath },
      secrets: { [binding.providerSecretRef]: { keyRef: "KEY-FEDCBA9876543210", path: envelopePath } }
    })).toThrow(/configuration/i);
    expect(() => createEncryptedFileExternalMcpSecretProvider({
      providerId: binding.providerId,
      keys: { [keyRef]: keyPath },
      secrets: { [binding.providerSecretRef]: { keyRef, path: keyPath } }
    })).toThrow(/configuration/i);
    expect(() => createEncryptedFileExternalMcpSecretProvider({
      providerId: binding.providerId,
      keys: { [keyRef]: "relative-key.bin" },
      secrets: { [binding.providerSecretRef]: { keyRef, path: envelopePath } }
    })).toThrow(/configuration/i);
    expect(() => createEncryptedFileExternalMcpSecretProvider({
      providerId: binding.providerId,
      keys: { [keyRef]: keyPath },
      secrets: { [binding.providerSecretRef]: { keyRef, path: envelopePath } }
    }, { maximumSecretBytes: 8193 })).toThrow(/maximum bytes/i);
  });
});

function providerFor(secretPath: string, secretKeyPath: string) {
  return createEncryptedFileExternalMcpSecretProvider({
    providerId: binding.providerId,
    keys: { [keyRef]: secretKeyPath },
    secrets: { [binding.providerSecretRef]: { keyRef, path: secretPath } }
  });
}

async function readText(
  provider: ReturnType<typeof createEncryptedFileExternalMcpSecretProvider>,
  credential: ExternalMcpCredentialBinding
): Promise<string> {
  const bytes = await provider.read(credential, new AbortController().signal);
  try {
    return Buffer.from(bytes).toString("utf8");
  } finally {
    bytes.fill(0);
  }
}

async function writeEnvelope(
  path: string,
  token: string,
  encryptionKey: Uint8Array,
  credential: ExternalMcpCredentialBinding,
  encryptionKeyRef: string
): Promise<void> {
  const nonce = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", encryptionKey, nonce, { authTagLength: 16 });
  cipher.setAAD(bindingAad(credential, encryptionKeyRef));
  const ciphertext = Buffer.concat([cipher.update(token, "utf8"), cipher.final()]);
  const envelope = Buffer.concat([
    externalMcpEncryptedSecretEnvelopeMagic,
    nonce,
    ciphertext,
    cipher.getAuthTag()
  ]);
  await writeFile(path, envelope, { mode: 0o600 });
  await chmod(path, 0o600);
}

function bindingAad(credential: ExternalMcpCredentialBinding, encryptionKeyRef: string): Buffer {
  return Buffer.from([
    "dipole.agent.external-mcp-secret.v1",
    credential.tenantId,
    credential.credentialRef,
    String(credential.credentialVersion),
    credential.providerId,
    credential.providerSecretRef,
    encryptionKeyRef
  ].join("\0"), "utf8");
}
