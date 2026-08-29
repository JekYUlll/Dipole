import { z } from "zod";

import { ExternalMcpCapabilityDefinitionRegistry } from "./external-mcp-deployment-route-manifest.js";

export const repositoryIssueReadCapabilityId = "repository.issue.read" as const;

const repositoryOwnerSchema = z.string()
  .trim()
  .min(1)
  .max(39)
  .regex(/^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$/)
  .transform(value => value.toLowerCase());
const repositoryNameSchema = z.string()
  .trim()
  .min(1)
  .max(100)
  .regex(/^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$/)
  .refine(value => value !== "." && value !== "..")
  .transform(value => value.toLowerCase());
const repositoryIssueReadInputSchema = z.object({
  owner: repositoryOwnerSchema,
  repo: repositoryNameSchema,
  issue_number: z.number().int().min(1).max(2_147_483_647)
}).strict();

export function createExternalMcpReadCapabilityDefinitions(): ExternalMcpCapabilityDefinitionRegistry {
  const definitions = new ExternalMcpCapabilityDefinitionRegistry();
  definitions.register({
    descriptor: {
      id: repositoryIssueReadCapabilityId,
      risk: "read",
      requiredPermission: repositoryIssueReadCapabilityId
    },
    inputSchema: repositoryIssueReadInputSchema,
    egressCeiling: {
      allowedArgumentNames: ["owner", "repo", "issue_number"],
      maximumBytes: 1024
    },
    resolveResource: input => ({
      resourceType: "repository_issue",
      resourceId: `${input.owner}/${input.repo}#${input.issue_number}`,
      action: "read"
    })
  });
  return definitions.seal();
}
