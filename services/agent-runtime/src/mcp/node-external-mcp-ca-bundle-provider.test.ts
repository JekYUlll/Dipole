import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { chmod, mkdtemp, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { createFileExternalMcpCaBundleProvider } from "./node-external-mcp-ca-bundle-provider.js";

const execFileAsync = promisify(execFile);
const caRef = "CA-0123456789ABCDEF";
let directory = "";
let certificatePath = "";
let alternateCertificatePath = "";

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), "dipole-mcp-ca-"));
  certificatePath = join(directory, "ca.pem");
  alternateCertificatePath = join(directory, "alternate.pem");
  await generateCertificate(certificatePath, join(directory, "ca-key.pem"), "mcp.example.com");
  await generateCertificate(alternateCertificatePath, join(directory, "alternate-key.pem"), "other.example.com");
  await chmod(certificatePath, 0o644);
  await chmod(alternateCertificatePath, 0o644);
});

afterAll(async () => {
  await rm(directory, { recursive: true, force: true });
});

describe("file external MCP CA bundle Provider", () => {
  it("reloads a bounded certificate-only bundle on every read", async () => {
    const livePath = join(directory, "live.pem");
    await writeFile(livePath, await readFile(certificatePath), { mode: 0o644 });
    const provider = createFileExternalMcpCaBundleProvider({ [caRef]: livePath });
    const first = await provider.read(caRef, new AbortController().signal);

    await writeFile(livePath, await readFile(alternateCertificatePath), { mode: 0o644 });
    const second = await provider.read(caRef, new AbortController().signal);

    expect(digest(first)).not.toBe(digest(second));
    expect(Buffer.from(second).toString("utf8")).toContain("BEGIN CERTIFICATE");
  });

  it("rejects symlinks, writable files, malformed PEM and unknown refs", async () => {
    const symlinkPath = join(directory, "ca-link.pem");
    await symlink(certificatePath, symlinkPath);
    await expect(createFileExternalMcpCaBundleProvider({ [caRef]: symlinkPath }).read(
      caRef, new AbortController().signal
    )).rejects.toThrow();

    const writablePath = join(directory, "writable.pem");
    await writeFile(writablePath, await readFile(certificatePath), { mode: 0o666 });
    await chmod(writablePath, 0o666);
    await expect(createFileExternalMcpCaBundleProvider({ [caRef]: writablePath }).read(
      caRef, new AbortController().signal
    )).rejects.toThrow(/file evidence/i);

    const malformedPath = join(directory, "malformed.pem");
    await writeFile(malformedPath, `${"x".repeat(256)}\n-----BEGIN PRIVATE KEY-----\nforbidden\n-----END PRIVATE KEY-----\n`, { mode: 0o644 });
    await expect(createFileExternalMcpCaBundleProvider({ [caRef]: malformedPath }).read(
      caRef, new AbortController().signal
    )).rejects.toThrow(/certificates/i);

    const provider = createFileExternalMcpCaBundleProvider({ [caRef]: certificatePath });
    await expect(provider.read("CA-FFFFFFFFFFFFFFFF", new AbortController().signal)).rejects.toThrow(/not configured/i);
  });

  it("honors pre-cancellation before file access", async () => {
    const controller = new AbortController();
    controller.abort(new Error("cancelled before CA read"));
    const provider = createFileExternalMcpCaBundleProvider({ [caRef]: certificatePath });
    await expect(provider.read(caRef, controller.signal)).rejects.toThrow(/cancelled before CA read/i);
  });
});

async function generateCertificate(certPath: string, keyPath: string, hostname: string): Promise<void> {
  await execFileAsync("openssl", [
    "req", "-x509", "-newkey", "rsa:2048", "-sha256", "-nodes",
    "-keyout", keyPath, "-out", certPath, "-days", "1", "-subj", `/CN=${hostname}`,
    "-addext", `subjectAltName=DNS:${hostname}`,
    "-addext", "basicConstraints=critical,CA:TRUE"
  ]);
}

function digest(value: Uint8Array): string {
  return createHash("sha256").update(value).digest("hex");
}
