import { z } from "zod";

const uniqueStrings = (values: string[]) => new Set(values).size === values.length;

export const resourceScopeSchema = z.object({
  resourceType: z.string().trim().min(1),
  resourceId: z.string().trim().min(1),
  actions: z.array(z.string().trim().min(1)).min(1).refine(uniqueStrings, "scope actions must be unique")
}).strict();

export const executionContextSchema = z.object({
  tenantId: z.string().trim().min(1),
  principalUuid: z.string().trim().min(1),
  agentUuid: z.string().trim().min(1),
  delegatedByUuid: z.string().trim().min(1).optional(),
  taskId: z.string().trim().min(1),
  runId: z.string().trim().min(1),
  mode: z.enum(["shadow", "active"]),
  permissions: z.array(z.string().trim().min(1)).min(1).refine(uniqueStrings, "permissions must be unique"),
  resourceScopes: z.array(resourceScopeSchema).min(1),
  approvedCapabilities: z.array(z.string().trim().min(1)).refine(uniqueStrings, "approvals must be unique"),
  requestId: z.string().trim().min(1).optional(),
  traceId: z.string().trim().min(1).optional(),
  eventId: z.string().trim().min(1).optional()
}).strict().superRefine((context, refinement) => {
  if (context.delegatedByUuid !== undefined && context.delegatedByUuid !== context.principalUuid) {
    refinement.addIssue({ code: "custom", message: "delegator must match principal", path: ["delegatedByUuid"] });
  }
});

export type ResourceScope = z.infer<typeof resourceScopeSchema>;
export type ExecutionContext = z.infer<typeof executionContextSchema>;
