import { execFile } from "node:child_process";
import { readFile, mkdtemp, rm } from "node:fs/promises";
import { createServer as createHttpsServer, type Server as HttpsServer } from "node:https";
import { createServer as createTcpServer, type Server as TcpServer, type Socket } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import type { ExternalMcpCaBundleProvider } from "./node-external-mcp-ca-bundle-provider.js";
import {
  NodeExternalMcpPinnedTlsDispatcher,
  type NodeExternalMcpHttpsClient
} from "./node-external-mcp-pinned-tls-dispatcher.js";

const execFileAsync = promisify(execFile);
const caRef = "CA-0123456789ABCDEF";
let directory = "";
let certificate: Buffer;
let privateKey: Buffer;
let alternateCertificate: Buffer;
let httpsServer: HttpsServer;
let httpsPort = 0;

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), "dipole-mcp-tls-"));
  const certPath = join(directory, "server.pem");
  const keyPath = join(directory, "server-key.pem");
  const alternatePath = join(directory, "alternate.pem");
  await generateCertificate(certPath, keyPath, "mcp.example.com");
  await generateCertificate(alternatePath, join(directory, "alternate-key.pem"), "other.example.com");
  certificate = await readFile(certPath);
  privateKey = await readFile(keyPath);
  alternateCertificate = await readFile(alternatePath);
  httpsServer = createHttpsServer({ cert: certificate, key: privateKey }, (request, response) => {
    if (request.url === "/redirect") {
      response.writeHead(302, { location: "https://mcp.example.com/other" });
      response.end();
      return;
    }
    const chunks: Buffer[] = [];
    request.on("data", chunk => chunks.push(Buffer.from(chunk)));
    request.on("end", () => {
      response.writeHead(200, { "content-type": "text/plain", "x-dipole-test": "tls" });
      response.end(Buffer.concat(chunks));
    });
  });
  await listen(httpsServer);
  httpsPort = (httpsServer.address() as { port: number }).port;
});

afterAll(async () => {
  await close(httpsServer);
  await rm(directory, { recursive: true, force: true });
});

describe("Node external MCP pinned TLS Dispatcher", () => {
  it("streams a request through approved-address lookup and returns verified peer evidence", async () => {
    const dispatcher = new NodeExternalMcpPinnedTlsDispatcher(caProvider(certificate));
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(Buffer.from("hello "));
        controller.enqueue(Buffer.from("mcp"));
        controller.close();
      }
    });
    const request = new Request(`https://mcp.example.com:${httpsPort}/echo`, {
      method: "POST", body, duplex: "half"
    } as RequestInit & { duplex: "half" });

    const result = await dispatcher.dispatch({
      request, addresses: [{ address: "127.0.0.1", family: 4 }],
      tlsServerName: "mcp.example.com", caBundleRef: caRef
    }, request.signal);

    expect(result.connectedAddress).toBe("127.0.0.1");
    expect(result.response.status).toBe(200);
    expect(result.response.headers.get("x-dipole-test")).toBe("tls");
    await expect(result.response.text()).resolves.toBe("hello mcp");
  });

  it("enforces TLS ServerName and CA chain", async () => {
    const trusted = new NodeExternalMcpPinnedTlsDispatcher(caProvider(certificate));
    const wrongName = new Request(`https://other.example.com:${httpsPort}/`);
    await expect(trusted.dispatch({
      request: wrongName, addresses: [{ address: "127.0.0.1", family: 4 }],
      tlsServerName: "other.example.com", caBundleRef: caRef
    }, wrongName.signal)).rejects.toThrow(/TLS request failed/i);

    const wrongCa = new NodeExternalMcpPinnedTlsDispatcher(caProvider(alternateCertificate));
    const request = new Request(`https://mcp.example.com:${httpsPort}/`);
    await expect(wrongCa.dispatch({
      request, addresses: [{ address: "127.0.0.1", family: 4 }],
      tlsServerName: "mcp.example.com", caBundleRef: caRef
    }, request.signal)).rejects.toThrow(/TLS request failed/i);
  });

  it("returns redirects without following them", async () => {
    const dispatcher = new NodeExternalMcpPinnedTlsDispatcher(caProvider(certificate));
    const request = new Request(`https://mcp.example.com:${httpsPort}/redirect`);
    const result = await dispatcher.dispatch({
      request, addresses: [{ address: "127.0.0.1", family: 4 }],
      tlsServerName: "mcp.example.com", caBundleRef: caRef
    }, request.signal);
    expect(result.response.status).toBe(302);
    expect(result.response.headers.get("location")).toBe("https://mcp.example.com/other");
  });

  it("rejects a client-reported peer outside the approved set and cancels its body", async () => {
    const cancel = vi.fn();
    const client: NodeExternalMcpHttpsClient = {
      request: vi.fn(async () => ({
        response: new Response(new ReadableStream({ cancel })), connectedAddress: "1.1.1.1"
      }))
    };
    const dispatcher = new NodeExternalMcpPinnedTlsDispatcher(caProvider(certificate), { httpsClient: client });
    const request = new Request("https://mcp.example.com/");
    await expect(dispatcher.dispatch({
      request, addresses: [{ address: "8.8.8.8", family: 4 }],
      tlsServerName: "mcp.example.com", caBundleRef: caRef
    }, request.signal)).rejects.toThrow(/outside the approved/i);
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("reloads CA material for every dispatch", async () => {
    const read = vi.fn(async () => Uint8Array.from(certificate));
    const client: NodeExternalMcpHttpsClient = {
      request: vi.fn(async input => ({
        response: new Response(null, { status: 204 }),
        connectedAddress: input.addresses[0]!.address
      }))
    };
    const dispatcher = new NodeExternalMcpPinnedTlsDispatcher({ read }, { httpsClient: client });
    for (let index = 0; index < 2; index += 1) {
      const request = new Request("https://mcp.example.com/");
      await dispatcher.dispatch({
        request, addresses: [{ address: "8.8.8.8", family: 4 }],
        tlsServerName: "mcp.example.com", caBundleRef: caRef
      }, request.signal);
    }
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("propagates cancellation and enforces TLS connect timeout", async () => {
    const tcpServer = createTcpServer(() => undefined);
    const sockets = new Set<Socket>();
    tcpServer.on("connection", socket => {
      sockets.add(socket);
      socket.once("close", () => sockets.delete(socket));
    });
    await listen(tcpServer);
    const port = (tcpServer.address() as { port: number }).port;
    try {
      const dispatcher = new NodeExternalMcpPinnedTlsDispatcher(caProvider(certificate), { connectTimeoutMs: 100 });
      const controller = new AbortController();
      const cancelledRequest = new Request(`https://mcp.example.com:${port}/`, { signal: controller.signal });
      const pending = dispatcher.dispatch({
        request: cancelledRequest, addresses: [{ address: "127.0.0.1", family: 4 }],
        tlsServerName: "mcp.example.com", caBundleRef: caRef
      }, controller.signal);
      controller.abort(new Error("cancelled TLS request"));
      await expect(pending).rejects.toThrow(/cancelled TLS request/i);

      const timeoutRequest = new Request(`https://mcp.example.com:${port}/`);
      await expect(dispatcher.dispatch({
        request: timeoutRequest, addresses: [{ address: "127.0.0.1", family: 4 }],
        tlsServerName: "mcp.example.com", caBundleRef: caRef
      }, timeoutRequest.signal)).rejects.toThrow(/TLS request failed/i);
    } finally {
      for (const socket of sockets) socket.destroy();
      await close(tcpServer);
    }
  });
});

function caProvider(value: Uint8Array): ExternalMcpCaBundleProvider {
  return { read: vi.fn(async () => Uint8Array.from(value)) };
}

async function generateCertificate(certPath: string, keyPath: string, hostname: string): Promise<void> {
  await execFileAsync("openssl", [
    "req", "-x509", "-newkey", "rsa:2048", "-sha256", "-nodes",
    "-keyout", keyPath, "-out", certPath, "-days", "1", "-subj", `/CN=${hostname}`,
    "-addext", `subjectAltName=DNS:${hostname}`,
    "-addext", "basicConstraints=critical,CA:TRUE"
  ]);
}

function listen(server: HttpsServer | TcpServer): Promise<void> {
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      server.removeListener("error", reject);
      resolve();
    });
  });
}

function close(server: HttpsServer | TcpServer): Promise<void> {
  return new Promise((resolve, reject) => server.close(error => error === undefined ? resolve() : reject(error)));
}
