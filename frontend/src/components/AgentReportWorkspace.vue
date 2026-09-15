<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { agentTaskClient, type AgentTaskState } from '@/api/agentTasks'
import type { Message } from '@/types'

const props = defineProps<{ messages: Message[]; ownerId: string; conversationName: string; group: boolean; agentId: string }>()
const emit = defineEmits<{ create: [content: string]; close: [] }>()
const query = ref('总结本会话的进展、决策、风险，列出消息依据；信息不足时向我提问。')
const deadline = ref('')
const error = ref('')
const submitting = ref(false)
const tasks = ref<{ id: string; title: string }[]>([])
const selected = ref('')
const state = ref<AgentTaskState>()
const values = ref<Record<string, string>>({})
const loading = ref(false)
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined
const pending = computed(() => state.value?.pending)
const labels: Record<string, string> = { created: '准备执行', running: '正在执行', waiting_input: '等待你的补充', waiting_approval: '等待确认', completed: '已完成', cancelled: '已取消', failed: '执行失败' }

function create() {
  const at = new Date(deadline.value).getTime()
  if (!query.value.trim() || query.value.length > 500 || !Number.isFinite(at) || at <= Date.now() || at > Date.now() + 7 * 86400000) {
    error.value = '请填写 500 字以内的目标，并选择未来七天内的补充截止时间。'; return
  }
  error.value = ''
  emit('create', `${props.group ? '@AI ' : ''}/report ${new Date(at).toISOString()} ${query.value.trim()}`)
}

watch(() => [props.messages, props.ownerId, props.agentId, props.group] as const, async () => {
  const candidates = props.messages.filter(message => {
    if (message.from_uuid !== props.ownerId) return false
    return props.group ? /@(?:Dipole\s+AI|AI)\b/i.test(message.content) : true
  }).slice(-24).reverse()
  const current = ++generation
  try {
    const result: { id: string; title: string }[] = []
    for (const message of candidates) {
      const material = ['dipole.agent.policy.persistence.v1', 'dipole', props.agentId,
        props.group ? 'message.group.created' : 'message.direct.created', message.message_id].join('\n')
      const bytes = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(material))
      const hex = [...new Uint8Array(bytes)].map(n => n.toString(16).padStart(2, '0')).join('')
      result.push({ id: `task:${hex.slice(0, 59)}`, title: message.content.replace(/^(?:@(?:Dipole\s+AI|AI)\s+)?\/report\s+\S+\s*/i, '') })
    }
    if (current !== generation) return
    tasks.value = result
    if (!result.some(t => t.id === selected.value)) selected.value = result[0]?.id ?? ''
  } catch { error.value = '任务索引需要 HTTPS 或 localhost 安全上下文。' }
}, { immediate: true, deep: true })

async function refresh() {
  clearTimeout(timer)
  const id = selected.value
  if (!id) return
  loading.value = true
  try {
    const next = await agentTaskClient.getTask(id)
    if (id !== selected.value) return
    if (next.pending?.requestId !== state.value?.pending?.requestId) values.value = {}
    state.value = next
    error.value = ''
  } catch { if (id === selected.value) error.value = '任务正在接入，或服务暂不可用。将自动重试，你的输入会保留。' }
  finally {
    loading.value = false
    if (id === selected.value) timer = setTimeout(refresh, 3000)
  }
}
watch(selected, () => { state.value = undefined; values.value = {}; void refresh() })
async function act(action: 'input' | 'approved' | 'denied' | 'cancel') {
  if (!state.value || submitting.value) return
  submitting.value = true
  try {
    if (action === 'input' && pending.value?.kind === 'input') await agentTaskClient.provideInput(selected.value, pending.value.requestId, values.value)
    else if (action === 'cancel') await agentTaskClient.cancelTask(selected.value)
    else if ((action === 'approved' || action === 'denied') && pending.value?.kind === 'approval') await agentTaskClient.resolveApproval(selected.value, pending.value.approvalId, action)
    await refresh()
  } catch { error.value = '操作未确认成功，请刷新状态后重试。' }
  finally { submitting.value = false }
}
onBeforeUnmount(() => { generation++; clearTimeout(timer); selected.value = '' })
</script>

<template>
  <section class="report-workspace" aria-label="Agent 任务">
    <header><div><small>DIPOLE AGENT</small><h2>会话中的任务与执行记录</h2><p>{{ conversationName }} · 检索 → 补充 → 确认 → 完成</p></div><button aria-label="关闭任务面板" @click="emit('close')">关闭</button></header>
    <div class="report-columns">
      <aside>
        <form @submit.prevent="create">
          <label>总结目标<textarea v-model="query" maxlength="500" rows="4" required /></label>
          <label>补充信息截止时间<input v-model="deadline" type="datetime-local" required /></label>
          <p class="hint">使用本地时区。超时将标注未知项继续生成，发布仍需你确认。截止时间内最多询问一轮。</p>
          <button class="primary" type="submit">创建协作总结</button>
        </form>
        <h3>当前会话中的 Agent 任务</h3>
        <p v-if="!tasks.length" class="hint">向 AI 发送消息或在群里 @AI 后，任务会出现在这里；更早任务可先加载聊天历史。</p>
        <button v-for="task in tasks" :key="task.id" class="task-item" :class="{ selected: selected === task.id }" @click="selected = task.id">{{ task.title }}</button>
      </aside>
      <main aria-live="polite">
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <template v-if="state">
          <div class="status"><strong>{{ labels[state.status] }}</strong><small>{{ loading ? '同步中' : '每 3 秒自动更新' }}</small></div>
          <p class="hint">任务会保存进度；等待期间可以离开页面。</p>
          <template v-if="pending?.kind === 'input'">
            <pre>{{ pending.prompt }}</pre>
            <p class="hint">有效期至 {{ new Date(pending.expiresAtUnixMs).toLocaleString() }}</p>
            <form @submit.prevent="act('input')">
              <label v-for="field in pending.form.fields" :key="field.id">{{ field.label }}<textarea v-model="values[field.id]" :maxlength="field.type === 'text' ? field.maxLength : 1800" :required="field.required" rows="5" /></label>
              <button class="primary" :disabled="submitting" type="submit">提交并继续</button>
            </form>
          </template>
          <template v-else-if="pending?.kind === 'approval'">
            <pre>{{ pending.summary }}</pre><p class="hint">确认后将执行该操作；拒绝不会发送或修改内容。</p>
            <div class="actions"><button :disabled="submitting" @click="act('denied')">拒绝操作</button><button class="primary" :disabled="submitting" @click="act('approved')">确认操作</button></div>
          </template>
          <p v-else-if="state.status === 'completed'">任务已完成，结果可在当前会话的消息记录中查看。</p>
          <p v-else-if="state.status === 'failed'">任务未完成。已产生的消息不会自动撤销，请检查会话记录。</p>
          <button v-if="!['completed','failed','cancelled'].includes(state.status)" class="cancel" :disabled="submitting" @click="act('cancel')">取消任务</button>
          <details><summary>执行记录</summary><p class="task-id">{{ state.taskId }}</p><a :href="`/app/agent/tasks/${encodeURIComponent(state.taskId)}/timeline`">查看时间线</a></details>
        </template>
        <div v-else class="empty"><h3>{{ selected ? '正在接入任务' : '从一个明确的问题开始' }}</h3><p>Agent 会读取授权会话，向你核实缺失信息，并在发布前展示完整草稿。</p></div>
      </main>
    </div>
  </section>
</template>

<style scoped>
.report-workspace { position: absolute; inset: 64px 0 0; z-index: 15; overflow: auto; background: radial-gradient(ellipse at top right,var(--dp-accent-soft),transparent 55%),var(--dp-canvas); color: var(--dp-ink); padding: 24px; font-family: var(--dp-font-body); }
header { display:flex; justify-content:space-between; gap:16px; align-items:start; margin-bottom:24px; } h2 { font-family:var(--dp-font-display); font-size:25px; margin:8px 0; } small,.hint,header p { color:var(--dp-ink-soft); font-size:12px; line-height:1.7; }
.report-columns { display:grid; grid-template-columns:minmax(230px,1fr) minmax(280px,2fr); gap:24px; } aside,main { min-width:0; padding:20px; background:var(--dp-surface); border:1px solid var(--dp-line); border-radius:12px; } label { display:block; font-size:13px; margin-bottom:16px; } textarea,input { display:block; box-sizing:border-box; width:100%; margin-top:8px; padding:10px; border:1px solid var(--dp-line); border-radius:6px; background:var(--dp-canvas); color:inherit; font:inherit; } textarea { resize:vertical; } button { padding:9px 14px; border:1px solid var(--dp-line); border-radius:6px; background:var(--dp-surface); color:inherit; cursor:pointer; } button:disabled { opacity:.5; cursor:wait; } .primary { background:var(--dp-accent-strong); color:var(--dp-text-inverse); } .task-item { display:block; width:100%; text-align:left; margin-top:8px; overflow-wrap:anywhere; } .selected { border-color:var(--dp-accent-strong); background:var(--dp-accent-soft); } .status,.actions { display:flex; justify-content:space-between; gap:12px; } pre { white-space:pre-wrap; overflow-wrap:anywhere; line-height:1.8; font:inherit; padding:16px; background:var(--dp-canvas); border-left:3px solid var(--dp-accent-strong); } .error { color:var(--dp-danger); } .cancel,details { margin-top:24px; } .task-id { overflow-wrap:anywhere; font-size:11px; } .empty { padding:40px 0; line-height:1.8; } button:focus-visible,input:focus-visible,textarea:focus-visible { outline:2px solid var(--dp-accent-strong); outline-offset:3px; }
@media(max-width:760px) { .report-workspace { padding:12px; } .report-columns { grid-template-columns:1fr; gap:12px; } header h2 { font-size:20px; } }
</style>
