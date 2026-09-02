import { createCipheriv, createDecipheriv, createHash, randomBytes as nodeRandomBytes, timingSafeEqual } from "node:crypto";

const transactionVersion = "v1";
const transactionTTLMilliseconds = 10 * 60 * 1000;

export interface OAuthAuthorizationTransactionInput {
  readonly ownerUserId: string;
  readonly issuer: string;
  readonly redirectUri: string;
  readonly state: string;
  readonly codeVerifier: string;
  readonly encryptionKey: Uint8Array;
  readonly now?: Date;
  readonly randomBytes?: (size: number) => Buffer;
}

/**
 * This is the low-sensitivity record intended for a Core-owned SQLC store.
 * It contains no plaintext state, verifier, authorization code, or token.
 */
export interface OAuthAuthorizationTransaction {
  readonly transactionId: string;
  readonly ownerUserId: string;
  readonly issuer: string;
  readonly redirectUri: string;
  readonly stateSHA256: string;
  readonly sealedCodeVerifier: string;
  readonly expiresAt: string;
}

export interface OAuthAuthorizationTransactionStore {
  /** Atomically consumes an unexpired record matching owner and state digest. */
  consume(input: {
    readonly transactionId: string;
    readonly ownerUserId: string;
    readonly stateSHA256: string;
    readonly now: Date;
  }): Promise<OAuthAuthorizationTransaction | undefined>;
}

export interface ConsumedOAuthAuthorizationTransaction {
  readonly transaction: OAuthAuthorizationTransaction;
  readonly codeVerifier: string;
}

export function createOAuthAuthorizationTransaction(input: OAuthAuthorizationTransactionInput): OAuthAuthorizationTransaction {
  const ownerUserId = requiredIdentifier(input.ownerUserId, "owner user ID");
  const issuer = canonicalHttpsURL(input.issuer, "issuer");
  const redirectUri = canonicalHttpsURL(input.redirectUri, "redirect URI");
  const state = requiredOpaqueToken(input.state, "state");
  const codeVerifier = requiredOpaqueToken(input.codeVerifier, "code verifier");
  const now = canonicalNow(input.now ?? new Date());
  const random = input.randomBytes ?? nodeRandomBytes;
  const key = validatedKey(input.encryptionKey);
  const transactionId = randomToken(random, 16, "transaction ID");
  const stateSHA256 = digest(state);
  const expiresAt = new Date(now.getTime() + transactionTTLMilliseconds).toISOString();
  const binding = Object.freeze({ transactionId, ownerUserId, issuer, redirectUri, stateSHA256, expiresAt });
  const sealedCodeVerifier = seal(codeVerifier, binding, key, random);
  key.fill(0);
  return Object.freeze({ ...binding, sealedCodeVerifier });
}

/**
 * The store performs conditional consumption before this method is called.
 * Decryption re-checks every immutable binding as defense in depth.
 */
export function openConsumedOAuthAuthorizationTransaction(
  transaction: OAuthAuthorizationTransaction,
  ownerUserId: string,
  presentedState: string,
  encryptionKey: Uint8Array,
  now: Date = new Date()
): ConsumedOAuthAuthorizationTransaction {
  const trusted = validateTransaction(transaction);
  if (trusted.ownerUserId !== requiredIdentifier(ownerUserId, "owner user ID")) throw new Error("OAuth authorization transaction owner is invalid");
  if (canonicalNow(now).getTime() >= new Date(trusted.expiresAt).getTime()) throw new Error("OAuth authorization transaction has expired");
  const stateDigest = digest(requiredOpaqueToken(presentedState, "state"));
  if (!safeEqual(trusted.stateSHA256, stateDigest)) throw new Error("OAuth authorization transaction state is invalid");
  const key = validatedKey(encryptionKey);
  const codeVerifier = unseal(trusted.sealedCodeVerifier, trusted, key);
  key.fill(0);
  return Object.freeze({ transaction: trusted, codeVerifier });
}

export function oauthAuthorizationStateSHA256(state: string): string {
  return digest(requiredOpaqueToken(state, "state"));
}

function seal(value: string, binding: Omit<OAuthAuthorizationTransaction, "sealedCodeVerifier">, key: Buffer, random: (size: number) => Buffer): string {
  const nonce = randomBytes(random, 12, "encryption nonce");
  const cipher = createCipheriv("aes-256-gcm", key, nonce);
  cipher.setAAD(Buffer.from(aad(binding), "utf8"));
  const ciphertext = Buffer.concat([cipher.update(value, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return `${transactionVersion}.${nonce.toString("base64url")}.${ciphertext.toString("base64url")}.${tag.toString("base64url")}`;
}

function unseal(sealed: string, binding: OAuthAuthorizationTransaction, key: Buffer): string {
  const parts = sealed.split(".");
  if (parts.length !== 4 || parts[0] !== transactionVersion || parts.slice(1).some(value => !/^[A-Za-z0-9_-]+$/u.test(value))) {
    throw new Error("OAuth authorization transaction verifier is invalid");
  }
  let nonce: Buffer, ciphertext: Buffer, tag: Buffer;
  try {
    nonce = Buffer.from(parts[1]!, "base64url"); ciphertext = Buffer.from(parts[2]!, "base64url"); tag = Buffer.from(parts[3]!, "base64url");
  } catch { throw new Error("OAuth authorization transaction verifier is invalid"); }
  if (nonce.length !== 12 || ciphertext.length === 0 || tag.length !== 16) throw new Error("OAuth authorization transaction verifier is invalid");
  try {
    const decipher = createDecipheriv("aes-256-gcm", key, nonce);
    decipher.setAAD(Buffer.from(aad(binding), "utf8"));
    decipher.setAuthTag(tag);
    return requiredOpaqueToken(Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString("utf8"), "code verifier");
  } catch { throw new Error("OAuth authorization transaction verifier is invalid"); }
}

function validateTransaction(transaction: OAuthAuthorizationTransaction): OAuthAuthorizationTransaction {
  const transactionId = requiredOpaqueToken(transaction.transactionId, "transaction ID");
  const ownerUserId = requiredIdentifier(transaction.ownerUserId, "owner user ID");
  const issuer = canonicalHttpsURL(transaction.issuer, "issuer");
  const redirectUri = canonicalHttpsURL(transaction.redirectUri, "redirect URI");
  if (!/^[a-f0-9]{64}$/u.test(transaction.stateSHA256)) throw new Error("OAuth authorization transaction state digest is invalid");
  const expiresAt = canonicalNow(new Date(transaction.expiresAt)).toISOString();
  if (transaction.expiresAt !== expiresAt) throw new Error("OAuth authorization transaction expiry is invalid");
  return Object.freeze({ transactionId, ownerUserId, issuer, redirectUri, stateSHA256: transaction.stateSHA256, sealedCodeVerifier: transaction.sealedCodeVerifier, expiresAt });
}

function aad(binding: Omit<OAuthAuthorizationTransaction, "sealedCodeVerifier"> | OAuthAuthorizationTransaction): string {
  return [transactionVersion, binding.transactionId, binding.ownerUserId, binding.issuer, binding.redirectUri, binding.stateSHA256, binding.expiresAt].join("\n");
}

function canonicalHttpsURL(raw: string, label: string): string {
  let url: URL;
  try { url = new URL(raw); } catch { throw new Error(`OAuth ${label} must be a canonical HTTPS URL`); }
  if (url.protocol !== "https:" || url.username || url.password || url.search || url.hash) {
    throw new Error(`OAuth ${label} must be a canonical HTTPS URL`);
  }
  return url.toString();
}

function canonicalNow(value: Date): Date {
  if (!(value instanceof Date) || !Number.isFinite(value.getTime())) throw new Error("OAuth authorization transaction time is invalid");
  return new Date(value.getTime());
}

function validatedKey(value: Uint8Array): Buffer {
  if (!(value instanceof Uint8Array) || value.byteLength !== 32) throw new Error("OAuth authorization encryption key is invalid");
  return Buffer.from(value);
}

function randomToken(random: (size: number) => Buffer, size: number, label: string): string { return randomBytes(random, size, label).toString("base64url"); }
function randomBytes(random: (size: number) => Buffer, size: number, label: string): Buffer {
  const bytes = random(size);
  if (!Buffer.isBuffer(bytes) || bytes.length !== size) throw new Error(`OAuth ${label} generator returned invalid bytes`);
  return bytes;
}
function digest(value: string): string { return createHash("sha256").update(value, "utf8").digest("hex"); }
function safeEqual(left: string, right: string): boolean { return left.length === right.length && timingSafeEqual(Buffer.from(left, "ascii"), Buffer.from(right, "ascii")); }
function requiredOpaqueToken(value: string, label: string): string {
  if (typeof value !== "string" || !/^[A-Za-z0-9_-]{16,512}$/u.test(value)) throw new Error(`OAuth ${label} is invalid`);
  return value;
}
function requiredIdentifier(value: string, label: string): string {
  if (typeof value !== "string" || value.trim() !== value || value.length < 1 || value.length > 255) throw new Error(`OAuth ${label} is invalid`);
  return value;
}
