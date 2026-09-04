import { constants, createCipheriv, createDecipheriv, createHash, createPrivateKey, createPublicKey, privateDecrypt, publicEncrypt, randomBytes } from "node:crypto";

import type { TokenLifecycleBundle } from "./oauth-callback-token-lifecycle.js";

const version = "v1";
const nonceBytes = 12;
const tagBytes = 16;

export interface OAuthTokenLifecycleEnvelopeBinding {
  readonly handoffId: string;
  readonly runtimeKeyId: string;
  readonly state: "active" | "refreshed";
  readonly tokenBundleSHA256: string;
  readonly accessTokenExpiresAt: string;
  readonly scope: string;
}

export interface SealedOAuthTokenLifecycleBundle {
  readonly envelope: string;
  readonly sha256: string;
  readonly expiresAt: Date;
  readonly scope: string;
}

/**
 * Seals the canonical provider bundle for Core persistence. Core receives the
 * opaque envelope and digest only; the Runtime private-key boundary remains
 * the sole decryptor for later approved refresh work.
 */
export function sealOAuthTokenLifecycleBundle(
  bundle: TokenLifecycleBundle,
  binding: Omit<OAuthTokenLifecycleEnvelopeBinding, "tokenBundleSHA256" | "accessTokenExpiresAt" | "scope">,
  runtimePrivateKeyPEM: string | Buffer
): SealedOAuthTokenLifecycleBundle {
  const plaintext = canonicalBundle(bundle);
  const sha256 = createHash("sha256").update(plaintext).digest("hex");
  const expiresAt = canonicalDate(bundle.expiresAt);
  const scope = canonicalScope(bundle.scope ?? "");
  const trusted = validateBinding({ ...binding, tokenBundleSHA256: sha256, accessTokenExpiresAt: expiresAt.toISOString(), scope });
  let dataKey: Buffer | undefined;
  let plaintextBytes: Buffer | undefined;
  try {
    dataKey = randomBytes(32);
    plaintextBytes = Buffer.from(plaintext, "utf8");
    const nonce = randomBytes(nonceBytes);
    const cipher = createCipheriv("aes-256-gcm", dataKey, nonce, { authTagLength: tagBytes });
    cipher.setAAD(Buffer.from(aad(trusted), "utf8"));
    const ciphertext = Buffer.concat([cipher.update(plaintextBytes), cipher.final()]);
    const tag = cipher.getAuthTag();
    const publicKey = createPublicKey(createPrivateKey(runtimePrivateKeyPEM));
    const wrappedKey = publicEncrypt({ key: publicKey, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, dataKey);
    return Object.freeze({ envelope: `${version}.${nonce.toString("base64url")}.${ciphertext.toString("base64url")}.${tag.toString("base64url")}.${wrappedKey.toString("base64url")}`, sha256, expiresAt, scope });
  } finally {
    dataKey?.fill(0);
    plaintextBytes?.fill(0);
  }
}

/** Test and future refresh helper; Core must never call this function. */
export function openOAuthTokenLifecycleBundle(envelope: string, binding: OAuthTokenLifecycleEnvelopeBinding, runtimePrivateKeyPEM: string | Buffer): TokenLifecycleBundle {
  const trusted = validateBinding(binding);
  const parts = envelope.split(".");
  if (parts.length !== 5 || parts[0] !== version || parts.slice(1).some(part => !/^[A-Za-z0-9_-]+$/u.test(part))) throw new Error("OAuth token lifecycle envelope is invalid");
  let dataKey: Buffer | undefined;
  let plaintext: Buffer | undefined;
  try {
    const nonce = Buffer.from(parts[1]!, "base64url");
    const ciphertext = Buffer.from(parts[2]!, "base64url");
    const tag = Buffer.from(parts[3]!, "base64url");
    const wrappedKey = Buffer.from(parts[4]!, "base64url");
    if (nonce.length !== nonceBytes || ciphertext.length < 1 || ciphertext.length > 16384 || tag.length !== tagBytes || wrappedKey.length < 256 || wrappedKey.length > 1024) throw new Error("invalid envelope");
    dataKey = privateDecrypt({ key: runtimePrivateKeyPEM, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, wrappedKey);
    if (dataKey.length !== 32) throw new Error("invalid data key");
    const decipher = createDecipheriv("aes-256-gcm", dataKey, nonce, { authTagLength: tagBytes });
    decipher.setAAD(Buffer.from(aad(trusted), "utf8"));
    decipher.setAuthTag(tag);
    plaintext = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
    if (createHash("sha256").update(plaintext).digest("hex") !== trusted.tokenBundleSHA256) throw new Error("invalid digest");
    return parseBundle(plaintext.toString("utf8"), trusted);
  } catch {
    throw new Error("OAuth token lifecycle envelope is invalid");
  } finally {
    dataKey?.fill(0);
    plaintext?.fill(0);
  }
}

function canonicalBundle(bundle: TokenLifecycleBundle): string {
  if (typeof bundle.accessToken !== "string" || bundle.accessToken.length < 1 || bundle.accessToken.length > 8192 ||
      typeof bundle.tokenType !== "string" || bundle.tokenType.length < 1 || bundle.tokenType.length > 64 ||
      (bundle.refreshToken !== undefined && (typeof bundle.refreshToken !== "string" || bundle.refreshToken.length < 1 || bundle.refreshToken.length > 8192))) {
    throw new Error("OAuth token bundle is invalid");
  }
  const expiresAt = canonicalDate(bundle.expiresAt).toISOString();
  const scope = canonicalScope(bundle.scope ?? "");
  return JSON.stringify({ access_token: bundle.accessToken, expires_at: expiresAt, refresh_token: bundle.refreshToken ?? null, scope, token_type: bundle.tokenType });
}

function parseBundle(value: string, binding: OAuthTokenLifecycleEnvelopeBinding): TokenLifecycleBundle {
  let parsed: unknown;
  try { parsed = JSON.parse(value); } catch { throw new Error("invalid JSON"); }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) throw new Error("invalid bundle");
  const item = parsed as Record<string, unknown>;
  if (typeof item.access_token !== "string" || typeof item.token_type !== "string" || typeof item.expires_at !== "string" ||
      (item.refresh_token !== null && typeof item.refresh_token !== "string") || typeof item.scope !== "string") throw new Error("invalid bundle");
  const expiresAt = canonicalDate(new Date(item.expires_at));
  if (expiresAt.toISOString() !== binding.accessTokenExpiresAt || item.scope !== binding.scope) throw new Error("invalid binding");
  return Object.freeze({ accessToken: item.access_token, tokenType: item.token_type, expiresAt, scope: item.scope, ...(typeof item.refresh_token === "string" ? { refreshToken: item.refresh_token } : {}) });
}

function validateBinding(value: OAuthTokenLifecycleEnvelopeBinding): OAuthTokenLifecycleEnvelopeBinding {
  if (!/^[A-Za-z0-9_-]{16,64}$/u.test(value.handoffId) || !/^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/u.test(value.runtimeKeyId) ||
      (value.state !== "active" && value.state !== "refreshed") || !/^[a-f0-9]{64}$/u.test(value.tokenBundleSHA256)) throw new Error("OAuth token lifecycle binding is invalid");
  const expiresAt = canonicalDate(new Date(value.accessTokenExpiresAt));
  if (expiresAt.toISOString() !== value.accessTokenExpiresAt) throw new Error("OAuth token lifecycle binding is invalid");
  return Object.freeze({ ...value, scope: canonicalScope(value.scope) });
}

function canonicalScope(value: string): string {
  if (typeof value !== "string" || value.length > 2048 || value.trim() !== value || /[\r\n\0]/u.test(value)) throw new Error("OAuth token scope is invalid");
  return value;
}

function canonicalDate(value: Date): Date {
  if (!(value instanceof Date) || !Number.isFinite(value.getTime())) throw new Error("OAuth token expiry is invalid");
  return new Date(Math.trunc(value.getTime()));
}

function aad(value: OAuthTokenLifecycleEnvelopeBinding): string {
  return ["dipole.agent.oauth-token-lifecycle.v1", value.handoffId, value.runtimeKeyId, value.state, value.tokenBundleSHA256, value.accessTokenExpiresAt, value.scope].join("\n");
}
