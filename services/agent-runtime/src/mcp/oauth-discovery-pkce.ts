import { createHash, randomBytes as nodeRandomBytes } from "node:crypto";

export interface OAuthAuthorizationServerMetadata {
  readonly issuer: string;
  readonly authorizationEndpoint: string;
  readonly tokenEndpoint: string;
}

export interface PkceAuthorizationRequestInput {
  readonly metadata: OAuthAuthorizationServerMetadata;
  readonly clientId: string;
  readonly redirectUri: string;
  readonly scope: string;
  readonly randomBytes?: (size: number) => Buffer;
}

export interface PkceAuthorizationRequest {
  readonly url: URL;
  readonly codeVerifier: string;
  readonly state: string;
}

/** RFC 8414 places the well-known path before any issuer path component. */
export function authorizationServerMetadataURL(rawIssuer: string): string {
  const issuer = canonicalHttpsURL(rawIssuer, "issuer");
  const issuerPath = issuer.pathname === "/" ? "" : issuer.pathname;
  return `${issuer.origin}/.well-known/oauth-authorization-server${issuerPath}`;
}

export function parseAuthorizationServerMetadata(
  issuer: string,
  raw: unknown
): OAuthAuthorizationServerMetadata {
  const expectedIssuer = canonicalHttpsURL(issuer, "issuer").toString();
  if (!isRecord(raw)) throw new Error("OAuth authorization server metadata must be an object");

  const metadataIssuer = readString(raw, "issuer");
  if (metadataIssuer !== expectedIssuer) {
    throw new Error("OAuth authorization server metadata issuer does not exactly match the requested issuer");
  }
  const authorizationEndpoint = canonicalHttpsURL(readString(raw, "authorization_endpoint"), "authorization endpoint").toString();
  const tokenEndpoint = canonicalHttpsURL(readString(raw, "token_endpoint"), "token endpoint").toString();
  const methods = raw.code_challenge_methods_supported;
  if (!Array.isArray(methods) || !methods.every(method => typeof method === "string") || !methods.includes("S256")) {
    throw new Error("OAuth authorization server metadata must advertise the S256 PKCE method");
  }

  return { issuer: expectedIssuer, authorizationEndpoint, tokenEndpoint };
}

export function createPkceAuthorizationRequest(input: PkceAuthorizationRequestInput): PkceAuthorizationRequest {
  const authorizationEndpoint = canonicalHttpsURL(input.metadata.authorizationEndpoint, "authorization endpoint");
  const redirectURI = canonicalHttpsURL(input.redirectUri, "redirect URI");
  const clientId = required(input.clientId, "client ID");
  const scope = required(input.scope, "scope");
  const random = input.randomBytes ?? nodeRandomBytes;
  const codeVerifier = randomUrlToken(random, "PKCE verifier");
  const state = randomUrlToken(random, "OAuth state");
  const codeChallenge = createHash("sha256").update(codeVerifier, "ascii").digest("base64url");

  authorizationEndpoint.searchParams.set("response_type", "code");
  authorizationEndpoint.searchParams.set("client_id", clientId);
  authorizationEndpoint.searchParams.set("redirect_uri", redirectURI.toString());
  authorizationEndpoint.searchParams.set("scope", scope);
  authorizationEndpoint.searchParams.set("state", state);
  authorizationEndpoint.searchParams.set("code_challenge", codeChallenge);
  authorizationEndpoint.searchParams.set("code_challenge_method", "S256");
  return { url: authorizationEndpoint, codeVerifier, state };
}

function canonicalHttpsURL(raw: string, label: string): URL {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`OAuth ${label} must be a canonical HTTPS URL`);
  }
  if (url.protocol !== "https:" || url.username !== "" || url.password !== "" || url.search !== "" || url.hash !== "") {
    throw new Error(`OAuth ${label} must be a canonical HTTPS URL without credentials, query, or fragment`);
  }
  return url;
}

function randomUrlToken(random: (size: number) => Buffer, label: string): string {
  const bytes = random(32);
  if (!Buffer.isBuffer(bytes) || bytes.length !== 32) throw new Error(`OAuth ${label} generator returned invalid bytes`);
  return bytes.toString("base64url");
}

function readString(value: Record<string, unknown>, key: string): string {
  const candidate = value[key];
  if (typeof candidate !== "string" || candidate.trim() === "") throw new Error(`OAuth authorization server metadata ${key} is required`);
  return candidate;
}

function required(value: string, label: string): string {
  const normalized = value.trim();
  if (normalized === "") throw new Error(`OAuth ${label} is required`);
  return normalized;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
