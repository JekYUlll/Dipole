<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { agentTaskClient, type AgentTaskClient } from '@/api/agentTasks'

type ViewState = 'idle' | 'validation_error' | 'submitting' | 'unavailable'

const props = withDefaults(defineProps<{
  client?: AgentTaskClient
  requestId?: () => string
}>(), {
  client: () => agentTaskClient,
  requestId: () => () => crypto.randomUUID(),
})

const router = useRouter()
const goal = ref('')
const request = ref('')
const state = ref<ViewState>('idle')
const error = ref('')
const characterCount = computed(() => Array.from(goal.value.trim()).length)

async function submit() {
  const normalizedGoal = goal.value.trim()
  if (normalizedGoal.length === 0 || Array.from(normalizedGoal).length > 4000) {
    state.value = 'validation_error'
    error.value = '请填写 1 到 4000 个字符的任务目标。'
    return
  }
  if (props.client.startTask === undefined) {
    state.value = 'unavailable'
    error.value = '任务创建暂不可用。'
    return
  }
  if (request.value === '') request.value = props.requestId()
  state.value = 'submitting'
  error.value = ''
  try {
    const result = await props.client.startTask({ clientRequestId: request.value, goal: normalizedGoal })
    await router.replace({ name: 'agent-task-timeline', params: { taskId: result.taskId } })
  } catch {
    state.value = 'unavailable'
    error.value = '任务创建暂不可用。'
  }
}
</script>

<template>
  <section class="task-create" :data-agent-task-create-state="state" aria-live="polite" :aria-busy="state === 'submitting'">
    <header class="task-create-header">
      <div>
        <p class="eyebrow">AGENT TASK / CREATE</p>
        <h1>创建 Agent 任务</h1>
        <p class="description">任务提交后进入只读时间线。</p>
      </div>
      <span class="request-badge">REQUEST / {{ request || 'LOCAL' }}</span>
    </header>

    <form class="task-create-form" data-agent-task-create-form @submit.prevent="submit">
      <label for="agent-task-goal">任务目标</label>
      <textarea id="agent-task-goal" v-model="goal" data-agent-task-goal :disabled="state === 'submitting'" maxlength="4000" rows="7" placeholder="描述希望助手完成的工作" aria-describedby="agent-task-goal-help agent-task-goal-count" />
      <div class="field-meta">
        <span id="agent-task-goal-help">只读取当前认证账号已授权的会话。</span>
        <span id="agent-task-goal-count">{{ characterCount }}/4000</span>
      </div>
      <p class="boundary">提交不会启用 Runtime、Tool 或外部服务。</p>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <div class="action-row">
        <span class="state-label">{{ state === 'submitting' ? 'SUBMITTING' : 'VALIDATE BEFORE SUBMIT' }}</span>
        <button type="submit" class="primary-button" :disabled="state === 'submitting'" data-agent-task-create-submit>{{ state === 'submitting' ? '正在提交' : '提交任务' }}</button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.task-create { box-sizing: border-box; width: min(100%, 46rem); margin: 0 auto; padding: var(--dp-space-lg); border: 1px solid var(--dp-line); border-radius: var(--dp-radius-md); background: var(--dp-surface); color: var(--dp-ink); font-family: var(--dp-font-body); }
.task-create-header { display: flex; justify-content: space-between; gap: var(--dp-space-md); margin-bottom: var(--dp-space-lg); }
.eyebrow, .state-label, .request-badge { color: var(--dp-accent-strong); font: 700 .68rem/1.2 var(--dp-font-data); letter-spacing: .12em; }
.eyebrow { margin: 0 0 .35rem; }
h1 { margin: 0; font: 700 1.5rem/1.2 var(--dp-font-display); }
.description, .boundary, .field-meta { color: var(--dp-ink-soft); font-size: .85rem; }
.description { margin: .4rem 0 0; }
.request-badge { align-self: start; padding: .45rem .55rem; border: 1px solid var(--dp-line); border-radius: var(--dp-radius-sm); white-space: nowrap; }
.task-create-form { display: grid; gap: .65rem; }
label { font-weight: 700; }
textarea { box-sizing: border-box; width: 100%; resize: vertical; border: 1px solid var(--dp-line); border-radius: var(--dp-radius-sm); padding: .8rem; background: var(--dp-canvas); color: var(--dp-ink); font: inherit; }
textarea:focus { outline: 2px solid var(--dp-accent); outline-offset: 2px; }
.field-meta, .action-row { display: flex; justify-content: space-between; gap: .75rem; }
.boundary { margin: .25rem 0; padding: .65rem; border-left: 3px solid var(--dp-accent); background: var(--dp-canvas); }
.form-error { margin: 0; color: var(--dp-danger); font-size: .85rem; }
.action-row { align-items: center; margin-top: .4rem; }
.primary-button { border: 0; border-radius: var(--dp-radius-sm); padding: .7rem 1rem; background: var(--dp-accent); color: var(--dp-canvas); cursor: pointer; font: 700 .9rem/1 var(--dp-font-body); }
.primary-button:disabled { cursor: wait; opacity: .65; }
@media (max-width: 560px) { .task-create { min-height: 100vh; border: 0; border-radius: 0; padding: var(--dp-space-md); } .task-create-header { display: grid; } .request-badge { justify-self: start; } .field-meta, .action-row { align-items: start; flex-direction: column; } .primary-button { width: 100%; } }
</style>
