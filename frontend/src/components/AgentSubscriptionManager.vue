<template>
  <section class="subscription-shell" :data-agent-subscription-state="viewState" :aria-busy="busy">
    <aside class="control-rail" aria-label="Agent control navigation">
      <div class="brand"><span class="brand-dot" />DIPOLE</div>
      <p class="rail-kicker">AGENT CONTROL</p>
      <div class="rail-active">⌁ <span>事件订阅</span></div>
      <div class="rail-item">▣ <span>Agent 定义</span></div>
      <div class="rail-item">☷ <span>任务运行</span></div>
      <div class="rail-item">♢ <span>审批记录</span></div>
      <p class="rail-boundary">OWNER CONTROL<br>RUNTIME: DIRECT_TARGET<br>高成本模型调用未启用</p>
    </aside>

    <main class="subscription-main">
      <header class="page-header">
        <div>
          <p class="eyebrow">PROJECT GUARDIAN / EVENT WATCH</p>
          <h1>事件订阅</h1>
          <p class="subtitle">查看当前账号创建的订阅，以及触发前采用的确定性过滤规则。</p>
        </div>
        <button data-agent-subscription-create class="create-button" disabled title="等待经过鉴权的 Definition 目录">
          <span>＋</span> 创建需 Definition 目录
        </button>
      </header>

      <div class="shadow-notice" role="note">
        <strong>SHADOW · DIRECT_TARGET</strong>
        <span>订阅控制面已持久化；当前 Runtime 保持 direct_target，列表状态不会自动启用共享事件触发。</span>
      </div>

      <div v-if="viewState === 'loading'" class="state-card" role="status">
        <span class="spinner" aria-hidden="true" />
        <h2>正在读取订阅</h2>
        <p>只显示当前认证 principal 的分页结果。</p>
      </div>
      <div v-else-if="viewState === 'unavailable'" class="state-card state-danger" role="alert">
        <p class="state-code">UNAVAILABLE</p>
        <h2>订阅控制面暂时不可用</h2>
        <p>已清空缓存列表并关闭撤销动作，请重新确认权威状态。</p>
        <button data-agent-subscription-retry class="text-action danger" @click="load(true)">重新确认 →</button>
      </div>
      <div v-else-if="viewState === 'definition_stale'" class="state-card state-warning" role="alert">
        <p class="state-code">DEFINITION STALE</p>
        <h2>Definition 或订阅状态已变化</h2>
        <p>撤销请求未被接受。重新读取精确版本后再决定下一步。</p>
        <button data-agent-subscription-retry class="text-action warning" @click="load(true)">重新读取 →</button>
      </div>
      <div v-else-if="subscriptions.length === 0" class="state-card" role="status">
        <p class="state-code success">EMPTY</p>
        <h2>还没有事件订阅</h2>
        <p>创建前需要 active Definition version 与可读 conversation scope。</p>
      </div>

      <template v-else>
        <div class="list-heading">
          <h2>我的订阅 · {{ String(subscriptions.length).padStart(2, '0') }}</h2>
          <span>ACTIVE FIRST ↓</span>
        </div>
        <div class="content-grid">
          <div class="subscription-list">
            <article v-for="item in subscriptions" :key="item.subscriptionId" class="subscription-card" :class="{ revoked: item.status === 'revoked' }">
              <div class="card-top">
                <div>
                  <h3>{{ item.agentId === 'UAI' ? 'Project Guardian' : `Agent ${item.agentId}` }}</h3>
                  <p class="mono">{{ item.definitionId }} &nbsp;/&nbsp; VERSION {{ String(item.definitionVersion).padStart(2, '0') }}</p>
                </div>
                <span class="status-pill" :class="item.status"><i />{{ item.status === 'active' ? 'ACTIVE / SHADOW' : 'REVOKED' }}</span>
              </div>
              <p class="binding mono">{{ item.eventType }} &nbsp;·&nbsp; {{ item.resourceType }} {{ item.resourceId }}</p>
              <div class="card-bottom">
                <strong>{{ filterSummary(item) }}</strong>
                <button v-if="item.status === 'active'" :data-agent-subscription-revoke="item.subscriptionId" class="text-action" @click="openRevoke(item)">撤销 →</button>
                <span v-else class="audit-copy">{{ item.revokeReason }}</span>
              </div>
            </article>
            <button v-if="nextCursor" class="load-more" :disabled="busy" @click="loadMore">加载下一页 →</button>
          </div>

          <aside class="authority-panel">
            <p class="eyebrow">PINNED AUTHORITY</p>
            <h2>精确版本与范围</h2>
            <p>所有行来自当前 owner 的 Core 查询。创建入口将在 Definition 目录可用后开放。</p>
            <dl>
              <dt>RUNTIME</dt><dd>direct_target</dd>
              <dt>TRIGGER</dt><dd>default off</dd>
              <dt>FILTER</dt><dd>deterministic</dd>
            </dl>
            <div class="authority-warning"><strong>创建前检查</strong><br>只接受 owner 的 active Definition version；scope 或版本漂移立即拒绝。</div>
          </aside>
        </div>
      </template>
    </main>

    <div v-if="selected" class="dialog-backdrop" @click.self="closeRevoke" @keydown.esc="closeRevoke">
      <section role="dialog" aria-modal="true" aria-labelledby="revoke-title" class="revoke-dialog">
        <div class="sheet-handle" aria-hidden="true" />
        <p class="eyebrow danger">DESTRUCTIVE · AUDITED</p>
        <h2 id="revoke-title">撤销 {{ selected.agentId === 'UAI' ? 'Project Guardian' : selected.agentId }}？</h2>
        <p>撤销将绑定精确 subscription ID 和原因，并保留审计记录。该动作不会启动或停止模型 Runtime。</p>
        <label for="subscription-reason">REVOKE REASON · REQUIRED</label>
        <textarea id="subscription-reason" ref="reasonInput" v-model="reason" data-agent-subscription-reason maxlength="1000" :aria-invalid="reasonError ? 'true' : 'false'" :aria-describedby="reasonError ? 'subscription-reason-error' : undefined" />
        <p v-if="reasonError" id="subscription-reason-error" class="field-error" role="alert">{{ reasonError }}</p>
        <div class="dialog-actions">
          <button class="cancel-button" :disabled="viewState === 'revoking'" @click="closeRevoke">取消</button>
          <button data-agent-subscription-confirm class="confirm-button" :disabled="viewState === 'revoking'" @click="confirmRevoke">
            {{ viewState === 'revoking' ? '正在撤销…' : '确认撤销' }}
          </button>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { agentSubscriptionClient, type AgentSubscription, type AgentSubscriptionClient } from '@/api/agentSubscriptions'

const props = withDefaults(defineProps<{ client?: AgentSubscriptionClient }>(), { client: () => agentSubscriptionClient })
type ViewState = 'loading' | 'ready' | 'unavailable' | 'definition_stale' | 'revoking'
const viewState = ref<ViewState>('loading')
const subscriptions = ref<AgentSubscription[]>([])
const nextCursor = ref('')
const busy = ref(false)
const selected = ref<AgentSubscription>()
const reason = ref('')
const reasonError = ref('')
const reasonInput = ref<HTMLTextAreaElement>()

onMounted(() => load(true))

async function load(replace: boolean) {
  busy.value = true
  if (replace) viewState.value = 'loading'
  try {
    const page = await props.client.list(replace ? '' : nextCursor.value, 50)
    subscriptions.value = replace ? page.subscriptions : [...subscriptions.value, ...page.subscriptions]
    nextCursor.value = page.nextCursor
    viewState.value = 'ready'
  } catch {
    subscriptions.value = []
    nextCursor.value = ''
    viewState.value = 'unavailable'
  } finally {
    busy.value = false
  }
}

function loadMore() { if (!busy.value && nextCursor.value) void load(false) }

async function openRevoke(item: AgentSubscription) {
  selected.value = item
  reason.value = ''
  reasonError.value = ''
  await nextTick()
  reasonInput.value?.focus()
}

function closeRevoke() {
  if (viewState.value === 'revoking') return
  selected.value = undefined
  reason.value = ''
  reasonError.value = ''
}

async function confirmRevoke() {
  const item = selected.value
  const normalized = reason.value.trim()
  if (!item || !normalized) {
    reasonError.value = '请输入撤销原因'
    await nextTick()
    reasonInput.value?.focus()
    return
  }
  reasonError.value = ''
  viewState.value = 'revoking'
  try {
    const revoked = await props.client.revoke(item.subscriptionId, normalized)
    subscriptions.value = subscriptions.value.map(candidate => candidate.subscriptionId === revoked.subscriptionId ? revoked : candidate)
    selected.value = undefined
    viewState.value = 'ready'
  } catch (error) {
    selected.value = undefined
    subscriptions.value = []
    nextCursor.value = ''
    viewState.value = error instanceof Error && /concurrent|changed|conflict/i.test(error.message) ? 'definition_stale' : 'unavailable'
  }
}

function filterSummary(item: AgentSubscription): string {
  return item.filterKind === 'all' ? 'all messages' : `contains any · ${item.filter.terms?.join(' / ') ?? ''}`
}
</script>

<style scoped>
.subscription-shell{--paper:#f4f6f8;--panel:#fffefb;--ink:#17211d;--muted:#69756f;--line:#dce2dd;--rail:#152126;--rail-soft:#26363e;--green:#008e5b;--green-soft:#d9f3e6;--amber:#b87512;--amber-soft:#fff0d4;--red:#cf4d45;--red-soft:#fbe8e5;min-height:100vh;background:var(--paper);color:var(--ink);display:grid;grid-template-columns:256px 1fr;font-family:"Noto Sans SC",sans-serif}.control-rail{background:var(--rail);color:#eef4f0;padding:34px 26px;display:flex;flex-direction:column;gap:22px}.brand{font:800 20px Manrope,sans-serif;letter-spacing:.08em;display:flex;align-items:center;gap:12px}.brand-dot{width:12px;height:12px;border-radius:50%;background:#00b978}.rail-kicker,.eyebrow,.mono,.state-code,.list-heading span,.revoke-dialog label{font-family:"Geist Mono",ui-monospace,monospace;letter-spacing:.1em}.rail-kicker{margin:18px 0 0;color:#91a09b;font-size:10px;font-weight:800}.rail-active,.rail-item{display:flex;gap:12px;align-items:center;border-radius:10px;padding:12px 14px;font-weight:700}.rail-active{background:var(--rail-soft);color:#fff}.rail-item{color:#9aa9a4}.rail-boundary{margin-top:auto;color:#91a09b;font:10px/1.8 "Geist Mono",monospace}.subscription-main{padding:38px 42px 64px;min-width:0}.page-header{display:flex;justify-content:space-between;align-items:center;gap:28px}.eyebrow{color:var(--green);font-size:10px;font-weight:800;margin:0 0 10px}.page-header h1{font:800 clamp(34px,4vw,52px)/1 Manrope,sans-serif;letter-spacing:-.04em;margin:0}.subtitle{color:var(--muted);margin:12px 0 0}.create-button{border:0;border-radius:12px;background:#17211d;color:#fff;padding:15px 18px;font:700 13px inherit}.create-button:disabled{cursor:not-allowed;opacity:.62}.shadow-notice{margin:30px 0;background:var(--amber-soft);border-radius:12px;padding:14px 18px;display:flex;align-items:center;gap:18px;font-size:13px}.shadow-notice strong{color:var(--amber);font:800 10px "Geist Mono",monospace;letter-spacing:.1em;white-space:nowrap}.state-card{max-width:720px;background:var(--panel);border:1px solid var(--line);border-radius:18px;padding:32px;margin:72px auto}.state-card h2{font:700 26px Manrope,sans-serif;margin:10px 0}.state-card p{color:var(--muted)}.state-danger{background:var(--red-soft);border-left:5px solid var(--red)}.state-warning{background:var(--amber-soft);border-left:5px solid var(--amber)}.state-code{font-size:10px!important;font-weight:800;color:var(--red)!important}.state-code.success{color:var(--green)!important}.spinner{display:block;width:24px;height:24px;border:3px solid var(--line);border-top-color:var(--green);border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}.list-heading{display:flex;justify-content:space-between;align-items:center}.list-heading h2{font:600 22px Manrope,sans-serif}.list-heading span{color:#99a39f;font-size:9px;font-weight:800}.content-grid{display:grid;grid-template-columns:minmax(0,1fr) 330px;gap:24px}.subscription-list{display:grid;gap:16px}.subscription-card{background:var(--panel);border:1px solid var(--line);border-radius:16px;padding:24px}.subscription-card.revoked{opacity:.72}.card-top,.card-bottom{display:flex;justify-content:space-between;align-items:center;gap:18px}.card-top h3{font:700 20px Manrope,sans-serif;margin:0}.mono{font-size:10px;color:#8b9691}.status-pill{display:inline-flex;align-items:center;gap:8px;border-radius:999px;padding:10px 14px;background:var(--green-soft);color:var(--green);font:800 10px "Geist Mono",monospace;letter-spacing:.1em}.status-pill i{width:8px;height:8px;background:#00b978;border-radius:50%}.status-pill.revoked{background:var(--red-soft);color:var(--red)}.status-pill.revoked i{background:#e1847f}.binding{margin:20px 0;color:#77827d}.card-bottom strong{font-size:13px}.text-action{border:0;background:transparent;color:var(--green);font:800 12px inherit;cursor:pointer}.text-action.danger{color:var(--red)}.text-action.warning{color:var(--amber)}.audit-copy{color:var(--muted);font-size:11px}.authority-panel{background:var(--panel);border:1px solid var(--line);border-radius:18px;padding:26px;height:max-content}.authority-panel h2{font:700 23px Manrope,sans-serif}.authority-panel>p:not(.eyebrow){color:var(--muted);font-size:12px;line-height:1.7}.authority-panel dl{display:grid;grid-template-columns:90px 1fr;gap:10px;padding:18px 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line)}.authority-panel dt{color:#97a19d;font:800 9px "Geist Mono",monospace}.authority-panel dd{margin:0;font:600 11px "Geist Mono",monospace}.authority-warning{margin-top:20px;padding:14px;border-radius:10px;background:var(--red-soft);font-size:11px;line-height:1.6}.authority-warning strong{color:var(--red)}.load-more{border:0;background:transparent;color:var(--muted);padding:8px;text-align:left;cursor:pointer}.dialog-backdrop{position:fixed;inset:0;background:rgba(11,20,17,.45);display:flex;align-items:flex-end;justify-content:center;z-index:20}.revoke-dialog{width:min(560px,calc(100% - 32px));background:var(--panel);border-radius:22px 22px 0 0;padding:18px 26px 28px;box-shadow:0 -24px 70px rgba(10,20,16,.2)}.sheet-handle{width:42px;height:4px;border-radius:4px;background:var(--line);margin:0 auto 20px}.revoke-dialog h2{font:750 26px Manrope,sans-serif;margin:6px 0}.revoke-dialog>p:not(.eyebrow):not(.field-error){color:var(--muted);line-height:1.6}.revoke-dialog label{display:block;color:#909b96;font-size:9px;font-weight:800;margin:20px 0 7px}.revoke-dialog textarea{box-sizing:border-box;width:100%;min-height:86px;resize:vertical;border:1px solid var(--line);border-radius:12px;background:#f5f6f3;padding:12px;font:inherit;color:var(--ink)}.revoke-dialog textarea:focus{outline:3px solid rgba(0,142,91,.2);border-color:var(--green)}.field-error{color:var(--red);font-size:12px}.dialog-actions{display:flex;gap:10px;margin-top:20px}.dialog-actions button{flex:1;min-height:46px;border:0;border-radius:12px;font:800 13px inherit;cursor:pointer}.cancel-button{background:#f1f3f0;color:var(--ink)}.confirm-button{background:var(--red);color:white}.dialog-actions button:disabled{opacity:.6;cursor:wait}button:focus-visible,textarea:focus-visible{outline:3px solid rgba(0,142,91,.3);outline-offset:3px}
@media(max-width:900px){.subscription-shell{grid-template-columns:1fr}.control-rail{display:none}.subscription-main{padding:26px 20px 60px}.content-grid{grid-template-columns:1fr}.authority-panel{order:-1}.page-header{align-items:flex-start}.create-button{font-size:0;padding:12px}.create-button span{font-size:22px}.shadow-notice{align-items:flex-start;flex-direction:column;gap:6px}}
@media(max-width:560px){.subscription-main{padding:20px 16px 48px}.page-header h1{font-size:34px}.subtitle{font-size:12px}.shadow-notice{margin:22px 0}.content-grid{display:block}.authority-panel{display:none}.subscription-list{gap:14px}.subscription-card{padding:18px}.card-top{align-items:flex-start}.card-top h3{font-size:18px}.status-pill{padding:8px 10px}.binding{white-space:normal;line-height:1.5}.card-bottom{align-items:flex-end}.dialog-backdrop{background:rgba(11,20,17,.24)}.revoke-dialog{width:100%;border-radius:22px 22px 0 0;padding:14px 20px 24px}.dialog-actions{flex-direction:row}}
</style>
