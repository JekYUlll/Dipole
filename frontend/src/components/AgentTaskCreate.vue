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
  requestId: () => crypto.randomUUID(),
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
        <p class="description">为当前账号创建一个可恢复的协作任务，并以只读时间线持续呈现进展。</p>
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
      <p class="boundary"><strong>只读范围</strong>：提交只使用当前认证账号已授权的会话。涉及写入的操作将保持关闭，并在未来通过“需要确认后执行”呈现。</p>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <div class="action-row">
        <span class="state-label">{{ state === 'submitting' ? 'SUBMITTING' : 'VALIDATE BEFORE SUBMIT' }}</span>
        <button type="submit" class="primary-button" :disabled="state === 'submitting'" data-agent-task-create-submit>{{ state === 'submitting' ? '正在提交' : '提交任务' }}</button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.task-create { box-sizing: border-box; width: 100%; margin: 0 auto; padding: clamp(22px, 4vw, 42px); border: 1px solid rgba(9, 37, 69, .12); border-radius: 2px; background: var(--dp-surface); box-shadow: var(--dp-v3-shadow); color: var(--dp-ink); font-family: var(--dp-font-body); }
.task-create-header { display: flex; justify-content: space-between; gap: var(--dp-space-md); margin-bottom: 2rem; padding-bottom: 1.2rem; border-bottom: 1px solid var(--dp-line); }
.eyebrow, .state-label, .request-badge { color: var(--dp-v3-gold); font: 800 .68rem/1.2 var(--dp-font-data); letter-spacing: .12em; }
.eyebrow { margin: 0 0 .35rem; }
h1 { margin: 0; color: var(--dp-v3-navy); font: 800 clamp(1.5rem, 3vw, 2rem)/1.1 var(--dp-font-display); letter-spacing: -.045em; }
.description, .boundary, .field-meta { color: var(--dp-ink-soft); font-size: .85rem; }
.description { max-width: 32rem; margin: .6rem 0 0; line-height: 1.65; }
.request-badge { align-self: start; padding: .48rem .55rem; border: 1px solid var(--dp-line); color: var(--dp-v3-muted); white-space: nowrap; }
.task-create-form { display: grid; gap: .72rem; }
label { color: var(--dp-v3-navy); font-weight: 800; }
textarea { box-sizing: border-box; width: 100%; min-height: 11rem; resize: vertical; border: 1px solid var(--dp-line); border-radius: 0; padding: 1rem; background: #fff; color: var(--dp-ink); font: inherit; line-height: 1.65; }
textarea::placeholder { color: var(--dp-ink-faint); }
textarea:focus { border-color: var(--dp-v3-red); outline: 3px solid rgba(242, 38, 42, .12); outline-offset: 0; }
.field-meta, .action-row { display: flex; justify-content: space-between; gap: .75rem; }
.boundary { margin: .65rem 0 .25rem; padding: .85rem 1rem; border-left: 3px solid var(--dp-v3-gold); background: var(--dp-v3-gold-soft); line-height: 1.65; }
.boundary strong { color: var(--dp-v3-navy); }
.form-error { margin: 0; color: var(--dp-danger); font-size: .85rem; }
.action-row { align-items: center; margin-top: .85rem; padding-top: 1rem; border-top: 1px solid var(--dp-line); }
.primary-button { border: 0; border-radius: 0; padding: .85rem 1.25rem; background: var(--dp-v3-red); color: #fff; cursor: pointer; font: 800 .9rem/1 var(--dp-font-body); box-shadow: 4px 4px 0 var(--dp-v3-navy); transition: transform .16s ease, box-shadow .16s ease; }
.primary-button:hover:not(:disabled) { box-shadow: 2px 2px 0 var(--dp-v3-navy); transform: translate(2px, 2px); }
.primary-button:disabled { cursor: wait; opacity: .65; }
@media (max-width: 560px) { .task-create { min-height: 0; border-radius: 0; padding: 22px; } .task-create-header { display: grid; } .request-badge { justify-self: start; } .field-meta, .action-row { align-items: start; flex-direction: column; } .primary-button { width: 100%; } }
</style>
