import { describe, expect, it } from "vitest";

import {
  authorizationServerMetadataURL,
  createPkceAuthorizationRequest,
  parseAuthorizationServerMetadata
} from "./oauth-discovery-pkce.js";

const issuer = "https://auth.example.com/tenant-a";

describe("OAuth discovery and PKCE foundation", () => {
  it("derives RFC 8414 metadata before an issuer path and accepts exact S256 metadata", () => {
    expect(authorizationServerMetadataURL(issuer)).toBe(
      "https://auth.example.com/.well-known/oauth-authorization-server/tenant-a"
    );

    expect(parseAuthorizationServerMetadata(issuer, {
      issuer,
      authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["plain", "S256"]
    })).toEqual({
      issuer,
      authorizationEndpoint: "https://auth.example.com/tenant-a/authorize",
      tokenEndpoint: "https://auth.example.com/tenant-a/token"
    });
  });

  it("fails closed for identifier drift, insecure endpoints, or missing S256", () => {
    expect(() => parseAuthorizationServerMetadata(issuer, {
      issuer: "https://auth.example.com/other",
      authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["S256"]
    })).toThrow(/issuer/i);
    expect(() => parseAuthorizationServerMetadata(issuer, {
      issuer,
      authorization_endpoint: "http://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["S256"]
    })).toThrow(/HTTPS/i);
    expect(() => parseAuthorizationServerMetadata(issuer, {
      issuer,
      authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["plain"]
    })).toThrow(/S256/i);
  });

  it("creates a URL-safe S256 PKCE authorization request without persisting token material", () => {
    const request = createPkceAuthorizationRequest({
      metadata: parseAuthorizationServerMetadata(issuer, {
        issuer,
        authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
        token_endpoint: "https://auth.example.com/tenant-a/token",
        code_challenge_methods_supported: ["S256"]
      }),
      clientId: "dipole-agent",
      redirectUri: "https://agent.example.com/oauth/callback",
      scope: "mcp.read",
      randomBytes: size => Buffer.alloc(size, 7)
    });

    expect(request.codeVerifier).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(request.state).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(request.url.toString()).toContain("code_challenge_method=S256");
    expect(request.url.searchParams.get("code_challenge")).toHaveLength(43);
    expect(request.url.searchParams.get("state")).toBe(request.state);
    expect(request.url.searchParams.has("code_verifier")).toBe(false);
  });

  it("rejects unsafe issuer and callback URLs before constructing a request", () => {
    expect(() => authorizationServerMetadataURL("https://auth.example.com/tenant?test=1")).toThrow(/canonical/i);
    expect(() => createPkceAuthorizationRequest({
      metadata: parseAuthorizationServerMetadata(issuer, {
        issuer,
        authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
        token_endpoint: "https://auth.example.com/tenant-a/token",
        code_challenge_methods_supported: ["S256"]
      }),
      clientId: "dipole-agent",
      redirectUri: "http://agent.example.com/oauth/callback",
      scope: "mcp.read"
    })).toThrow(/redirect/i);
  });
});
