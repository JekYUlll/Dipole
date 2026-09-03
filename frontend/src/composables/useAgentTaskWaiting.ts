import { computed, ref } from 'vue'
import {
  parseAgentTaskWaitingEvent,
  type AgentOwnedTask,
  type AgentTaskInboxPage,
  type AgentTaskWaitingLocator,
} from '@/api/agentTasks'

export interface AgentTaskWaitingOptions {
  enabled: boolean
  list?: () => Promise<AgentTaskInboxPage>
}

export function waitingNoticeHeadline(notices: readonly AgentTaskWaitingLocator[]): string {
  if (notices.length === 0) return ''
  if (notices.length === 1) {
    return notices[0].pendingKind === 'approval' ? '有任务等待审批' : '有任务等待补充信息'
  }
  return `${notices.length} 个任务等待处理`
}

export function useAgentTaskWaiting(options: AgentTaskWaitingOptions) {
  const notices = ref<AgentTaskWaitingLocator[]>([])
  const seen = new Map<string, number>()
  const headline = computed(() => waitingNoticeHeadline(notices.value))

  function acceptLocator(raw: unknown): boolean {
    if (!options.enabled) return false
    let locator: AgentTaskWaitingLocator
    try {
      locator = parseAgentTaskWaitingEvent(raw)
    } catch {
      return false
    }
    const previous = seen.get(locator.taskId)
    if (previous !== undefined && previous >= locator.revision) return false
    seen.set(locator.taskId, locator.revision)
    upsert(locator)
    void refreshFromInbox('merge')
    return true
  }

  async function refreshFromInbox(mode: 'merge' | 'replace' = 'replace'): Promise<void> {
    if (!options.enabled || options.list === undefined) return
    try {
      const waiting = waitingFromInbox(await options.list())
      if (mode === 'replace') {
        for (const locator of waiting) {
          const previous = seen.get(locator.taskId)
          if (previous === undefined || previous < locator.revision) seen.set(locator.taskId, locator.revision)
        }
        notices.value = waiting
        return
      }
      for (const locator of waiting) {
        const previous = seen.get(locator.taskId)
        if (previous !== undefined && previous >= locator.revision) continue
        seen.set(locator.taskId, locator.revision)
        upsert(locator)
      }
    } catch {
      // Locator notices stay until the owner inbox list succeeds.
    }
  }

  function upsert(locator: AgentTaskWaitingLocator) {
    const index = notices.value.findIndex(item => item.taskId === locator.taskId)
    if (index >= 0) {
      notices.value = notices.value.map((item, itemIndex) => itemIndex === index ? locator : item)
      return
    }
    notices.value = [...notices.value, locator]
  }

  return { notices, headline, acceptLocator, refreshFromInbox }
}

function waitingFromInbox(page: AgentTaskInboxPage): AgentTaskWaitingLocator[] {
  return page.tasks.flatMap(task => {
    const locator = locatorFromOwnedTask(task)
    return locator === undefined ? [] : [locator]
  })
}

function locatorFromOwnedTask(task: AgentOwnedTask): AgentTaskWaitingLocator | undefined {
  if (task.pendingKind === undefined || task.revision < 1) return undefined
  return { taskId: task.taskId, pendingKind: task.pendingKind, revision: task.revision }
}
