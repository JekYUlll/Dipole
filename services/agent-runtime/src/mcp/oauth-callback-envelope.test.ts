import { constants, createCipheriv, createHash, generateKeyPairSync, publicEncrypt, randomBytes } from "node:crypto";
import { describe, expect, it } from "vitest";

import { openOAuthCallbackEnvelope } from "./oauth-callback-envelope.js";

const code = "oauth-code-0123456789";
const binding = Object.freeze({ handoffId: "aaaaaaaaaaaaaaaaaaaaaa", transactionId: "bbbbbbbbbbbbbbbbbbbbbb", ownerUserId: "U100", issuer: "https://auth.example.com", redirectUri: "https://dipole.example.com/oauth/callback", authorizationCodeSHA256: createHash("sha256").update(code, "utf8").digest("hex"), runtimeKeyId: "oauth-runtime-2026-08", expiresAt: "2026-08-31T00:10:00.000Z" });

describe("OAuth callback Runtime envelope", () => {
  it("opens a hybrid envelope and authenticates every handoff binding", () => {
    const { privateKey, publicKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
    const envelope = seal(code, binding, publicKey.export({ type: "spki", format: "pem" }).toString());
    expect(envelope).not.toContain(code);
    expect(openOAuthCallbackEnvelope(envelope, binding, privateKey.export({ type: "pkcs8", format: "pem" }).toString())).toBe(code);
    expect(() => openOAuthCallbackEnvelope(envelope, { ...binding, ownerUserId: "U200" }, privateKey.export({ type: "pkcs8", format: "pem" }).toString())).toThrow(/^OAuth callback envelope is invalid$/);
  });

  it("rejects malformed ciphertext and code digest mismatches", () => {
    const { privateKey, publicKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
    const privatePEM = privateKey.export({ type: "pkcs8", format: "pem" }).toString();
    expect(() => openOAuthCallbackEnvelope("v1.bad", binding, privatePEM)).toThrow(/^OAuth callback envelope is invalid$/);
    const envelope = seal(code, binding, publicKey.export({ type: "spki", format: "pem" }).toString());
    expect(() => openOAuthCallbackEnvelope(envelope, { ...binding, authorizationCodeSHA256: "a".repeat(64) }, privatePEM)).toThrow(/^OAuth callback envelope is invalid$/);
  });
});

function seal(authorizationCode: string, value: typeof binding, publicKeyPEM: string): string {
  const dataKey = randomBytes(32); const nonce = randomBytes(12);
  try {
    const cipher = createCipheriv("aes-256-gcm", dataKey, nonce, { authTagLength: 16 });
    cipher.setAAD(Buffer.from(aad(value), "utf8"));
    const ciphertext = Buffer.concat([cipher.update(authorizationCode, "utf8"), cipher.final()]);
    const wrapped = publicEncrypt({ key: publicKeyPEM, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, dataKey);
    return ["v1", nonce.toString("base64url"), ciphertext.toString("base64url"), cipher.getAuthTag().toString("base64url"), wrapped.toString("base64url")].join(".");
  } finally { dataKey.fill(0); }
}

function aad(value: typeof binding): string { return ["dipole.agent.oauth-callback-handoff.v1", value.handoffId, value.transactionId, value.ownerUserId, value.issuer, value.redirectUri, value.authorizationCodeSHA256, value.runtimeKeyId, value.expiresAt].join("\n"); }
