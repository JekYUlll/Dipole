<template>
  <section class="elicitation-shell" :data-agent-elicit-state="viewState" :aria-busy="busy" aria-live="polite">
    <header class="elicitation-header">
      <div>
        <p class="eyebrow">DIPOLE AGENT / INPUT REQUEST</p>
        <h1>补充任务信息</h1>
      </div>
      <span class="task-badge">{{ shortTaskId }}</span>
    </header>

    <div v-if="viewState === 'loading'" class="state-card" role="status">
      <span class="pulse-dot" aria-hidden="true"></span>
      正在确认当前输入请求
    </div>

    <div v-else-if="viewState === 'unavailable'" class="state-card state-card-danger" role="alert">
      <p class="state-title">无法确认当前输入请求</p>
      <p>为避免向失效任务提交数据，表单已暂时隐藏。</p>
      <button type="button" class="secondary-button" data-agent-retry @click="loadTask">重新确认</button>
    </div>

    <div v-else-if="pending" class="elicitation-grid">
      <aside class="source-rail">
        <p class="rail-label">REQUEST SOURCE</p>
        <dl class="binding-list">
          <dt>Task</dt><dd>{{ task?.taskId }}</dd>
          <dt>Revision</dt><dd>{{ task?.revision }}</dd>
        </dl>
        <template v-if="pending.source.kind === 'mcp'">
          <span class="trust-chip">UNTRUSTED MCP</span>
          <dl>
            <dt>Server</dt><dd>{{ pending.source.serverId }}</dd>
            <dt>Tool</dt><dd>{{ pending.source.toolName }}</dd>
            <dt>Invocation</dt><dd>{{ pending.source.invocationId }}</dd>
          </dl>
          <p class="source-note">来源仅用于披露，不授予工具额外权限。</p>
        </template>
        <template v-else>
          <span class="trust-chip local">DIPOLE AGENT</span>
          <p class="source-note">此请求由当前 Agent Task 发起。</p>
        </template>
        <div class="deadline">
          <span>有效期</span>
          <strong>{{ deadlineLabel }}</strong>
        </div>
      </aside>

      <form class="form-panel" data-agent-submit @submit.prevent="submit">
        <div class="prompt-block">
          <p class="rail-label">AGENT ASKS</p>
          <h2>{{ pending.prompt }}</h2>
          <p>请确认下列普通信息。此表单不接受密码、令牌或支付凭据。</p>
        </div>

        <div class="fields">
          <fieldset v-for="field in pending.form.fields" :key="field.id" class="field-group">
            <legend :id="fieldLegendId(field.id)">
              {{ field.label }}
              <span v-if="field.required" class="required">必填</span>
            </legend>

            <input
              v-if="field.type === 'text'"
              v-model="values[field.id] as string"
              :id="fieldControlId(field.id)"
              :data-agent-field="field.id"
              :maxlength="field.maxLength"
              :aria-labelledby="fieldLegendId(field.id)"
              :aria-describedby="fieldAriaDescribedBy(field.id)"
              :aria-invalid="Boolean(errors[field.id])"
              type="text"
              autocomplete="off"
            />

            <div v-else-if="field.type === 'select'" class="select-wrap">
              <select
                v-model="values[field.id] as string"
                :id="fieldControlId(field.id)"
                :data-agent-field="field.id"
                :aria-labelledby="fieldLegendId(field.id)"
                :aria-describedby="fieldAriaDescribedBy(field.id)"
                :aria-invalid="Boolean(errors[field.id])"
              >
                <option value="" disabled>请选择</option>
                <option v-for="option in field.options" :key="option" :value="option">{{ option }}</option>
              </select>
            </div>

            <div v-else-if="field.type === 'multiselect'" class="option-grid">
              <label v-for="(option, optionIndex) in field.options" :key="option" class="check-option" :for="fieldOptionId(field.id, optionIndex)">
                <input
                  v-model="values[field.id] as string[]"
                  :id="fieldOptionId(field.id, optionIndex)"
                  :data-agent-field="field.id"
                  :value="option"
                  :aria-describedby="fieldAriaDescribedBy(field.id)"
                  :aria-invalid="Boolean(errors[field.id])"
                  type="checkbox"
                />
                <span>{{ option }}</span>
              </label>
              <small v-if="field.maxSelections">最多选择 {{ field.maxSelections }} 项</small>
            </div>

            <label v-else class="boolean-option" :for="fieldControlId(field.id)">
              <input
                v-model="values[field.id] as boolean"
                :id="fieldControlId(field.id)"
                :data-agent-field="field.id"
                :aria-describedby="fieldAriaDescribedBy(field.id)"
                :aria-invalid="Boolean(errors[field.id])"
                type="checkbox"
              />
              <span>启用</span>
            </label>

            <p v-if="errors[field.id]" :id="fieldErrorId(field.id)" class="field-error" role="alert">{{ errors[field.id] }}</p>
          </fieldset>
        </div>

        <div class="action-row">
          <button type="button" class="secondary-button" :disabled="busy" @click="cancel">取消任务</button>
          <button type="submit" class="primary-button" :disabled="busy || expired">
            {{ viewState === 'submitting' ? '正在提交' : expired ? '请求已过期' : '提交并继续任务' }}
          </button>
        </div>
      </form>
    </div>

    <div v-else class="state-card state-card-complete">
      <p class="state-title">{{ terminalTitle }}</p>
      <p>{{ terminalDescription }}</p>
      <a href="/app/" class="secondary-button link-button">返回消息</a>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  agentTaskClient,
  type AgentElicitationField,
  type AgentElicitationValue,
  type AgentInputPending,
  type AgentTaskClient,
  type AgentTaskState,
} from '@/api/agentTasks'

type ViewState = 'loading' | 'waiting_input' | 'validation_error' | 'submitting' | 'running' | 'cancelled' | 'expired' | 'unavailable' | 'completed' | 'failed'

const props = withDefaults(defineProps<{
  taskId: string
  client?: AgentTaskClient
  now?: () => number
}>(), {
  client: () => agentTaskClient,
  now: () => Date.now,
})

const task = ref<AgentTaskState>()
const pending = ref<AgentInputPending>()
const viewState = ref<ViewState>('loading')
const values = reactive<AgentElicitationValue>({})
const errors = reactive<Record<string, string>>({})
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
  running: '信息已接收，任务继续执行', cancelled: '任务已取消', expired: '输入请求已过期',
  completed: '任务已完成', failed: '任务执行失败',
}
const terminalTitle = computed(() => terminalTitles[viewState.value] ?? '当前无需补充信息')
const terminalDescription = computed(() => viewState.value === 'running'
  ? 'Agent 已恢复执行，可返回消息页面等待结果。'
  : '当前页面不会再接受这次请求的数据。')

onMounted(() => {
  void loadTask()
  clockTimer = setInterval(() => {
    clock.value = props.now()
    if (expired.value && pending.value) void loadTask()
  }, 1_000)
})

onBeforeUnmount(() => {
  if (clockTimer) clearInterval(clockTimer)
})

async function loadTask(): Promise<void> {
  viewState.value = 'loading'
  pending.value = undefined
  clearObject(values)
  clearObject(errors)
  try {
    const current = await props.client.getTask(props.taskId)
    task.value = current
    applyTask(current)
  } catch {
    task.value = undefined
    pending.value = undefined
    viewState.value = 'unavailable'
  }
}

function applyTask(current: AgentTaskState): void {
  if (current.status === 'waiting_input' && current.pending?.kind === 'input') {
    if (props.now() >= current.pending.expiresAtUnixMs) {
      viewState.value = 'expired'
      return
    }
    pending.value = current.pending
    initializeValues(current.pending)
    viewState.value = 'waiting_input'
    return
  }
  pending.value = undefined
  if (current.status === 'cancelled' && current.cancellation?.reason === 'input_expired') viewState.value = 'expired'
  else if (current.status === 'cancelled') viewState.value = 'cancelled'
  else if (current.status === 'completed') viewState.value = 'completed'
  else if (current.status === 'failed') viewState.value = 'failed'
  else viewState.value = 'running'
}

function initializeValues(input: AgentInputPending): void {
  clearObject(values)
  for (const field of input.form.fields) {
    values[field.id] = field.type === 'multiselect' ? [] : field.type === 'boolean' ? false : ''
  }
}

async function submit(): Promise<void> {
  const current = pending.value
  if (!current || expired.value || busy.value) return
  clearObject(errors)
  for (const field of current.form.fields) validateField(field)
  if (Object.keys(errors).length > 0) {
    viewState.value = 'validation_error'
    await nextTick()
    const firstInvalid = current.form.fields.find(field => errors[field.id] !== undefined)
    if (firstInvalid) document.getElementById(fieldFocusId(firstInvalid))?.focus()
    return
  }
  viewState.value = 'submitting'
  try {
    await props.client.provideInput(props.taskId, current.requestId, snapshotValues(current))
    await loadTask()
  } catch {
    pending.value = undefined
    clearObject(values)
    viewState.value = 'unavailable'
  }
}

async function cancel(): Promise<void> {
  if (busy.value) return
  viewState.value = 'submitting'
  try {
    await props.client.cancelTask(props.taskId)
    await loadTask()
  } catch {
    pending.value = undefined
    clearObject(values)
    viewState.value = 'unavailable'
  }
}

function validateField(field: AgentElicitationField): void {
  const value = values[field.id]
  if (field.required && (value === '' || (Array.isArray(value) && value.length === 0))) {
    errors[field.id] = `${field.label}为必填项`
    return
  }
  if (field.type === 'text' && typeof value === 'string' && value.length > (field.maxLength ?? 1_000)) {
    errors[field.id] = `${field.label}超过长度限制`
  }
  if (field.type === 'multiselect' && Array.isArray(value) && value.length > (field.maxSelections ?? field.options.length)) {
    errors[field.id] = `${field.label}最多选择 ${field.maxSelections ?? field.options.length} 项`
  }
}

function snapshotValues(input: AgentInputPending): AgentElicitationValue {
  return Object.fromEntries(input.form.fields.map(field => {
    const value = values[field.id]
    return [field.id, Array.isArray(value) ? [...value] : value]
  })) as AgentElicitationValue
}

function fieldControlId(fieldId: string): string {
  return `agent-elicit-${fieldId}`
}

function fieldLegendId(fieldId: string): string {
  return `${fieldControlId(fieldId)}-label`
}

function fieldErrorId(fieldId: string): string {
  return `${fieldControlId(fieldId)}-error`
}

function fieldOptionId(fieldId: string, optionIndex: number): string {
  return `${fieldControlId(fieldId)}-${optionIndex}`
}

function fieldFocusId(field: AgentElicitationField): string {
  return field.type === 'multiselect' ? fieldOptionId(field.id, 0) : fieldControlId(field.id)
}

function fieldAriaDescribedBy(fieldId: string): string | undefined {
  return errors[fieldId] ? fieldErrorId(fieldId) : undefined
}

function clearObject(value: Record<string, unknown>): void {
  for (const key of Object.keys(value)) delete value[key]
}
</script>

<style scoped>
.elicitation-shell {
  --ink: #17221d;
  --muted: #68736d;
  --paper: #f5f0e6;
  --cream: #fffdf7;
  --line: #d9d1c2;
  --accent: #d4532f;
  --forest: #193d32;
  min-height: 100vh;
  padding: clamp(24px, 5vw, 64px);
  color: var(--ink);
  background:
    radial-gradient(circle at 82% 4%, rgba(212, 83, 47, .14), transparent 27%),
    repeating-linear-gradient(90deg, transparent 0 79px, rgba(25, 61, 50, .035) 80px),
    var(--paper);
  font-family: Manrope, "Noto Sans SC", sans-serif;
}
.elicitation-header { max-width: 1120px; margin: 0 auto 24px; display: flex; align-items: end; justify-content: space-between; gap: 24px; }
.elicitation-header h1 { margin: 4px 0 0; font-family: Georgia, "Noto Serif SC", serif; font-size: clamp(30px, 5vw, 52px); font-weight: 500; letter-spacing: -.04em; }
.eyebrow, .rail-label { margin: 0; color: var(--accent); font-size: 11px; font-weight: 800; letter-spacing: .16em; }
.task-badge { padding: 8px 11px; border: 1px solid var(--line); border-radius: 999px; color: var(--muted); background: rgba(255,255,255,.45); font: 700 11px ui-monospace, monospace; }
.elicitation-grid { max-width: 1120px; margin: 0 auto; display: grid; grid-template-columns: minmax(240px, 320px) 1fr; overflow: hidden; border: 1px solid rgba(23,34,29,.16); border-radius: 6px 24px 6px 6px; box-shadow: 0 26px 70px rgba(40, 48, 39, .14); }
.source-rail { padding: 32px; color: #eef4ef; background: var(--forest); }
.source-rail .rail-label { color: #91bba8; }
.trust-chip { display: inline-flex; margin: 26px 0; padding: 6px 9px; border: 1px solid #ef8f6e; border-radius: 3px; color: #ffc0a8; font-size: 10px; font-weight: 800; letter-spacing: .1em; }
.trust-chip.local { border-color: #82b49f; color: #b8decd; }
.source-rail dl { margin: 0; }
.binding-list { margin-top: 24px !important; padding-bottom: 18px; border-bottom: 1px solid rgba(255,255,255,.14); }
.source-rail dt { margin-top: 18px; color: #91a99e; font-size: 10px; text-transform: uppercase; letter-spacing: .12em; }
.source-rail dd { margin: 4px 0 0; overflow-wrap: anywhere; font: 600 12px ui-monospace, monospace; }
.source-note { margin: 28px 0; color: #b9c8c1; font-size: 12px; line-height: 1.6; }
.deadline { margin-top: 36px; padding-top: 20px; border-top: 1px solid rgba(255,255,255,.14); display: flex; justify-content: space-between; color: #9fb4aa; font-size: 11px; }
.deadline strong { color: #fff; }
.form-panel { padding: clamp(28px, 5vw, 56px); background: var(--cream); }
.prompt-block { padding-bottom: 26px; border-bottom: 1px solid var(--line); }
.prompt-block h2 { margin: 10px 0; font-family: Georgia, "Noto Serif SC", serif; font-size: clamp(22px, 3vw, 34px); line-height: 1.22; font-weight: 500; }
.prompt-block > p:last-child { margin: 0; color: var(--muted); font-size: 12px; }
.fields { display: grid; gap: 24px; margin-top: 30px; }
.field-group { min-width: 0; margin: 0; padding: 0; border: 0; }
.field-group legend { width: 100%; margin-bottom: 9px; font-size: 13px; font-weight: 750; }
.required { margin-left: 7px; color: var(--accent); font-size: 10px; font-weight: 700; }
input[type="text"], select { box-sizing: border-box; width: 100%; height: 44px; padding: 0 13px; border: 1px solid var(--line); border-radius: 3px; color: var(--ink); background: #fff; font: inherit; outline: none; }
input[type="text"]:focus, select:focus { border-color: var(--forest); box-shadow: 0 0 0 3px rgba(25,61,50,.09); }
button:focus-visible, .link-button:focus-visible, input:focus-visible, select:focus-visible { outline: 3px solid rgba(212,83,47,.35); outline-offset: 3px; }
.select-wrap { position: relative; }
.select-wrap::after { content: "↓"; position: absolute; right: 13px; top: 12px; pointer-events: none; color: var(--accent); }
select { appearance: none; }
.option-grid { display: flex; flex-wrap: wrap; gap: 8px; }
.check-option, .boolean-option { display: inline-flex; align-items: center; gap: 8px; padding: 10px 12px; border: 1px solid var(--line); border-radius: 3px; background: #fff; cursor: pointer; }
.option-grid small { width: 100%; color: var(--muted); }
.field-error { margin: 7px 0 0; color: #ac321f; font-size: 12px; }
.action-row { display: flex; justify-content: flex-end; gap: 10px; margin-top: 38px; padding-top: 24px; border-top: 1px solid var(--line); }
button, .link-button { min-height: 42px; padding: 0 17px; border-radius: 3px; font: 750 12px inherit; cursor: pointer; }
.primary-button { border: 1px solid var(--accent); color: white; background: var(--accent); }
.secondary-button { border: 1px solid var(--line); color: var(--ink); background: transparent; }
button:disabled { cursor: not-allowed; opacity: .55; }
.state-card { max-width: 760px; margin: 80px auto; padding: 34px; border: 1px solid var(--line); border-radius: 4px 18px 4px 4px; background: var(--cream); box-shadow: 0 20px 55px rgba(40,48,39,.11); }
.state-card-danger { border-left: 5px solid var(--accent); }
.state-card-complete { border-left: 5px solid #52856e; }
.state-title { margin: 0 0 8px; font-family: Georgia, "Noto Serif SC", serif; font-size: 22px; }
.state-card > p:not(.state-title) { color: var(--muted); }
.state-card .secondary-button { margin-top: 12px; }
.link-button { display: inline-flex; align-items: center; text-decoration: none; }
.pulse-dot { display: inline-block; width: 8px; height: 8px; margin-right: 8px; border-radius: 50%; background: var(--accent); animation: pulse 1.1s infinite alternate; }
@keyframes pulse { to { opacity: .25; transform: scale(.7); } }
@media (max-width: 760px) {
  .elicitation-shell { padding: 20px 14px; }
  .elicitation-header { align-items: start; }
  .task-badge { display: none; }
  .elicitation-grid { grid-template-columns: 1fr; border-radius: 4px 16px 4px 4px; }
  .source-rail { padding: 24px; }
  .source-rail dl { display: grid; grid-template-columns: 90px 1fr; gap: 7px; }
  .source-rail dt { margin: 0; }
  .source-rail dd { margin: 0; }
  .source-note, .deadline { margin-top: 20px; }
  .form-panel { padding: 26px 20px; }
  .action-row { flex-direction: column-reverse; }
  .action-row button { width: 100%; }
}
</style>
