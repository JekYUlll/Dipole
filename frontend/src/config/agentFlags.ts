function enabled(value: string | undefined): boolean {
  return value === 'true'
}

export const agentFlags = {
  elicitation: enabled(import.meta.env.VITE_AGENT_ELICITATION_ENABLED),
  approval: enabled(import.meta.env.VITE_AGENT_APPROVAL_ENABLED),
  subscriptions: enabled(import.meta.env.VITE_AGENT_SUBSCRIPTIONS_ENABLED),
  definitions: enabled(import.meta.env.VITE_AGENT_DEFINITIONS_ENABLED),
  memories: enabled(import.meta.env.VITE_AGENT_MEMORIES_ENABLED),
  memoryCorrection: enabled(import.meta.env.VITE_AGENT_MEMORY_CORRECTION_ENABLED),
  taskCreate: enabled(import.meta.env.VITE_AGENT_TASK_CREATE_ENABLED),
  timeline: enabled(import.meta.env.VITE_AGENT_TIMELINE_ENABLED),
  artifacts: enabled(import.meta.env.VITE_AGENT_ARTIFACTS_ENABLED),
}

export const agentTaskCreatePageEnabled = agentFlags.taskCreate && agentFlags.timeline
export const agentTaskInboxEnabled = agentFlags.timeline

export function agentTaskRunTarget(): { name: string } | undefined {
  if (agentTaskInboxEnabled) return { name: 'agent-task-inbox' }
  if (agentTaskCreatePageEnabled) return { name: 'agent-task-create' }
  return undefined
}

export interface AgentSettingsLink {
  id: string
  label: string
  to: { name: string }
}

export function agentSettingsLinks(): AgentSettingsLink[] {
  const links: AgentSettingsLink[] = []
  if (agentFlags.definitions) links.push({ id: 'definitions', label: 'Agent 定义', to: { name: 'agent-definitions' } })
  if (agentFlags.subscriptions) links.push({ id: 'subscriptions', label: '事件订阅', to: { name: 'agent-subscriptions' } })
  if (agentFlags.memories) links.push({ id: 'memories', label: '长期记忆', to: { name: 'agent-memories' } })
  if (agentFlags.artifacts) links.push({ id: 'artifacts', label: '任务产物', to: { name: 'agent-artifact-inbox' } })
  if (agentTaskInboxEnabled) links.push({ id: 'inbox', label: '任务运行', to: { name: 'agent-task-inbox' } })
  if (agentTaskCreatePageEnabled) links.push({ id: 'create', label: '创建任务', to: { name: 'agent-task-create' } })
  return links
}

export function agentControlHome(): { name: string } | undefined {
  return agentSettingsLinks()[0]?.to
}
