<script setup lang="ts">
// SettingsDialog — 从 AppShell 顶部齿轮 / 右上头像打开的模态设置窗。
//
// 布局：左侧是账户卡 + 分区导航（个人资料 / 账户安全 / Agent / 客户端 / 关于），
// 右侧是当前分区的内容。相比旧版本的堆叠列表，用侧栏 + 内容对齐替换密集分组，
// 消除"很粗糙"的观感；同时保持对既有 API 契约的兼容：
//   - `auth.fetchMe()`
//   - `/api/v1/users/{uuid}/profile` PATCH（只写 signature）
//   - `ChangePasswordDialog` 独立弹窗（不在本对话框中直接采集密码）
//   - 设备安全跳转 `name: 'devices'`
//   - Agent 分区仍由 `agentSettingsLinks()` 提供入口
// 组件仍是 PrimeVue Dialog（`data-settings-dialog`），不是整页。

import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Dialog from 'primevue/dialog'
import api from '@/api'
import { agentSettingsLinks } from '@/config/agentFlags'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import {
  IconAgent,
  IconCheck,
  IconChevronRight,
  IconEdit,
  IconInfo,
  IconLogout,
  IconRefreshCw,
  IconSettings,
} from '@/components/icons'
import ChangePasswordDialog from './ChangePasswordDialog.vue'

type SectionId = 'profile' | 'security' | 'agent' | 'client' | 'about'

interface SectionMeta {
  id: SectionId
  label: string
  eyebrow: string
  icon: unknown
}

const visible = defineModel<boolean>('visible', { default: false })
const router = useRouter()
const auth = useAuthStore()
const chat = useChatStore()

const activeSection = ref<SectionId>('profile')
const signature = ref('')
const loading = ref(false)
const saving = ref(false)
const saved = ref(false)
const errorMessage = ref('')
const passwordOpen = ref(false)

const sections = computed<SectionMeta[]>(() => {
  const list: SectionMeta[] = [
    { id: 'profile', label: '个人资料', eyebrow: 'PROFILE', icon: IconEdit },
    { id: 'security', label: '账户安全', eyebrow: 'SECURITY', icon: IconSettings },
  ]
  if (agentLinks.value.length) {
    list.push({ id: 'agent', label: 'Agent 控制', eyebrow: 'AGENT', icon: IconAgent })
  }
  list.push({ id: 'client', label: '客户端', eyebrow: 'CLIENT', icon: IconRefreshCw })
  list.push({ id: 'about', label: '关于', eyebrow: 'ABOUT', icon: IconInfo })
  return list
})

const displayName = computed(() =>
  auth.currentUser?.nickname?.trim() || auth.currentUser?.telephone || auth.currentUser?.email || '未登录',
)
const accountIdentity = computed(() =>
  auth.currentUser?.telephone || auth.currentUser?.email || auth.currentUser?.uuid || '—',
)
const avatarInitials = computed(() => {
  const seed = displayName.value
  if (!seed || seed === '未登录') return '·'
  return seed.trim().slice(0, 2).toUpperCase()
})

const syncLabel = computed(() => ({
  idle: '尚未同步',
  restoring: '正在恢复',
  current: '已同步',
  error: '同步异常',
  storage_full: '本地空间不足',
}[chat.syncStatus] || '未知状态'))
const syncTone = computed(() => {
  switch (chat.syncStatus) {
    case 'current': return 'ok'
    case 'restoring': return 'warn'
    case 'error':
    case 'storage_full': return 'danger'
    default: return 'muted'
  }
})

const agentLinks = computed(() => agentSettingsLinks())

const buildTag = computed(() => (import.meta.env.MODE ?? 'production').toUpperCase())

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    await auth.fetchMe()
    signature.value = auth.currentUser?.signature || ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '设置加载失败'
  } finally {
    loading.value = false
  }
}

async function saveProfile() {
  if (!auth.currentUser) return
  saving.value = true
  saved.value = false
  errorMessage.value = ''
  try {
    await api.patch(`/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile`, {
      signature: signature.value.trim(),
    })
    await auth.fetchMe()
    signature.value = auth.currentUser?.signature || ''
    saved.value = true
    setTimeout(() => { saved.value = false }, 2000)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '资料保存失败'
  } finally {
    saving.value = false
  }
}

async function logout() {
  await auth.logout()
  visible.value = false
  await router.push({ name: 'login' })
}

function gotoDevices() {
  visible.value = false
  router.push({ name: 'devices' })
}

function gotoAgentLink(link: ReturnType<typeof agentSettingsLinks>[number]) {
  visible.value = false
  router.push(link.to as never)
}

function selectSection(id: SectionId) {
  activeSection.value = id
}

onMounted(load)
watch(visible, (open) => {
  if (open) {
    activeSection.value = 'profile'
    load()
  }
})
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    dismissable-mask
    :draggable="false"
    header="设置"
    :style="{ width: 'min(760px, 96vw)' }"
    class="dp-settings-dialog"
    data-settings-dialog
  >
    <div class="settings-layout">
      <aside class="settings-side" aria-label="设置分区">
        <div class="account-card">
          <div class="account-card__avatar" aria-hidden="true">{{ avatarInitials }}</div>
          <div class="account-card__meta">
            <p class="account-card__name">{{ displayName }}</p>
            <p class="account-card__identity">{{ accountIdentity }}</p>
          </div>
        </div>
        <nav class="side-nav" role="tablist" aria-orientation="vertical">
          <button
            v-for="section in sections"
            :key="section.id"
            type="button"
            class="side-nav__item"
            :class="{ 'side-nav__item--active': activeSection === section.id }"
            role="tab"
            :aria-selected="activeSection === section.id"
            @click="selectSection(section.id)"
          >
            <component :is="section.icon" :size="16" class="side-nav__icon" />
            <span class="side-nav__label">{{ section.label }}</span>
            <IconChevronRight :size="14" class="side-nav__chev" />
          </button>
        </nav>
        <button class="side-logout" type="button" @click="logout">
          <IconLogout :size="15" />
          <span>退出登录</span>
        </button>
      </aside>

      <section class="settings-pane">
        <p v-if="errorMessage" class="pane-notice error" role="alert">
          <span>{{ errorMessage }}</span>
          <button type="button" class="link-btn" @click="load">重试</button>
        </p>

        <!-- 个人资料 -->
        <div v-show="activeSection === 'profile'" class="pane" role="tabpanel" aria-labelledby="profile-title">
          <header class="pane-head">
            <p class="pane-eyebrow">PROFILE</p>
            <h2 id="profile-title" class="pane-title">个人资料</h2>
            <p class="pane-copy">别人在联系人和群成员列表里看到的信息。</p>
          </header>
          <dl class="field-grid">
            <div>
              <dt>昵称</dt>
              <dd>{{ auth.currentUser?.nickname || '—' }}</dd>
            </div>
            <div>
              <dt>账号</dt>
              <dd>{{ accountIdentity }}</dd>
            </div>
          </dl>
          <label class="field-block">
            <span class="field-label">个性签名</span>
            <textarea
              v-model="signature"
              class="field-textarea"
              maxlength="255"
              rows="3"
              :disabled="loading || saving"
              placeholder="写下想让协作者了解的一句话"
            />
            <span class="field-hint">{{ signature.length }}/255</span>
          </label>
          <div class="pane-actions">
            <span v-if="saved" class="badge success" role="status">
              <IconCheck :size="12" /> 已保存
            </span>
            <button
              class="btn btn--primary"
              type="button"
              :disabled="loading || saving || !auth.currentUser"
              @click="saveProfile"
            >{{ saving ? '保存中…' : '保存资料' }}</button>
          </div>
        </div>

        <!-- 账户安全 -->
        <div v-show="activeSection === 'security'" class="pane" role="tabpanel" aria-labelledby="security-title">
          <header class="pane-head">
            <p class="pane-eyebrow">SECURITY</p>
            <h2 id="security-title" class="pane-title">账户安全</h2>
            <p class="pane-copy">密码与设备会话；不显示网络地址等敏感字段。</p>
          </header>
          <ul class="action-list">
            <li class="action-row">
              <div class="action-copy">
                <p class="action-title">修改登录密码</p>
                <p class="action-desc">修改成功后当前会话会自动注销，需要重新登录。</p>
              </div>
              <button
                class="btn btn--ghost"
                type="button"
                data-open-change-password
                @click="passwordOpen = true"
              >修改密码</button>
            </li>
            <li class="action-row">
              <div class="action-copy">
                <p class="action-title">设备与会话</p>
                <p class="action-desc">查看已登录设备的名称、类型与活动时间。</p>
              </div>
              <button class="btn btn--ghost" type="button" @click="gotoDevices">打开设备安全</button>
            </li>
          </ul>
        </div>

        <!-- Agent 控制 -->
        <div v-show="activeSection === 'agent'" class="pane" role="tabpanel" aria-labelledby="agent-title">
          <header class="pane-head">
            <p class="pane-eyebrow">AGENT CONTROL</p>
            <h2 id="agent-title" class="pane-title">Agent 控制</h2>
            <p class="pane-copy">打开 Chat 右侧 Agent 抽屉的对应工作区。</p>
          </header>
          <ul v-if="agentLinks.length" class="chip-grid">
            <li v-for="link in agentLinks" :key="link.id">
              <button type="button" class="chip" @click="gotoAgentLink(link)">
                <span>{{ link.label }}</span>
                <IconChevronRight :size="14" />
              </button>
            </li>
          </ul>
          <p v-else class="empty">当前构建未启用 Agent 特性。</p>
        </div>

        <!-- 客户端 -->
        <div v-show="activeSection === 'client'" class="pane" role="tabpanel" aria-labelledby="client-title">
          <header class="pane-head">
            <p class="pane-eyebrow">LOCAL SYNC</p>
            <h2 id="client-title" class="pane-title">客户端同步</h2>
            <p class="pane-copy">本地消息按登录用户隔离；退出、身份失效或 WebSocket 掉线会清理本机同步数据。</p>
          </header>
          <div class="stat-grid">
            <div class="stat">
              <span class="stat__label">同步状态</span>
              <span class="stat__value" :data-tone="syncTone">{{ syncLabel }}</span>
            </div>
            <div class="stat">
              <span class="stat__label">安全游标</span>
              <span class="stat__value">{{ chat.safeSyncSeq || '尚未建立' }}</span>
            </div>
          </div>
        </div>

        <!-- 关于 -->
        <div v-show="activeSection === 'about'" class="pane" role="tabpanel" aria-labelledby="about-title">
          <header class="pane-head">
            <p class="pane-eyebrow">ABOUT</p>
            <h2 id="about-title" class="pane-title">关于 Dipole</h2>
            <p class="pane-copy">实时协作 IM 与受治理的 Agent 平台。</p>
          </header>
          <dl class="field-grid">
            <div><dt>产品</dt><dd>Dipole</dd></div>
            <div><dt>构建</dt><dd>{{ buildTag }}</dd></div>
          </dl>
        </div>
      </section>
    </div>

    <ChangePasswordDialog v-model:visible="passwordOpen" />
  </Dialog>
</template>

<style scoped>
.dp-settings-dialog :deep(.p-dialog-header) {
  padding: 16px 22px 12px;
  border-bottom: 1px solid var(--dp-line);
}
.dp-settings-dialog :deep(.p-dialog-header .p-dialog-title) {
  font: 700 15px var(--dp-font-display);
  letter-spacing: 0.02em;
}
.dp-settings-dialog :deep(.p-dialog-content) {
  padding: 0;
  max-height: 78vh;
  overflow: hidden;
}

.settings-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  min-height: 460px;
}

/* ─── sidebar ─────────────────────────────────────── */
.settings-side {
  border-right: 1px solid var(--dp-line);
  background: var(--dp-surface-muted, color-mix(in srgb, var(--dp-canvas) 92%, transparent));
  display: flex;
  flex-direction: column;
  padding: 14px 12px 12px;
  gap: 10px;
}
.account-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 8px;
  border-radius: var(--dp-radius-sm);
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
}
.account-card__avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--dp-accent);
  color: var(--dp-text-inverse);
  display: grid;
  place-items: center;
  font: 700 13px var(--dp-font-data);
  letter-spacing: 0.04em;
}
.account-card__meta { min-width: 0; flex: 1; }
.account-card__name {
  font: 700 13px var(--dp-font-body);
  color: var(--dp-ink);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.account-card__identity {
  font: 500 11px var(--dp-font-data);
  color: var(--dp-ink-soft);
  margin: 2px 0 0;
  letter-spacing: 0.02em;
}

.side-nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 4px;
}
.side-nav__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--dp-radius-sm);
  color: var(--dp-ink-soft);
  cursor: pointer;
  font: 600 13px var(--dp-font-body);
  text-align: left;
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}
.side-nav__item:hover {
  color: var(--dp-ink);
  background: var(--dp-surface);
}
.side-nav__item--active {
  color: var(--dp-ink);
  background: var(--dp-surface);
  border-color: var(--dp-line);
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.02);
}
.side-nav__item--active .side-nav__chev { color: var(--dp-accent); }
.side-nav__label { flex: 1; min-width: 0; }
.side-nav__icon { color: currentColor; }
.side-nav__chev { color: var(--dp-ink-faint); }

.side-logout {
  margin-top: auto;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--dp-radius-sm);
  color: var(--dp-danger);
  font: 700 12px var(--dp-font-body);
  cursor: pointer;
}
.side-logout:hover {
  background: var(--dp-danger-soft, color-mix(in srgb, var(--dp-danger) 12%, transparent));
  border-color: var(--dp-danger);
}

/* ─── pane ────────────────────────────────────────── */
.settings-pane {
  padding: 20px 24px 22px;
  overflow-y: auto;
  max-height: calc(78vh - 60px);
}
.pane-head { margin-bottom: 14px; }
.pane-eyebrow {
  color: var(--dp-ink-faint);
  font: 700 10px var(--dp-font-data);
  letter-spacing: 0.14em;
  margin: 0 0 4px;
}
.pane-title {
  font: 700 15px var(--dp-font-display);
  color: var(--dp-ink);
  margin: 0 0 4px;
}
.pane-copy {
  color: var(--dp-ink-soft);
  font: 500 12px var(--dp-font-body);
  line-height: 1.6;
  margin: 0;
}

.pane-notice {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  margin: 0 0 12px;
  border-radius: var(--dp-radius-sm);
  font: 600 12px var(--dp-font-body);
}
.pane-notice.error {
  background: var(--dp-danger-soft, color-mix(in srgb, var(--dp-danger) 12%, transparent));
  color: var(--dp-danger);
}
.link-btn {
  background: transparent;
  border: 0;
  color: inherit;
  cursor: pointer;
  font: inherit;
  text-decoration: underline;
}

.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin: 0 0 14px;
  padding: 0;
}
.field-grid > div {
  background: var(--dp-surface-muted, color-mix(in srgb, var(--dp-canvas) 92%, transparent));
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  padding: 10px 12px;
}
.field-grid dt {
  color: var(--dp-ink-soft);
  font: 500 11px var(--dp-font-data);
  letter-spacing: 0.02em;
}
.field-grid dd {
  color: var(--dp-ink);
  font: 700 13px var(--dp-font-body);
  margin: 4px 0 0;
  word-break: break-all;
}

.field-block {
  display: grid;
  gap: 6px;
  margin-bottom: 4px;
}
.field-label {
  font: 700 12px var(--dp-font-body);
  color: var(--dp-ink);
}
.field-textarea {
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  font: 500 13px var(--dp-font-body);
  padding: 10px;
  resize: vertical;
  min-height: 76px;
  background: var(--dp-surface);
  color: var(--dp-ink);
}
.field-textarea:focus {
  outline: 2px solid var(--dp-accent);
  outline-offset: -1px;
  border-color: var(--dp-accent);
}
.field-hint {
  color: var(--dp-ink-faint);
  text-align: right;
  font: 500 10px var(--dp-font-data);
  letter-spacing: 0.04em;
}

.pane-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
}

.action-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 8px;
}
.action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
}
.action-copy { min-width: 0; flex: 1; }
.action-title {
  font: 700 13px var(--dp-font-body);
  color: var(--dp-ink);
  margin: 0 0 2px;
}
.action-desc {
  font: 500 12px var(--dp-font-body);
  color: var(--dp-ink-soft);
  line-height: 1.55;
  margin: 0;
}

.chip-grid {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px;
}
.chip {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 12px;
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  color: var(--dp-ink);
  cursor: pointer;
  font: 600 13px var(--dp-font-body);
}
.chip:hover {
  border-color: var(--dp-accent-strong);
  color: var(--dp-accent-strong);
}
.empty {
  color: var(--dp-ink-soft);
  font: 500 12px var(--dp-font-body);
  margin: 0;
}

.stat-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.stat {
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  padding: 12px;
  display: grid;
  gap: 4px;
}
.stat__label { color: var(--dp-ink-soft); font: 500 11px var(--dp-font-data); letter-spacing: 0.04em; }
.stat__value {
  font: 700 14px var(--dp-font-data);
  color: var(--dp-ink);
}
.stat__value[data-tone="ok"] { color: var(--dp-success); }
.stat__value[data-tone="warn"] { color: var(--dp-warning, #b3720c); }
.stat__value[data-tone="danger"] { color: var(--dp-danger); }
.stat__value[data-tone="muted"] { color: var(--dp-ink-soft); }

.badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 999px;
  font: 700 11px var(--dp-font-body);
  letter-spacing: 0.02em;
}
.badge.success {
  background: var(--dp-success-soft, color-mix(in srgb, var(--dp-success) 14%, transparent));
  color: var(--dp-success);
}

.btn {
  border: 1px solid transparent;
  border-radius: var(--dp-radius-sm);
  cursor: pointer;
  padding: 7px 14px;
  font: 700 12px var(--dp-font-body);
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}
.btn--primary {
  background: var(--dp-accent);
  color: var(--dp-text-inverse);
  border-color: var(--dp-accent);
}
.btn--primary:hover:not(:disabled) {
  background: var(--dp-accent-strong);
  border-color: var(--dp-accent-strong);
}
.btn--ghost {
  background: var(--dp-surface);
  color: var(--dp-ink);
  border-color: var(--dp-line);
}
.btn--ghost:hover:not(:disabled) {
  border-color: var(--dp-ink-soft);
  background: var(--dp-surface-muted);
}
.btn:disabled { opacity: 0.55; cursor: not-allowed; }

@media (max-width: 640px) {
  .settings-layout { grid-template-columns: 1fr; min-height: auto; }
  .settings-side {
    border-right: 0;
    border-bottom: 1px solid var(--dp-line);
    flex-direction: row;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
  }
  .account-card { flex: 1 1 100%; }
  .side-nav { flex-direction: row; flex-wrap: wrap; margin-top: 0; }
  .side-nav__chev { display: none; }
  .side-logout { margin: 0 0 0 auto; }
  .settings-pane { max-height: 60vh; padding: 16px 18px 18px; }
  .field-grid { grid-template-columns: 1fr; }
  .stat-grid { grid-template-columns: 1fr; }
}
</style>
