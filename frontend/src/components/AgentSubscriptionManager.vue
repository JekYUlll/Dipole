<template>
  <section class="subscription-shell" :data-agent-subscription-state="viewState" :aria-busy="busy">
    <aside class="control-rail" aria-label="Agent control navigation">
      <div class="brand"><span class="brand-dot" />DIPOLE</div>
      <p class="rail-kicker">AGENT CONTROL</p>
      <RouterLink v-if="nav.subscriptions" class="rail-active" :to="{ name: 'agent-subscriptions' }">⌁ <span>事件订阅</span></RouterLink>
      <div v-else class="rail-active">⌁ <span>事件订阅</span></div>
      <RouterLink v-if="nav.definitions" class="rail-item" :to="{ name: 'agent-definitions' }">▣ <span>Agent 定义</span></RouterLink>
      <div v-else class="rail-item">▣ <span>Agent 定义</span></div>
      <RouterLink v-if="nav.taskRun" class="rail-item" :to="nav.taskRun">☷ <span>任务运行</span></RouterLink>
      <div v-else class="rail-item">☷ <span>任务运行</span></div>
      <RouterLink v-if="nav.artifacts" class="rail-item" :to="{ name: 'agent-artifact-inbox' }">▦ <span>任务产物</span></RouterLink>
      <div v-else class="rail-item">▦ <span>任务产物</span></div>
      <p class="rail-boundary">OWNER CONTROL<br>RUNTIME: DIRECT_TARGET<br>高成本模型调用未启用</p>
    </aside>

    <main class="subscription-main">
      <header class="page-header">
        <div>
          <p class="eyebrow">PROJECT GUARDIAN / EVENT WATCH</p>
          <h1>事件订阅</h1>
          <p class="subtitle">查看当前账号创建的订阅，以及触发前采用的确定性过滤规则。</p>
        </div>
        <button data-agent-subscription-create class="create-button" :disabled="busy || viewState !== 'ready'" @click="openCreate">
          <span>＋</span> 创建事件订阅
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
            <article v-for="item in subscriptions" :key="item.subscriptionId" class="subscription-card" :data-agent-subscription-id="item.subscriptionId" :class="{ revoked: item.status === 'revoked' }">
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

    <div v-if="createOpen" class="dialog-backdrop create-backdrop" @click.self="closeCreate" @keydown.esc="closeCreate">
      <section role="dialog" aria-modal="true" aria-labelledby="create-title" class="create-dialog">
        <div class="create-dialog-heading">
          <div>
            <p class="eyebrow">AGENT CONTROL / NEW SUBSCRIPTION</p>
            <h2 id="create-title">创建事件订阅</h2>
          </div>
          <button class="dialog-close" :disabled="createState === 'submitting'" aria-label="关闭创建订阅" @click="closeCreate">×</button>
        </div>
        <div class="create-boundary" role="note"><strong>CONTROL ONLY · DIRECT_TARGET</strong><span>创建只写入控制记录，不会启用共享事件 Runtime。</span></div>

        <div v-if="createState === 'catalog_loading'" class="create-state" role="status">正在读取 owner 的 active Definition…</div>
        <template v-else-if="definitions.length">
          <label class="create-field" for="subscription-definition">
            <span>01 · ACTIVE DEFINITION</span>
            <select id="subscription-definition" v-model.number="selectedDefinitionIndex" data-agent-subscription-definition :disabled="createState === 'submitting'" @change="loadConversationOptions">
              <option v-for="(definition, index) in definitions" :key="`${definition.definitionId}:${definition.version}`" :value="index">
                {{ definition.agentId === 'UAI' ? 'Project Guardian' : definition.agentId }} · {{ definition.definitionId }} v{{ definition.version }}
              </option>
            </select>
          </label>

          <div v-if="createState === 'conversation_loading'" class="create-state" role="status">正在计算 principal readable ∩ Definition scope…</div>
          <label v-else-if="conversationOptions.length" class="create-field" for="subscription-conversation">
            <span>02 · READABLE ∩ SCOPE</span>
            <select id="subscription-conversation" v-model="selectedConversation" data-agent-subscription-conversation :disabled="createState === 'submitting'">
              <option v-for="option in conversationOptions" :key="option.conversationKey" :value="option.conversationKey">{{ option.conversationKey }}</option>
            </select>
            <small>候选项由 Core 计算；提交时再次复核会话可读性。</small>
          </label>

          <fieldset class="create-filter" :disabled="createState === 'submitting'">
            <legend>03 · DETERMINISTIC FILTER</legend>
            <label><input v-model="filterKind" type="radio" value="all"> 全部消息</label>
            <label><input v-model="filterKind" type="radio" value="message_contains_any"> 包含任一关键词</label>
            <template v-if="filterKind === 'message_contains_any'">
              <input v-model="termsText" data-agent-subscription-terms class="terms-input" maxlength="1024" placeholder="线上事故, 延期风险, 阻塞" :aria-invalid="termsError ? 'true' : 'false'" :aria-describedby="termsError ? 'subscription-terms-error' : undefined">
              <small v-if="termsError" id="subscription-terms-error" class="field-error" role="alert">{{ termsError }}</small>
            </template>
          </fieldset>
        </template>

        <div v-if="createState === 'no_eligible_scope'" class="create-state warning" role="alert">当前 Definition 与可读会话没有交集，请更换 Definition。</div>
        <div v-else-if="createState === 'definition_stale'" class="create-state danger" role="alert">Definition 已失效，请重新读取 active 版本。</div>
        <div v-else-if="createState === 'scope_denied'" class="create-state danger" role="alert">会话权限或 scope 已变化，请刷新候选范围。</div>
        <div v-else-if="createState === 'unavailable'" class="create-state danger" role="alert">创建控制面暂时不可用，旧候选已清空。</div>
        <p v-if="createError" class="field-error" role="alert">{{ createError }}</p>

        <div class="create-review">
          <strong>CANONICAL BINDING</strong>
          <span>{{ selectedDefinition ? `${selectedDefinition.definitionId} v${selectedDefinition.version}` : '—' }} → {{ selectedConversation || '—' }}</span>
          <small>创建成功 ≠ Runtime 已启用</small>
        </div>
        <div class="dialog-actions">
          <button class="cancel-button" :disabled="createState === 'submitting'" @click="closeCreate">取消</button>
          <button data-agent-subscription-submit class="create-submit" :disabled="!canSubmitCreate" @click="submitCreate">
            {{ createState === 'submitting' ? '正在创建…' : '创建控制记录' }}
          </button>
        </div>
      </section>
    </div>

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
import { computed, nextTick, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { agentDefinitionCatalogClient, type AgentDefinitionCatalogClient, type AgentDefinitionCatalogItem } from '@/api/agentDefinitions'
import { agentFlags, agentTaskRunTarget } from '@/config/agentFlags'
import { agentSubscriptionClient, type AgentSubscription, type AgentSubscriptionClient, type AgentSubscriptionConversationOption, type AgentSubscriptionFilterKind } from '@/api/agentSubscriptions'

const props = withDefaults(defineProps<{ client?: AgentSubscriptionClient, definitionClient?: AgentDefinitionCatalogClient }>(), {
  client: () => agentSubscriptionClient,
  definitionClient: () => agentDefinitionCatalogClient,
})
const nav = {
  definitions: agentFlags.definitions,
  subscriptions: agentFlags.subscriptions,
  artifacts: agentFlags.artifacts,
  taskRun: agentTaskRunTarget(),
}
type ViewState = 'loading' | 'ready' | 'unavailable' | 'definition_stale' | 'revoking'
const viewState = ref<ViewState>('loading')
const subscriptions = ref<AgentSubscription[]>([])
const nextCursor = ref('')
const busy = ref(false)
const selected = ref<AgentSubscription>()
const reason = ref('')
const reasonError = ref('')
const reasonInput = ref<HTMLTextAreaElement>()
type CreateState = 'idle' | 'catalog_loading' | 'conversation_loading' | 'ready' | 'no_eligible_scope' | 'definition_stale' | 'scope_denied' | 'submitting' | 'unavailable'
const createOpen = ref(false)
const createState = ref<CreateState>('idle')
const definitions = ref<AgentDefinitionCatalogItem[]>([])
const selectedDefinitionIndex = ref(0)
const conversationOptions = ref<AgentSubscriptionConversationOption[]>([])
const selectedConversation = ref('')
const filterKind = ref<AgentSubscriptionFilterKind>('all')
const termsText = ref('')
const createError = ref('')
const selectedDefinition = computed(() => definitions.value[selectedDefinitionIndex.value])
const termsError = computed(() => validateTerms(parseTerms()))
const canSubmitCreate = computed(() => createState.value === 'ready' && Boolean(selectedDefinition.value && selectedConversation.value) &&
  (filterKind.value === 'all' || termsError.value === ''))

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

async function openCreate() {
  createOpen.value = true
  createState.value = 'catalog_loading'
  definitions.value = []
  conversationOptions.value = []
  selectedConversation.value = ''
  createError.value = ''
  try {
    const page = await props.definitionClient.list('', 100)
    if (page.nextCursor) {
      createError.value = 'Active Definition 超过当前选择器上限，请先收窄可用版本'
      createState.value = 'unavailable'
      return
    }
    definitions.value = page.definitions
    selectedDefinitionIndex.value = 0
    if (!definitions.value.length) {
      createState.value = 'no_eligible_scope'
      return
    }
    await loadConversationOptions()
  } catch {
    createState.value = 'unavailable'
  }
}

async function loadConversationOptions() {
  const definition = selectedDefinition.value
  conversationOptions.value = []
  selectedConversation.value = ''
  createError.value = ''
  if (!definition) {
    createState.value = 'no_eligible_scope'
    return
  }
  createState.value = 'conversation_loading'
  try {
    const options = await props.client.listEligibleConversations(definition.definitionId, definition.version)
    conversationOptions.value = options.conversations
    selectedConversation.value = options.conversations[0]?.conversationKey ?? ''
    createState.value = options.conversations.length ? 'ready' : 'no_eligible_scope'
  } catch (error) {
    createState.value = error instanceof Error && /denied|stale|definition/i.test(error.message) ? 'definition_stale' : 'unavailable'
  }
}

function closeCreate() {
  if (createState.value === 'submitting') return
  createOpen.value = false
  createState.value = 'idle'
  definitions.value = []
  conversationOptions.value = []
  selectedConversation.value = ''
  createError.value = ''
}

async function submitCreate() {
  const definition = selectedDefinition.value
  const terms = parseTerms()
  if (!definition || !selectedConversation.value || (filterKind.value === 'message_contains_any' && validateTerms(terms))) {
    createError.value = '请选择有效 Definition、会话与过滤条件'
    return
  }
  createError.value = ''
  createState.value = 'submitting'
  try {
    const created = await props.client.create({
      definitionId: definition.definitionId,
      definitionVersion: definition.version,
      conversationKey: selectedConversation.value,
      filterKind: filterKind.value,
      filter: filterKind.value === 'all' ? {} : { terms },
    })
    subscriptions.value = [created, ...subscriptions.value.filter(item => item.subscriptionId !== created.subscriptionId)]
    closeCreateAfterSuccess()
  } catch (error) {
    conversationOptions.value = []
    selectedConversation.value = ''
    if (error instanceof Error && /denied|scope|permission/i.test(error.message)) createState.value = 'scope_denied'
    else if (error instanceof Error && /stale|definition|conflict|changed/i.test(error.message)) createState.value = 'definition_stale'
    else createState.value = 'unavailable'
  }
}

function closeCreateAfterSuccess() {
  createOpen.value = false
  createState.value = 'idle'
  definitions.value = []
  conversationOptions.value = []
  selectedConversation.value = ''
  filterKind.value = 'all'
  termsText.value = ''
}

function parseTerms(): string[] {
  const unique = new Set(termsText.value.split(/[，,\n]/u).map(term => term.trim()).filter(Boolean))
  return [...unique]
}

function validateTerms(terms: string[]): string {
  if (terms.length === 0) return '请输入至少一个关键词'
  if (terms.length > 32) return '关键词最多 32 项，请精简后提交'
  if (terms.some(term => [...term].length > 64 || /[\u0000-\u001f\u007f]/u.test(term))) {
    return '每个关键词最多 64 个字符，且不能包含控制字符'
  }
  return ''
}

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
.subscription-shell{--paper:var(--dp-canvas);--panel:var(--dp-surface);--ink:var(--dp-ink);--muted:var(--dp-ink-soft);--line:var(--dp-line);--rail:var(--dp-rail);--rail-soft:var(--dp-rail-soft);--green:var(--dp-rail);--green-soft:var(--dp-agent-soft);--amber:var(--dp-warning);--amber-soft:var(--dp-warning-soft);--red:var(--dp-danger);--red-soft:var(--dp-danger-soft);min-height:100vh;background:var(--paper);color:var(--ink);display:grid;grid-template-columns:256px 1fr;font-family:var(--dp-font-body)}.control-rail{background:var(--rail);color:var(--dp-text-inverse);padding:34px 26px;display:flex;flex-direction:column;gap:22px}.brand{font:800 20px var(--dp-font-display);letter-spacing:.08em;display:flex;align-items:center;gap:12px}.brand-dot{width:12px;height:12px;border-radius:50%;background:var(--dp-agent)}.rail-kicker,.eyebrow,.mono,.state-code,.list-heading span,.revoke-dialog label{font-family:var(--dp-font-data);letter-spacing:.1em}.rail-kicker{margin:18px 0 0;color:var(--dp-ink-faint);font-size:10px;font-weight:800}.rail-active,.rail-item{display:flex;gap:12px;align-items:center;border-radius:10px;padding:12px 14px;font-weight:700}.rail-active{background:var(--rail-soft);color:var(--dp-text-inverse)}.rail-item{color:var(--dp-ink-faint)}.rail-boundary{margin-top:auto;color:var(--dp-ink-faint);font:10px/1.8 var(--dp-font-data)}.subscription-main{padding:38px 42px 64px;min-width:0}.page-header{display:flex;justify-content:space-between;align-items:center;gap:28px}.eyebrow{color:var(--green);font-size:10px;font-weight:800;margin:0 0 10px}.page-header h1{font:800 clamp(34px,4vw,52px)/1 var(--dp-font-display);letter-spacing:-.04em;margin:0}.subtitle{color:var(--muted);margin:12px 0 0}.create-button{border:0;border-radius:var(--dp-radius-sm);background:var(--dp-rail);color:var(--dp-text-inverse);padding:15px 18px;font:700 13px inherit}.create-button:disabled{cursor:not-allowed;opacity:.62}.shadow-notice{margin:30px 0;background:var(--amber-soft);border-radius:var(--dp-radius-sm);padding:14px 18px;display:flex;align-items:center;gap:18px;font-size:13px}.shadow-notice strong{color:var(--amber);font:800 10px var(--dp-font-data);letter-spacing:.1em;white-space:nowrap}.state-card{max-width:720px;background:var(--panel);border:1px solid var(--line);border-radius:var(--dp-radius-md);padding:32px;margin:72px auto}.state-card h2{font:700 26px var(--dp-font-display);margin:10px 0}.state-card p{color:var(--muted)}.state-danger{background:var(--red-soft);border-left:5px solid var(--red)}.state-warning{background:var(--amber-soft);border-left:5px solid var(--amber)}.state-code{font-size:10px!important;font-weight:800;color:var(--red)!important}.state-code.success{color:var(--green)!important}.spinner{display:block;width:24px;height:24px;border:3px solid var(--line);border-top-color:var(--green);border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}.list-heading{display:flex;justify-content:space-between;align-items:center}.list-heading h2{font:600 22px var(--dp-font-display)}.list-heading span{color:var(--dp-ink-faint);font-size:9px;font-weight:800}.content-grid{display:grid;grid-template-columns:minmax(0,1fr) 330px;gap:24px}.subscription-list{display:grid;gap:16px}.subscription-card{background:var(--panel);border:1px solid var(--line);border-radius:var(--dp-radius-md);padding:24px}.subscription-card.revoked{opacity:.72}.card-top,.card-bottom{display:flex;justify-content:space-between;align-items:center;gap:18px}.card-top h3{font:700 20px var(--dp-font-display);margin:0}.mono{font-size:10px;color:var(--dp-ink-faint)}.status-pill{display:inline-flex;align-items:center;gap:8px;border-radius:999px;padding:10px 14px;background:var(--green-soft);color:var(--green);font:800 10px var(--dp-font-data);letter-spacing:.1em}.status-pill i{width:8px;height:8px;background:var(--dp-agent);border-radius:50%}.status-pill.revoked{background:var(--red-soft);color:var(--red)}.status-pill.revoked i{background:var(--dp-danger)}.binding{margin:20px 0;color:var(--dp-ink-soft)}.card-bottom strong{font-size:13px}.text-action{border:0;background:transparent;color:var(--green);font:800 12px inherit;cursor:pointer}.text-action.danger{color:var(--red)}.text-action.warning{color:var(--amber)}.audit-copy{color:var(--muted);font-size:11px}.authority-panel{background:var(--panel);border:1px solid var(--line);border-radius:var(--dp-radius-md);padding:26px;height:max-content}.authority-panel h2{font:700 23px var(--dp-font-display)}.authority-panel>p:not(.eyebrow){color:var(--muted);font-size:12px;line-height:1.7}.authority-panel dl{display:grid;grid-template-columns:90px 1fr;gap:10px;padding:18px 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line)}.authority-panel dt{color:var(--dp-ink-faint);font:800 9px var(--dp-font-data)}.authority-panel dd{margin:0;font:600 11px var(--dp-font-data)}.authority-warning{margin-top:20px;padding:14px;border-radius:var(--dp-radius-sm);background:var(--red-soft);font-size:11px;line-height:1.6}.authority-warning strong{color:var(--red)}.load-more{border:0;background:transparent;color:var(--muted);padding:8px;text-align:left;cursor:pointer}.dialog-backdrop{position:fixed;inset:0;background:rgba(11,20,17,.45);display:flex;align-items:flex-end;justify-content:center;z-index:20}.revoke-dialog{width:min(560px,calc(100% - 32px));background:var(--panel);border-radius:22px 22px 0 0;padding:18px 26px 28px;box-shadow:0 -24px 70px rgba(10,20,16,.2)}.sheet-handle{width:42px;height:4px;border-radius:var(--dp-radius-sm);background:var(--line);margin:0 auto 20px}.revoke-dialog h2{font:750 26px var(--dp-font-display);margin:6px 0}.revoke-dialog>p:not(.eyebrow):not(.field-error){color:var(--muted);line-height:1.6}.revoke-dialog label{display:block;color:var(--dp-ink-faint);font-size:9px;font-weight:800;margin:20px 0 7px}.revoke-dialog textarea{box-sizing:border-box;width:100%;min-height:86px;resize:vertical;border:1px solid var(--line);border-radius:var(--dp-radius-sm);background:var(--dp-surface-muted);padding:12px;font:inherit;color:var(--ink)}.revoke-dialog textarea:focus{outline:3px solid color-mix(in srgb, var(--dp-accent) 30%, transparent);border-color:var(--green)}.field-error{color:var(--red);font-size:12px}.dialog-actions{display:flex;gap:10px;margin-top:20px}.dialog-actions button{flex:1;min-height:46px;border:0;border-radius:var(--dp-radius-sm);font:800 13px inherit;cursor:pointer}.cancel-button{background:var(--dp-surface-muted);color:var(--ink)}.confirm-button{background:var(--red);color:var(--dp-text-inverse)}.dialog-actions button:disabled{opacity:.6;cursor:wait}button:focus-visible,textarea:focus-visible{outline:3px solid color-mix(in srgb, var(--dp-accent) 30%, transparent);outline-offset:3px}
.create-backdrop{align-items:center;padding:24px}.create-dialog{box-sizing:border-box;width:min(760px,100%);max-height:calc(100vh - 48px);overflow:auto;background:var(--panel);border-radius:22px;padding:26px;box-shadow:0 28px 90px rgba(10,20,16,.28)}.create-dialog-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:20px}.create-dialog-heading h2{font:750 28px Manrope,sans-serif;margin:0}.dialog-close{border:0;background:transparent;color:var(--muted);font-size:28px;cursor:pointer}.create-boundary{display:flex;flex-direction:column;gap:4px;margin:20px 0;padding:14px 16px;border-radius:12px;background:var(--amber-soft);color:#805312;font-size:12px}.create-boundary strong,.create-field>span,.create-filter legend,.create-review strong{font:800 10px "Geist Mono",monospace;letter-spacing:.08em}.create-field{display:grid;gap:8px;margin:16px 0}.create-field>span,.create-filter legend{color:var(--green)}.create-field select,.terms-input{box-sizing:border-box;width:100%;min-height:46px;border:1px solid var(--line);border-radius:11px;background:#f7f8f5;padding:10px 12px;color:var(--ink);font:600 13px inherit}.create-field small{color:var(--muted)}.create-filter{display:grid;grid-template-columns:1fr 1fr;gap:12px;border:1px solid var(--line);border-radius:13px;padding:15px;margin:18px 0}.create-filter legend{padding:0 5px}.terms-input{grid-column:1/-1}.create-state{margin:16px 0;padding:16px;border-radius:12px;background:var(--green-soft);color:var(--green);font-size:13px}.create-state.warning{background:var(--amber-soft);color:var(--amber)}.create-state.danger{background:var(--red-soft);color:var(--red)}.create-review{display:grid;gap:7px;background:var(--rail);color:#fff;border-radius:14px;padding:17px 18px}.create-review strong{color:#82deb3}.create-review span{font:650 16px "Geist Mono",monospace;overflow-wrap:anywhere}.create-review small{color:#afbeb8}.create-submit{background:var(--green);color:#fff}.create-submit:disabled{opacity:.5;cursor:not-allowed!important}
@media(max-width:900px){.subscription-shell{grid-template-columns:1fr}.control-rail{display:none}.subscription-main{padding:26px 20px 60px}.content-grid{grid-template-columns:1fr}.authority-panel{order:-1}.page-header{align-items:flex-start}.create-button{font-size:0;padding:12px}.create-button span{font-size:22px}.shadow-notice{align-items:flex-start;flex-direction:column;gap:6px}}
@media(max-width:560px){.subscription-main{padding:20px 16px 48px}.page-header h1{font-size:34px}.subtitle{font-size:12px}.shadow-notice{margin:22px 0}.content-grid{display:block}.authority-panel{display:none}.subscription-list{gap:14px}.subscription-card{padding:18px}.card-top{align-items:flex-start}.card-top h3{font-size:18px}.status-pill{padding:8px 10px}.binding{white-space:normal;line-height:1.5}.card-bottom{align-items:flex-end}.dialog-backdrop{background:rgba(11,20,17,.24)}.revoke-dialog{width:100%;border-radius:22px 22px 0 0;padding:14px 20px 24px}.dialog-actions{flex-direction:row}.create-backdrop{padding:0;align-items:flex-end}.create-dialog{width:100%;max-height:94vh;border-radius:22px 22px 0 0;padding:22px 20px}.create-filter{grid-template-columns:1fr}.terms-input{grid-column:1}.create-dialog-heading h2{font-size:24px}}
a.rail-item,a.rail-active{text-decoration:none;color:inherit}
</style>
