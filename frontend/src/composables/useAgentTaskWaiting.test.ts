import { flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { useAgentTaskWaiting, waitingNoticeHeadline } from './useAgentTaskWaiting'
import type { AgentOwnedTask, AgentTaskInboxPage } from '@/api/agentTasks'

const waitingApproval: AgentOwnedTask = {
  taskId: 'TASK-1', status: 'waiting_approval', revision: 2, pendingKind: 'approval',
  goal: 'Summarize unread work', updatedAtUnixMs: 1_700_000_000_000,
}

function page(tasks: AgentOwnedTask[], nextCursor = ''): AgentTaskInboxPage {
  return { tasks, nextCursor }
}

describe('useAgentTaskWaiting', () => {
  it('summarizes locator counts without rendering task bodies', () => {
    expect(waitingNoticeHeadline([])).toBe('')
    expect(waitingNoticeHeadline([{ taskId: 'TASK-1', pendingKind: 'approval', revision: 2 }])).toBe('有任务等待审批')
    expect(waitingNoticeHeadline([{ taskId: 'TASK-2', pendingKind: 'input', revision: 1 }])).toBe('有任务等待补充信息')
    expect(waitingNoticeHeadline([
      { taskId: 'TASK-1', pendingKind: 'approval', revision: 2 },
      { taskId: 'TASK-2', pendingKind: 'input', revision: 1 },
    ])).toBe('2 个任务等待处理')
  })

  it('accepts a newer revision, ignores duplicates, and keeps invalid locators off the banner', () => {
    const waiting = useAgentTaskWaiting({ enabled: true })
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'approval', revision: 2 })).toBe(true)
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'approval', revision: 2 })).toBe(false)
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'input', revision: 1 })).toBe(false)
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'approval', revision: 3, summary: 'secret' })).toBe(false)
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'input', revision: 4 })).toBe(true)
    expect(waiting.notices.value).toEqual([{ taskId: 'TASK-1', pendingKind: 'input', revision: 4 }])
    expect(waiting.headline.value).toBe('有任务等待补充信息')
  })

  it('stays inert when the inbox surface is closed', () => {
    const list = vi.fn().mockResolvedValue(page([waitingApproval]))
    const waiting = useAgentTaskWaiting({ enabled: false, list })
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'approval', revision: 2 })).toBe(false)
    return expect(waiting.refreshFromInbox('replace')).resolves.toBeUndefined().then(() => {
      expect(list).not.toHaveBeenCalled()
      expect(waiting.notices.value).toEqual([])
    })
  })

  it('merges inbox waiting rows onto locators and uses the list as reconnect authority', async () => {
    const list = vi.fn()
      .mockResolvedValueOnce(page([waitingApproval]))
      .mockResolvedValueOnce(page([]))
    const waiting = useAgentTaskWaiting({ enabled: true, list })
    expect(waiting.acceptLocator({ task_uuid: 'TASK-2', pending_kind: 'input', revision: 5 })).toBe(true)
    await flushPromises()
    expect(waiting.notices.value).toEqual([
      { taskId: 'TASK-2', pendingKind: 'input', revision: 5 },
      { taskId: 'TASK-1', pendingKind: 'approval', revision: 2 },
    ])
    await waiting.refreshFromInbox('replace')
    expect(waiting.notices.value).toEqual([])
    expect(waiting.acceptLocator({ task_uuid: 'TASK-2', pending_kind: 'input', revision: 5 })).toBe(false)
  })

  it('keeps locator notices when the inbox list fails', async () => {
    const list = vi.fn().mockRejectedValue(new Error('unavailable'))
    const waiting = useAgentTaskWaiting({ enabled: true, list })
    expect(waiting.acceptLocator({ task_uuid: 'TASK-1', pending_kind: 'approval', revision: 2 })).toBe(true)
    await waiting.refreshFromInbox('replace')
    expect(waiting.notices.value).toEqual([{ taskId: 'TASK-1', pendingKind: 'approval', revision: 2 }])
  })
})
