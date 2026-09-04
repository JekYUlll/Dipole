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

export type AgentDrawerTarget = { path: '/'; query: { agent: '1'; view: string; drawer?: string } }

export function agentTaskRunTarget(): AgentDrawerTarget | undefined {
  if (agentTaskInboxEnabled) return { path: '/', query: { agent: '1', view: 'tasks' } }
  if (agentTaskCreatePageEnabled) return { path: '/', query: { agent: '1', view: 'tasks', drawer: 'create' } }
  return undefined
}

export interface AgentSettingsLink {
  id: string
  label: string
  to: AgentDrawerTarget
}

export function agentSettingsLinks(): AgentSettingsLink[] {
  const links: AgentSettingsLink[] = []
  if (agentFlags.definitions) links.push({ id: 'definitions', label: 'Agent 定义', to: { path: '/', query: { agent: '1', view: 'definitions' } } })
  if (agentFlags.subscriptions) links.push({ id: 'subscriptions', label: '事件订阅', to: { path: '/', query: { agent: '1', view: 'subscriptions' } } })
  if (agentFlags.memories) links.push({ id: 'memories', label: '长期记忆', to: { path: '/', query: { agent: '1', view: 'memories' } } })
  if (agentFlags.artifacts) links.push({ id: 'artifacts', label: '任务产物', to: { path: '/', query: { agent: '1', view: 'artifacts' } } })
  if (agentTaskInboxEnabled) links.push({ id: 'inbox', label: '任务运行', to: { path: '/', query: { agent: '1', view: 'tasks' } } })
  if (agentTaskCreatePageEnabled) links.push({ id: 'create', label: '创建任务', to: { path: '/', query: { agent: '1', view: 'tasks', drawer: 'create' } } })
  return links
}

export function agentControlHome(): AgentDrawerTarget | undefined {
  return agentSettingsLinks()[0]?.to
}
