interface RuntimeFlags {
  elicitation?: boolean
  approval?: boolean
  subscriptions?: boolean
  definitions?: boolean
  memories?: boolean
  memoryCorrection?: boolean
  taskCreate?: boolean
  timeline?: boolean
  artifacts?: boolean
}

declare global {
  interface Window { __DIPOLE_FLAGS__?: RuntimeFlags }
}

function flag(runtimeKey: keyof RuntimeFlags, viteEnv: string | undefined): boolean {
  const runtime = window.__DIPOLE_FLAGS__
  if (runtime && runtimeKey in runtime) return runtime[runtimeKey] === true
  return viteEnv === 'true'
}

export const agentFlags = {
  elicitation: flag('elicitation', import.meta.env.VITE_AGENT_ELICITATION_ENABLED),
  approval: flag('approval', import.meta.env.VITE_AGENT_APPROVAL_ENABLED),
  subscriptions: flag('subscriptions', import.meta.env.VITE_AGENT_SUBSCRIPTIONS_ENABLED),
  definitions: flag('definitions', import.meta.env.VITE_AGENT_DEFINITIONS_ENABLED),
  memories: flag('memories', import.meta.env.VITE_AGENT_MEMORIES_ENABLED),
  memoryCorrection: flag('memoryCorrection', import.meta.env.VITE_AGENT_MEMORY_CORRECTION_ENABLED),
  taskCreate: flag('taskCreate', import.meta.env.VITE_AGENT_TASK_CREATE_ENABLED),
  timeline: flag('timeline', import.meta.env.VITE_AGENT_TIMELINE_ENABLED),
  artifacts: flag('artifacts', import.meta.env.VITE_AGENT_ARTIFACTS_ENABLED),
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
