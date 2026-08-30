import { constants, createDecipheriv, createHash, privateDecrypt } from "node:crypto";

const version = "v1";
const nonceBytes = 12;
const tagBytes = 16;
const wrappedDataKeyMinimumBytes = 256;

export interface OAuthCallbackEnvelopeBinding {
  readonly handoffId: string;
  readonly transactionId: string;
  readonly ownerUserId: string;
  readonly issuer: string;
  readonly redirectUri: string;
  readonly authorizationCodeSHA256: string;
  readonly runtimeKeyId: string;
  readonly expiresAt: string;
}

/** Opens the Gateway-produced v1 envelope at the Runtime private-key boundary. */
export function openOAuthCallbackEnvelope(envelope: string, binding: OAuthCallbackEnvelopeBinding, runtimePrivateKeyPEM: string | Buffer): string {
  const trusted = validateBinding(binding);
  const parts = envelope.split(".");
  if (parts.length !== 5 || parts[0] !== version || parts.slice(1).some(part => !/^[A-Za-z0-9_-]+$/u.test(part))) {
    throw new Error("OAuth callback envelope is invalid");
  }
  let nonce: Buffer, ciphertext: Buffer, tag: Buffer, wrappedKey: Buffer, dataKey: Buffer | undefined, plaintext: Buffer | undefined;
  try {
    nonce = Buffer.from(parts[1]!, "base64url"); ciphertext = Buffer.from(parts[2]!, "base64url"); tag = Buffer.from(parts[3]!, "base64url"); wrappedKey = Buffer.from(parts[4]!, "base64url");
    if (nonce.length !== nonceBytes || ciphertext.length < 1 || ciphertext.length > 4096 || tag.length !== tagBytes || wrappedKey.length < wrappedDataKeyMinimumBytes || wrappedKey.length > 1024) throw new Error("invalid envelope");
    dataKey = privateDecrypt({ key: runtimePrivateKeyPEM, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, wrappedKey);
    if (dataKey.length !== 32) throw new Error("invalid data key");
    const decipher = createDecipheriv("aes-256-gcm", dataKey, nonce, { authTagLength: tagBytes });
    decipher.setAAD(Buffer.from(aad(trusted), "utf8"));
    decipher.setAuthTag(tag);
    plaintext = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
    const authorizationCode = plaintext.toString("utf8");
    if (plaintext.length === 0 || plaintext.length > 4096 || authorizationCode.includes("\0") ||
        createHash("sha256").update(plaintext).digest("hex") !== trusted.authorizationCodeSHA256) throw new Error("invalid authorization code");
    return authorizationCode;
  } catch {
    throw new Error("OAuth callback envelope is invalid");
  } finally {
    dataKey?.fill(0); plaintext?.fill(0);
  }
}

function validateBinding(value: OAuthCallbackEnvelopeBinding): OAuthCallbackEnvelopeBinding {
  for (const candidate of [value.handoffId, value.transactionId, value.ownerUserId, value.runtimeKeyId]) {
    if (typeof candidate !== "string" || candidate.length < 1 || candidate.length > 128 || candidate.trim() !== candidate || /[\r\n]/u.test(candidate)) throw new Error("OAuth callback envelope binding is invalid");
  }
  for (const candidate of [value.issuer, value.redirectUri]) {
    if (typeof candidate !== "string" || !candidate.startsWith("https://") || candidate.length > 2048 || /[?#@\r\n]/u.test(candidate)) throw new Error("OAuth callback envelope binding is invalid");
  }
  if (!/^[a-f0-9]{64}$/u.test(value.authorizationCodeSHA256) || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$/u.test(value.expiresAt) || new Date(value.expiresAt).toISOString() !== value.expiresAt) throw new Error("OAuth callback envelope binding is invalid");
  return Object.freeze({ ...value });
}

function aad(value: OAuthCallbackEnvelopeBinding): string {
  return ["dipole.agent.oauth-callback-handoff.v1", value.handoffId, value.transactionId, value.ownerUserId, value.issuer, value.redirectUri, value.authorizationCodeSHA256, value.runtimeKeyId, value.expiresAt].join("\n");
}
