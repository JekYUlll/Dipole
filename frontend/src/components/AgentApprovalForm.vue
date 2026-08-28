<template>
  <section class="approval-shell" :data-agent-approval-state="viewState" :aria-busy="busy" aria-live="polite">
    <header class="approval-header">
      <div>
        <p class="eyebrow">DIPOLE AGENT / APPROVAL REQUEST</p>
        <h1>确认 Agent 操作</h1>
      </div>
      <span class="task-badge">{{ shortTaskId }}</span>
    </header>

    <div v-if="viewState === 'loading'" class="state-card" role="status">正在确认当前审批请求</div>
    <div v-else-if="viewState === 'unavailable'" class="state-card state-card-danger" role="alert">
      <p class="state-title">无法确认当前审批请求</p>
      <p>为避免批准失效任务，审批操作已暂时隐藏。</p>
      <button type="button" class="secondary-button" data-agent-retry @click="loadTask">重新确认</button>
    </div>
    <div v-else-if="pending" class="approval-grid">
      <aside class="source-rail">
        <p class="rail-label">TASK CONTROL</p>
        <dl>
          <dt>Task</dt><dd>{{ task?.taskId }}</dd>
          <dt>Revision</dt><dd>{{ task?.revision }}</dd>
          <dt>Request</dt><dd>{{ pending.requestId }}</dd>
        </dl>
        <div class="deadline"><span>有效期</span><strong>{{ deadlineLabel }}</strong></div>
      </aside>
      <div class="approval-panel">
        <p class="rail-label">AGENT WANTS TO ACT</p>
        <h2>{{ pending.summary }}</h2>
        <p class="warning">批准后任务将继续执行。请确认操作对象和影响范围，拒绝会终止当前审批请求。</p>
        <div class="action-row">
          <button type="button" class="secondary-button" :disabled="busy || expired" data-agent-deny @click="resolve('denied')">拒绝并终止</button>
          <button type="button" class="primary-button" :disabled="busy || expired" data-agent-approve @click="resolve('approved')">
            {{ viewState === 'submitting' ? '正在提交' : expired ? '请求已过期' : '批准并继续' }}
          </button>
        </div>
      </div>
    </div>
    <div v-else class="state-card state-card-complete">
      <p class="state-title">{{ terminalTitle }}</p>
      <p>{{ terminalDescription }}</p>
      <a href="/app/" class="secondary-button link-button">返回消息</a>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { agentTaskClient, type AgentApprovalPending, type AgentTaskClient, type AgentTaskState } from '@/api/agentTasks'

type ViewState = 'loading' | 'waiting_approval' | 'submitting' | 'running' | 'cancelled' | 'expired' | 'completed' | 'failed' | 'unavailable'

const props = withDefaults(defineProps<{ taskId: string; client?: AgentTaskClient; now?: () => number }>(), {
  client: () => agentTaskClient,
  now: () => Date.now(),
})
const task = ref<AgentTaskState>()
const pending = ref<AgentApprovalPending>()
const viewState = ref<ViewState>('loading')
const clock = ref(props.now())
let clockTimer: ReturnType<typeof setInterval> | undefined

const shortTaskId = computed(() => props.taskId.length > 18 ? `${props.taskId.slice(0, 15)}...` : props.taskId)
const busy = computed(() => viewState.value === 'loading' || viewState.value === 'submitting')
const expired = computed(() => pending.value !== undefined && clock.value >= pending.value.expiresAtUnixMs)
const deadlineLabel = computed(() => {
  if (!pending.value) return '--'
  const remaining = Math.max(0, pending.value.expiresAtUnixMs - clock.value)
  if (remaining === 0) return '已过期'
  const minutes = Math.ceil(remaining / 60_000)
  return minutes < 60 ? `${minutes} 分钟内` : new Date(pending.value.expiresAtUnixMs).toLocaleString('zh-CN')
})
const terminalTitles: Partial<Record<ViewState, string>> = {
  running: '审批已接收，任务继续执行', cancelled: '审批已拒绝或任务已取消', expired: '审批请求已过期', completed: '任务已完成', failed: '任务执行失败',
}
const terminalTitle = computed(() => terminalTitles[viewState.value] ?? '当前无需审批')
const terminalDescription = computed(() => viewState.value === 'running' ? 'Agent 已恢复执行，可返回消息页面等待结果。' : '当前页面不会再接受这次审批决定。')

onMounted(() => {
  void loadTask()
  clockTimer = setInterval(() => {
    clock.value = props.now()
    if (expired.value && pending.value) void loadTask()
  }, 1_000)
})
onBeforeUnmount(() => { if (clockTimer) clearInterval(clockTimer) })

async function loadTask(): Promise<void> {
  viewState.value = 'loading'
  pending.value = undefined
  try {
    const current = await props.client.getTask(props.taskId)
    task.value = current
    applyTask(current)
  } catch {
    task.value = undefined
    viewState.value = 'unavailable'
  }
}

function applyTask(current: AgentTaskState): void {
  if (current.status === 'waiting_approval' && current.pending?.kind === 'approval') {
    if (props.now() >= current.pending.expiresAtUnixMs) { viewState.value = 'expired'; return }
    pending.value = current.pending
    viewState.value = 'waiting_approval'
    return
  }
  pending.value = undefined
  if (current.status === 'cancelled') viewState.value = current.cancellation?.reason === 'approval_expired' ? 'expired' : 'cancelled'
  else if (current.status === 'completed') viewState.value = 'completed'
  else if (current.status === 'failed') viewState.value = 'failed'
  else viewState.value = 'running'
}

async function resolve(decision: 'approved' | 'denied'): Promise<void> {
  const current = pending.value
  if (!current || expired.value || busy.value) return
  viewState.value = 'submitting'
  try {
    await props.client.resolveApproval(props.taskId, current.approvalId, decision)
    await loadTask()
  } catch {
    pending.value = undefined
    viewState.value = 'unavailable'
  }
}
</script>

<style scoped>
.approval-shell { --ink:#17221d; --muted:#68736d; --paper:#f5f0e6; --cream:#fffdf7; --line:#d9d1c2; --accent:#d4532f; --forest:#193d32; min-height:100vh; padding:clamp(24px,5vw,64px); color:var(--ink); background:radial-gradient(circle at 82% 4%,rgba(212,83,47,.14),transparent 27%),repeating-linear-gradient(90deg,transparent 0 79px,rgba(25,61,50,.035) 80px),var(--paper); font-family:Manrope,"Noto Sans SC",sans-serif }
.approval-header { max-width:920px; margin:0 auto 24px; display:flex; align-items:end; justify-content:space-between; gap:24px }.approval-header h1 { margin:4px 0 0; font-family:Georgia,"Noto Serif SC",serif; font-size:clamp(30px,5vw,52px); font-weight:500; letter-spacing:-.04em }.eyebrow,.rail-label { margin:0; color:var(--accent); font-size:11px; font-weight:800; letter-spacing:.16em }.task-badge { padding:8px 11px; border:1px solid var(--line); border-radius:999px; color:var(--muted); background:#ffffff73; font:700 11px ui-monospace,monospace }
.approval-grid { max-width:920px; margin:0 auto; display:grid; grid-template-columns:minmax(220px,290px) 1fr; overflow:hidden; border:1px solid #17221d29; border-radius:6px 24px 6px 6px; box-shadow:0 26px 70px #28302724 }.source-rail { padding:32px; color:#eef4ef; background:var(--forest) }.source-rail .rail-label { color:#91bba8 }.source-rail dl { margin:24px 0 0 }.source-rail dt { margin-top:18px; color:#91a99e; font-size:10px; text-transform:uppercase; letter-spacing:.12em }.source-rail dd { margin:4px 0 0; overflow-wrap:anywhere; font:600 12px ui-monospace,monospace }.deadline { margin-top:36px; padding-top:20px; border-top:1px solid #ffffff24; display:flex; justify-content:space-between; color:#9fb4aa; font-size:11px }.deadline strong { color:#fff }.approval-panel { padding:clamp(28px,5vw,56px); background:var(--cream) }.approval-panel h2 { margin:12px 0 18px; font-family:Georgia,"Noto Serif SC",serif; font-size:clamp(22px,3vw,34px); line-height:1.22; font-weight:500 }.warning { margin:0; padding:14px 16px; border-left:4px solid var(--accent); color:var(--muted); background:#d4532f0d; font-size:13px; line-height:1.65 }.action-row { display:flex; justify-content:flex-end; gap:10px; margin-top:38px; padding-top:24px; border-top:1px solid var(--line) }button,.link-button { min-height:42px; padding:0 17px; border-radius:3px; font:750 12px inherit; cursor:pointer }button:focus-visible,.link-button:focus-visible { outline:3px solid #d4532f59; outline-offset:3px }.primary-button { border:1px solid var(--accent); color:#fff; background:var(--accent) }.secondary-button { border:1px solid var(--line); color:var(--ink); background:transparent }button:disabled { cursor:not-allowed; opacity:.55 }.state-card { max-width:760px; margin:80px auto; padding:34px; border:1px solid var(--line); border-radius:4px 18px 4px 4px; background:var(--cream); box-shadow:0 20px 55px #2830271c }.state-card-danger { border-left:5px solid var(--accent) }.state-card-complete { border-left:5px solid #52856e }.state-title { margin:0 0 8px; font-family:Georgia,"Noto Serif SC",serif; font-size:22px }.state-card>p:not(.state-title) { color:var(--muted) }.state-card .secondary-button { margin-top:12px }.link-button { display:inline-flex; align-items:center; text-decoration:none }
@media (max-width:760px) { .approval-shell { padding:20px 14px }.approval-header { align-items:start }.task-badge { display:none }.approval-grid { grid-template-columns:1fr; border-radius:4px 16px 4px 4px }.source-rail { padding:24px }.source-rail dl { display:grid; grid-template-columns:90px 1fr; gap:7px }.source-rail dt { margin:0 }.source-rail dd { margin:0 }.deadline { margin-top:20px }.approval-panel { padding:26px 20px }.action-row { flex-direction:column-reverse }.action-row button { width:100% } }
</style>
