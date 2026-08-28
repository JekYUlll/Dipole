<template>
  <section class="memory-shell" :data-agent-memory-state="viewState" :aria-busy="busy">
    <aside class="control-rail" aria-label="Agent control navigation">
      <div class="brand"><span />DIPOLE</div>
      <p class="rail-kicker">AGENT CONTROL</p>
      <div class="rail-active">◉ <span>长期记忆</span></div>
      <div class="rail-item">▣ <span>Agent 定义</span></div>
      <div class="rail-item">⌁ <span>事件订阅</span></div>
      <div class="rail-item">☷ <span>任务运行</span></div>
      <div class="rail-item">♢ <span>审批记录</span></div>
      <p class="rail-boundary">OWNER GOVERNANCE<br>AUTO OBSERVATION: OFF<br>RETRIEVAL: CONTEXT ONLY</p>
    </aside>

    <main class="memory-main">
      <header class="page-header">
        <div>
          <p class="eyebrow">OWNER VIEW / MEMORY AUTHORITY</p>
          <h1>长期记忆</h1>
          <p class="subtitle">检查 Agent 保存的内容、来源与作用域；撤销后立即退出上下文召回。</p>
        </div>
        <div class="auto-status">⌾ <strong>AUTO WRITE&nbsp; OFF</strong></div>
      </header>

      <div class="trust-notice" role="note">
        <strong>UNTRUSTED MEMORY</strong>
        <span>记忆不会改变系统身份、权限或工具策略。自动 Observation 写入保持关闭。</span>
      </div>

      <div v-if="viewState === 'loading'" class="state-card" role="status">
        <span class="spinner" /><p class="state-code">LOADING</p><h2>正在读取长期记忆</h2><p>只显示当前认证 principal 的权威记录。</p>
      </div>
      <div v-else-if="viewState === 'unavailable'" class="state-card danger" role="alert">
        <p class="state-code">UNAVAILABLE</p><h2>记忆控制面暂时不可用</h2><p>已清空旧记录并关闭撤销动作，避免展示未经确认的缓存状态。</p>
        <button data-agent-memory-retry class="text-action danger-text" @click="load(true)">重新确认 →</button>
      </div>
      <div v-else-if="viewState === 'conflict'" class="state-card warning" role="alert">
        <p class="state-code">AUTHORITY CHANGED</p><h2>记忆状态已经变化</h2><p>撤销结果未被本地推断，请重新读取 Core 权威状态。</p>
        <button data-agent-memory-retry class="text-action warning-text" @click="load(true)">重新读取 →</button>
      </div>
      <div v-else-if="memories.length === 0" class="state-card" role="status">
        <p class="state-code success">EMPTY</p><h2>还没有长期记忆</h2><p>当前不会自动生成 Observation 或 Reflection。</p>
      </div>

      <template v-else>
        <div class="list-heading"><h2>MEMORY RECORDS&nbsp; {{ String(memories.length).padStart(2, '0') }}</h2><span>CREATED DESC ↓</span></div>
        <div class="content-grid">
          <div class="memory-list">
            <article v-for="item in memories" :key="item.memoryId" class="memory-card" :class="{ inactive: inactive(item) }" :data-agent-memory-id="item.memoryId">
              <div class="card-top">
                <div>
                  <h3>{{ item.content }}</h3>
                  <p class="mono">{{ item.memoryType.toUpperCase() }} · AGENT {{ item.agentId }} · PRIORITY {{ item.priority }}</p>
                </div>
                <span class="status-pill" :class="statusClass(item)"><i />{{ statusLabel(item) }}</span>
              </div>
              <p class="preview">{{ item.compactContent || item.content }}</p>
              <div class="card-bottom">
                <span class="mono">SOURCE {{ item.provenance.sourceType }}:{{ item.provenance.sourceId }}<template v-if="item.provenance.sequence"> · SEQ {{ item.provenance.sequence }}</template></span>
                <button v-if="!inactive(item)" :data-agent-memory-revoke="item.memoryId" class="text-action" @click="openRevoke(item)">查看来源与控制 →</button>
                <span v-else class="audit-copy">{{ item.revokeReason || 'expired' }}</span>
              </div>
            </article>
            <button v-if="nextCursor" class="load-more" :disabled="busy" @click="loadMore">加载下一页 →</button>
          </div>

          <aside class="authority-panel">
            <p class="eyebrow">OWNER AUTHORITY</p>
            <h2>可见、可追溯、可撤销</h2>
            <p>内容与来源对所有者可见。撤销会保留历史记录和审计信息，并从后续上下文编译中排除。</p>
            <dl><dt>WRITE</dt><dd>automatic off</dd><dt>TRUST</dt><dd>untrusted input</dd><dt>DELETE</dt><dd>audited revoke</dd></dl>
            <div class="correction-boundary"><strong>纠正将在版本化写入后开放</strong><br>当前不允许原地覆盖，避免来源链与审计历史丢失。</div>
          </aside>
        </div>
      </template>
    </main>

    <div v-if="selected" class="dialog-backdrop" @click.self="closeRevoke" @keydown.esc="closeRevoke">
      <section role="dialog" aria-modal="true" aria-labelledby="memory-revoke-title" class="revoke-dialog">
        <div class="sheet-handle" />
        <p class="eyebrow danger-text">DESTRUCTIVE OWNER ACTION</p>
        <h2 id="memory-revoke-title">撤销这条记忆？</h2>
        <p>撤销后立即退出上下文召回。历史内容与来源链继续保留，便于审计。</p>
        <label for="memory-reason">撤销原因（必填）</label>
        <textarea id="memory-reason" ref="reasonInput" v-model="reason" data-agent-memory-reason maxlength="1000" :aria-invalid="reasonError ? 'true' : 'false'" />
        <p v-if="reasonError" class="field-error" role="alert">{{ reasonError }}</p>
        <div class="dialog-actions">
          <button class="cancel-button" :disabled="viewState === 'revoking'" @click="closeRevoke">取消</button>
          <button data-agent-memory-confirm class="confirm-button" :disabled="viewState === 'revoking'" @click="confirmRevoke">{{ viewState === 'revoking' ? '正在撤销…' : '确认撤销' }}</button>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { agentMemoryClient, type AgentMemory, type AgentMemoryClient } from '@/api/agentMemories'

const props = withDefaults(defineProps<{ client?: AgentMemoryClient }>(), { client: () => agentMemoryClient })
type ViewState = 'loading' | 'ready' | 'unavailable' | 'revoking' | 'conflict'
const viewState = ref<ViewState>('loading')
const memories = ref<AgentMemory[]>([])
const nextCursor = ref('')
const selected = ref<AgentMemory>()
const reason = ref('')
const reasonError = ref('')
const reasonInput = ref<HTMLTextAreaElement>()
const busy = computed(() => viewState.value === 'loading' || viewState.value === 'revoking')

onMounted(() => load(false))

async function load(reset: boolean) {
  viewState.value = 'loading'
  if (reset) { memories.value = []; nextCursor.value = '' }
  try {
    const page = await props.client.list('', 50)
    memories.value = page.memories
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    memories.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

async function loadMore() {
  if (!nextCursor.value || busy.value) return
  viewState.value = 'loading'
  try {
    const page = await props.client.list(nextCursor.value, 50)
    const seen = new Set(memories.value.map(item => item.memoryId))
    if (page.memories.some(item => seen.has(item.memoryId))) throw new Error('duplicate Agent Memory page')
    memories.value.push(...page.memories)
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    memories.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  }
}

function openRevoke(item: AgentMemory) {
  if (inactive(item) || busy.value) return
  selected.value = item
  reason.value = ''
  reasonError.value = ''
  nextTick(() => reasonInput.value?.focus())
}

function closeRevoke() {
  if (viewState.value === 'revoking') return
  selected.value = undefined
  reason.value = ''
  reasonError.value = ''
}

async function confirmRevoke() {
  if (!selected.value) return
  const normalized = reason.value.trim()
  if (!normalized) { reasonError.value = '请输入撤销原因'; return }
  if ([...normalized].length > 1000 || /[\u0000-\u001f\u007f]/u.test(normalized)) { reasonError.value = '撤销原因格式无效'; return }
  viewState.value = 'revoking'
  try {
    const authoritative = await props.client.revoke(selected.value.memoryId, normalized)
    const index = memories.value.findIndex(item => item.memoryId === authoritative.memoryId)
    if (index < 0 || authoritative.status !== 'revoked') throw new Error('invalid authoritative revoke')
    memories.value[index] = authoritative
    selected.value = undefined
    reason.value = ''
    viewState.value = 'ready'
  } catch {
    selected.value = undefined
    viewState.value = 'conflict'
  }
}

function inactive(item: AgentMemory) {
  return item.status === 'revoked' || (item.expiresAtUnixMs !== undefined && item.expiresAtUnixMs <= Date.now())
}
function statusLabel(item: AgentMemory) {
  if (item.status === 'revoked') return 'REVOKED'
  if (item.expiresAtUnixMs !== undefined && item.expiresAtUnixMs <= Date.now()) return 'EXPIRED'
  if (item.expiresAtUnixMs !== undefined && item.expiresAtUnixMs - Date.now() < 7 * 86_400_000) return 'ACTIVE / EXPIRES SOON'
  return 'ACTIVE / UNTRUSTED'
}
function statusClass(item: AgentMemory) { return inactive(item) ? 'status-danger' : 'status-warning' }
</script>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Geist+Mono:wght@500;700&family=Manrope:wght@500;700;800&family=Noto+Sans+SC:wght@400;600;700&display=swap');
.memory-shell{--ink:#172126;--muted:#69756f;--line:#dfe4e1;--panel:#fffefb;--app:#f4f6f8;--rail:#172126;--green:#008e5b;--green-soft:#e4f7ef;--amber:#a66a00;--amber-soft:#fff3d5;--red:#b64d47;--red-soft:#fde9e6;min-height:100vh;background:var(--app);color:var(--ink);display:grid;grid-template-columns:256px 1fr;font-family:"Noto Sans SC",sans-serif}.control-rail{background:var(--rail);color:#d7dfdb;padding:34px 26px;display:flex;flex-direction:column;gap:20px}.brand{font:800 19px Manrope,sans-serif;letter-spacing:.12em;display:flex;align-items:center;gap:12px}.brand>span{width:12px;height:12px;border-radius:50%;background:#00a86b}.rail-kicker,.rail-boundary,.mono,.state-code{font-family:"Geist Mono",monospace}.rail-kicker{font-size:10px;color:#89958f;letter-spacing:.12em;margin-top:8px}.rail-active,.rail-item{border-radius:10px;padding:12px 14px;display:flex;gap:10px;align-items:center;font-size:13px}.rail-active{background:#263238;color:white;font-weight:700}.rail-item{color:#9ca7a2}.rail-boundary{margin-top:auto;font-size:9px;line-height:1.9;color:#7f8c86}.memory-main{padding:36px 42px 64px;max-width:1420px;width:100%;margin:auto}.page-header{display:flex;justify-content:space-between;align-items:center;gap:24px}.eyebrow{color:var(--green);font:700 10px "Geist Mono",monospace;letter-spacing:.1em}.page-header h1{font:800 38px Manrope,sans-serif;letter-spacing:-.04em;margin:7px 0}.subtitle{color:var(--muted);font-size:13px}.auto-status{background:var(--panel);border:1px solid var(--line);border-radius:11px;padding:12px 15px;font:10px "Geist Mono",monospace;color:var(--muted)}.trust-notice{margin:26px 0;background:var(--amber-soft);border-radius:11px;padding:14px 17px;display:flex;gap:18px;align-items:center;font-size:12px}.trust-notice strong{color:var(--amber);font:700 10px "Geist Mono",monospace;white-space:nowrap}.state-card{max-width:720px;background:var(--panel);border:1px solid var(--line);border-radius:18px;padding:32px;margin:70px auto}.state-card h2{font:700 25px Manrope,sans-serif;margin:8px 0}.state-card>p:not(.state-code){color:var(--muted)}.state-card.danger{background:var(--red-soft);border-left:5px solid var(--red)}.state-card.warning{background:var(--amber-soft);border-left:5px solid var(--amber)}.state-code{font-size:10px;font-weight:700;color:var(--red)}.state-code.success{color:var(--green)}.spinner{display:block;width:24px;height:24px;border:3px solid var(--line);border-top-color:var(--green);border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}.list-heading{display:flex;justify-content:space-between;align-items:center;margin-bottom:14px}.list-heading h2{font:700 11px "Geist Mono",monospace;color:#7d8983;letter-spacing:.08em}.list-heading span{font:700 9px "Geist Mono",monospace;color:#99a39f}.content-grid{display:grid;grid-template-columns:minmax(0,1fr) 348px;gap:24px}.memory-list{display:grid;gap:14px}.memory-card{background:var(--panel);border:1px solid var(--line);border-radius:15px;padding:20px}.memory-card.inactive{opacity:.7}.card-top,.card-bottom{display:flex;justify-content:space-between;align-items:center;gap:18px}.card-top h3{font:800 17px Manrope,sans-serif;margin:0;max-width:560px}.mono{font-size:9px;color:#8b9691;margin-top:6px}.status-pill{display:inline-flex;align-items:center;gap:7px;border-radius:999px;padding:9px 12px;font:700 9px "Geist Mono",monospace;white-space:nowrap}.status-pill i{width:7px;height:7px;border-radius:50%;background:currentColor}.status-warning{background:var(--amber-soft);color:var(--amber)}.status-danger{background:var(--red-soft);color:var(--red)}.preview{color:var(--muted);font-size:12px;line-height:1.6;margin:17px 0}.text-action,.load-more{border:0;background:transparent;color:var(--green);font:700 11px inherit;cursor:pointer}.audit-copy{font-size:11px;color:var(--muted)}.authority-panel{background:var(--panel);border:1px solid var(--line);border-radius:15px;padding:23px;height:max-content}.authority-panel h2{font:800 21px Manrope,sans-serif;margin:8px 0}.authority-panel>p:not(.eyebrow){color:var(--muted);font-size:12px;line-height:1.7}.authority-panel dl{display:grid;grid-template-columns:80px 1fr;gap:10px;padding:18px 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line);margin:18px 0}.authority-panel dt{font:700 9px "Geist Mono",monospace;color:#929d98}.authority-panel dd{margin:0;font:600 10px "Geist Mono",monospace}.correction-boundary{background:var(--amber-soft);border-radius:10px;padding:14px;font-size:10px;line-height:1.6;color:var(--muted)}.correction-boundary strong{color:var(--amber)}.dialog-backdrop{position:fixed;inset:0;background:rgba(11,20,17,.45);display:flex;align-items:flex-end;justify-content:center;z-index:20}.revoke-dialog{width:min(560px,calc(100% - 32px));background:var(--panel);border-radius:22px 22px 0 0;padding:18px 26px 28px}.sheet-handle{width:42px;height:4px;border-radius:4px;background:var(--line);margin:0 auto 20px}.revoke-dialog h2{font:800 25px Manrope,sans-serif;margin:7px 0}.revoke-dialog>p:not(.eyebrow):not(.field-error){color:var(--muted);line-height:1.6}.revoke-dialog label{display:block;color:#8f9995;font:700 9px "Geist Mono",monospace;margin:20px 0 7px}.revoke-dialog textarea{width:100%;min-height:86px;resize:vertical;border:1px solid var(--line);border-radius:12px;background:#f5f6f3;padding:12px;font:inherit}.field-error,.danger-text{color:var(--red)}.warning-text{color:var(--amber)}.dialog-actions{display:flex;gap:10px;margin-top:20px}.dialog-actions button{flex:1;min-height:46px;border:0;border-radius:12px;font:700 13px inherit}.cancel-button{background:#f1f3f0;color:var(--ink)}.confirm-button{background:var(--red);color:white}button:disabled{opacity:.6}.revoke-dialog textarea:focus-visible,button:focus-visible{outline:3px solid rgba(0,142,91,.28);outline-offset:3px}
@media(max-width:900px){.memory-shell{grid-template-columns:1fr}.control-rail{display:none}.memory-main{padding:26px 20px 60px}.content-grid{grid-template-columns:1fr}.authority-panel{order:-1}.page-header{align-items:flex-start}.trust-notice{align-items:flex-start;flex-direction:column;gap:6px}}
@media(max-width:560px){.memory-main{padding:20px 16px 48px}.page-header h1{font-size:34px}.auto-status{font-size:8px}.authority-panel{display:none}.memory-card{padding:17px}.card-top{align-items:flex-start}.card-top h3{font-size:16px}.status-pill{padding:7px 9px}.card-bottom{align-items:flex-end}.revoke-dialog{width:100%;padding:14px 20px 24px}.trust-notice{margin:22px 0}}
</style>
