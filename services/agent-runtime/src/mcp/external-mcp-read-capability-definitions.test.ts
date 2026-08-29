import { describe, expect, it } from "vitest";
import { z } from "zod";

import {
  ExternalMcpCapabilityDefinitionRegistry
} from "./external-mcp-deployment-route-manifest.js";
import {
  createExternalMcpReadCapabilityDefinitions,
  repositoryIssueReadCapabilityId
} from "./external-mcp-read-capability-definitions.js";

describe("external MCP code-owned read Capability definitions", () => {
  it("publishes one sealed read-only repository Issue authority", () => {
    const definitions = createExternalMcpReadCapabilityDefinitions();
    const definition = definitions.resolve(repositoryIssueReadCapabilityId);

    expect(definition?.descriptor).toEqual({
      id: repositoryIssueReadCapabilityId,
      risk: "read",
      requiredPermission: repositoryIssueReadCapabilityId
    });
    expect(definition?.egressCeiling).toEqual({
      allowedArgumentNames: ["owner", "repo", "issue_number"],
      maximumBytes: 1024
    });
    expect(Object.isFrozen(definition)).toBe(true);
    expect(Object.isFrozen(definition?.descriptor)).toBe(true);
    expect(Object.isFrozen(definition?.egressCeiling.allowedArgumentNames)).toBe(true);
    expect(definitions.resolve("repository.issue.write")).toBeUndefined();
  });

  it("normalizes route arguments into one canonical resource scope", () => {
    const definition = createExternalMcpReadCapabilityDefinitions().resolve(repositoryIssueReadCapabilityId)!;
    const input = definition.inputSchema.parse({ owner: " OpenAI ", repo: " CODEX.CLI ", issue_number: 42 });

    expect(input).toEqual({ owner: "openai", repo: "codex.cli", issue_number: 42 });
    expect(definition.resolveResource(input, {} as never)).toEqual({
      resourceType: "repository_issue",
      resourceId: "openai/codex.cli#42",
      action: "read"
    });
  });

  it("rejects ambiguous identifiers, unknown arguments and invalid issue numbers", () => {
    const schema = createExternalMcpReadCapabilityDefinitions()
      .resolve(repositoryIssueReadCapabilityId)!.inputSchema;
    const cases = [
      { owner: "-openai", repo: "codex", issue_number: 1 },
      { owner: "openai", repo: "../codex", issue_number: 1 },
      { owner: "openai", repo: "codex", issue_number: 0 },
      { owner: "openai", repo: "codex", issue_number: 1, admin: true }
    ];

    for (const input of cases) expect(() => schema.parse(input)).toThrow();
  });

  it("prevents startup composition from appending deployment-selected definitions", () => {
    const definitions = createExternalMcpReadCapabilityDefinitions();

    expect(() => definitions.register({
      descriptor: { id: "repository.issue.write", risk: "write", requiredPermission: "repository.issue.write" },
      inputSchema: z.object({}).strict(),
      egressCeiling: { allowedArgumentNames: [], maximumBytes: 128 },
      resolveResource: () => ({ resourceType: "repository_issue", resourceId: "*", action: "write" })
    })).toThrow(/^External MCP Capability definitions are sealed$/);
  });

  it("returns an isolated sealed Registry for each startup attempt", () => {
    const first = createExternalMcpReadCapabilityDefinitions();
    const second = createExternalMcpReadCapabilityDefinitions();

    expect(first).not.toBe(second);
    expect(first.resolve(repositoryIssueReadCapabilityId)).not.toBe(second.resolve(repositoryIssueReadCapabilityId));
  });

  it("captures definition authority before a caller can mutate its source object", () => {
    const source = {
      descriptor: { id: "repository.issue.test", risk: "read" as const, requiredPermission: "repository.issue.test" },
      inputSchema: z.object({ issue_number: z.number().int() }).strict(),
      egressCeiling: { allowedArgumentNames: ["issue_number"], maximumBytes: 128 },
      resolveResource: (input: { issue_number: number }) => ({
        resourceType: "repository_issue", resourceId: `original#${input.issue_number}`, action: "read"
      })
    };
    const definitions = new ExternalMcpCapabilityDefinitionRegistry();
    definitions.register(source);
    source.resolveResource = input => ({
      resourceType: "repository_issue", resourceId: `mutated#${input.issue_number}`, action: "write"
    });

    const definition = definitions.resolve("repository.issue.test")!;
    const input = definition.inputSchema.parse({ issue_number: 7 });
    expect(definition.resolveResource(input, {} as never)).toEqual({
      resourceType: "repository_issue", resourceId: "original#7", action: "read"
    });
  });
});
