<script setup lang="ts">
// SettingsDialog — 从 AppShell 顶部齿轮打开的模态设置窗。
//
// 替代之前的整页 SettingsView，仍然覆盖同一批 section：
//   - 个人资料签名
//   - 修改密码（独立入口，打开 ChangePasswordDialog）
//   - Agent 控制入口（跳到抽屉视图）
//   - 设备安全（跳到 /devices）
//   - 本地同步状态
//   - 退出当前账户
//
// 与 SettingsView 用同一份 API 契约，测试沿用 shared source assertion。

import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Dialog from 'primevue/dialog'
import api from '@/api'
import { agentSettingsLinks } from '@/config/agentFlags'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import ChangePasswordDialog from './ChangePasswordDialog.vue'

const visible = defineModel<boolean>('visible', { default: false })
const router = useRouter()
const auth = useAuthStore()
const chat = useChatStore()

const signature = ref('')
const loading = ref(false)
const saving = ref(false)
const saved = ref(false)
const errorMessage = ref('')
const passwordOpen = ref(false)

const accountIdentity = computed(() =>
  auth.currentUser?.telephone || auth.currentUser?.email || auth.currentUser?.uuid || '未登录',
)
const syncLabel = computed(() => ({
  idle: '尚未同步', restoring: '正在恢复', current: '已同步',
  error: '同步异常', storage_full: '本地空间不足',
}[chat.syncStatus] || '未知状态'))

const agentLinks = computed(() => agentSettingsLinks())

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

function goto(to: unknown) {
  visible.value = false
  router.push(to as never)
}

onMounted(load)
watch(visible, (open) => { if (open) load() })
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    dismissable-mask
    :draggable="false"
    header="设置"
    :style="{ width: 'min(560px, 94vw)' }"
    class="dp-settings-dialog"
    data-settings-dialog
  >
    <p v-if="errorMessage" class="notice error" role="alert">
      {{ errorMessage }}
      <button type="button" @click="load">重试</button>
    </p>

    <section class="section" aria-labelledby="profile-title">
      <header>
        <p class="eyebrow">PROFILE</p>
        <h2 id="profile-title">个人资料</h2>
        <span class="identity">{{ accountIdentity }}</span>
      </header>
      <label class="signature-field">
        <span>个性签名</span>
        <textarea
          v-model="signature"
          maxlength="255"
          :disabled="loading || saving"
          placeholder="写下想让协作者了解的一句话"
        />
        <small>{{ signature.length }}/255</small>
      </label>
      <div class="actions">
        <span v-if="saved" class="notice success" role="status">资料已保存</span>
        <button
          class="btn primary"
          type="button"
          :disabled="loading || saving || !auth.currentUser"
          @click="saveProfile"
        >{{ saving ? '保存中...' : '保存资料' }}</button>
      </div>
    </section>

    <section class="section" aria-labelledby="security-title">
      <header>
        <p class="eyebrow">SECURITY</p>
        <h2 id="security-title">账户安全</h2>
      </header>
      <div class="row">
        <div class="row-copy">
          <p class="row-title">修改登录密码</p>
          <p class="row-desc">修改后当前会话会自动注销，需要重新登录。</p>
        </div>
        <button
          class="btn secondary"
          type="button"
          data-open-change-password
          @click="passwordOpen = true"
        >修改密码</button>
      </div>
      <div class="row">
        <div class="row-copy">
          <p class="row-title">设备会话</p>
          <p class="row-desc">仅显示设备名称、粗粒度类型和活动时间，不披露 IP 或连接标识。</p>
        </div>
        <button class="btn secondary" type="button" @click="goto({ name: 'devices' })">打开设备安全</button>
      </div>
    </section>

    <section v-if="agentLinks.length" class="section" aria-labelledby="agent-title">
      <header>
        <p class="eyebrow">AGENT CONTROL</p>
        <h2 id="agent-title">Agent 控制</h2>
      </header>
      <p class="section-copy">打开 Chat 右侧 Agent 抽屉的对应工作区。</p>
      <ul class="chip-links">
        <li v-for="link in agentLinks" :key="link.id">
          <button type="button" class="chip" @click="goto(link.to)">{{ link.label }}</button>
        </li>
      </ul>
    </section>

    <section class="section" aria-labelledby="sync-title">
      <header>
        <p class="eyebrow">LOCAL SYNC</p>
        <h2 id="sync-title">客户端同步</h2>
        <span class="identity">{{ syncLabel }}</span>
      </header>
      <p class="section-copy">本地消息按登录用户隔离；退出、身份失效或 WebSocket 掉线会清理本机同步数据。</p>
      <dl class="sync-details">
        <div><dt>安全游标</dt><dd>{{ chat.safeSyncSeq || '尚未建立' }}</dd></div>
        <div><dt>同步状态</dt><dd>{{ syncLabel }}</dd></div>
      </dl>
    </section>

    <section class="section danger-section" aria-labelledby="logout-title">
      <header>
        <p class="eyebrow">SESSION</p>
        <h2 id="logout-title">退出当前账户</h2>
      </header>
      <div class="row">
        <p class="row-desc row-copy">退出会撤销当前会话并清理本机保存的账户数据。</p>
        <button class="btn danger" type="button" @click="logout">退出登录</button>
      </div>
    </section>

    <ChangePasswordDialog v-model:visible="passwordOpen" />
  </Dialog>
</template>

<style scoped>
.dp-settings-dialog :deep(.p-dialog-header) {
  padding: 18px 24px 14px;
  border-bottom: 1px solid var(--dp-line);
}
.dp-settings-dialog :deep(.p-dialog-header .p-dialog-title) {
  font: 700 15px var(--dp-font-display);
  letter-spacing: 0.02em;
}
.dp-settings-dialog :deep(.p-dialog-content) {
  padding: 4px 24px 24px;
  max-height: 78vh;
}
.section {
  padding: 18px 0;
  border-top: 1px solid var(--dp-line);
}
.section:first-of-type { border-top: 0; padding-top: 14px; }
.section > header {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}
.section h2 {
  font: 700 14px var(--dp-font-display);
  margin: 0;
  color: var(--dp-ink);
}
.eyebrow {
  color: var(--dp-ink-faint);
  font: 700 10px var(--dp-font-data);
  letter-spacing: 0.14em;
  margin: 0;
}
.danger-section .eyebrow { color: var(--dp-danger); }
.identity {
  margin-left: auto;
  color: var(--dp-ink-soft);
  font: 600 11px var(--dp-font-data);
}
.section-copy, .row-desc {
  color: var(--dp-ink-soft);
  font: 500 12px var(--dp-font-body);
  margin: 0;
  line-height: 1.6;
}
.signature-field {
  display: grid;
  gap: 6px;
  font: 600 12px var(--dp-font-body);
  margin-top: 8px;
}
.signature-field textarea {
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  font: 500 13px var(--dp-font-body);
  min-height: 76px;
  padding: 10px;
  resize: vertical;
}
.signature-field small {
  color: var(--dp-ink-faint);
  text-align: right;
  font: 500 10px var(--dp-font-data);
  letter-spacing: 0.04em;
}
.signature-field textarea:focus {
  outline: 2px solid var(--dp-accent);
  outline-offset: -1px;
  border-color: var(--dp-accent);
}
.actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 0;
}
.row + .row { border-top: 1px dashed var(--dp-line); }
.row-copy { flex: 1; min-width: 0; }
.row-title {
  font: 700 13px var(--dp-font-body);
  color: var(--dp-ink);
  margin: 0 0 2px;
}
.chip-links {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
}
.chip {
  border: 1px solid var(--dp-line);
  background: var(--dp-surface);
  color: var(--dp-ink);
  padding: 6px 12px;
  border-radius: var(--dp-radius-pill, 999px);
  font: 600 12px var(--dp-font-body);
  cursor: pointer;
}
.chip:hover { border-color: var(--dp-accent-strong); color: var(--dp-accent-strong); }
.sync-details {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin: 12px 0 0;
}
.sync-details div {
  background: var(--dp-surface-muted, color-mix(in srgb, var(--dp-canvas) 92%, transparent));
  border-radius: var(--dp-radius-sm);
  padding: 12px;
}
.sync-details dt { color: var(--dp-ink-soft); font: 500 11px var(--dp-font-data); }
.sync-details dd { font: 700 13px var(--dp-font-data); margin: 4px 0 0; color: var(--dp-ink); }
.btn {
  border: 1px solid transparent;
  border-radius: var(--dp-radius-sm);
  cursor: pointer;
  padding: 7px 14px;
  font: 700 12px var(--dp-font-body);
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}
.btn.primary {
  background: var(--dp-accent);
  color: var(--dp-text-inverse);
  border-color: var(--dp-accent);
}
.btn.primary:hover:not(:disabled) { background: var(--dp-accent-strong); border-color: var(--dp-accent-strong); }
.btn.secondary { background: var(--dp-surface); color: var(--dp-ink); border-color: var(--dp-line); }
.btn.secondary:hover:not(:disabled) { border-color: var(--dp-ink-soft); color: var(--dp-ink); background: var(--dp-surface-muted); }
.btn.danger { background: var(--dp-surface); color: var(--dp-danger); border-color: var(--dp-danger); }
.btn.danger:hover:not(:disabled) { background: var(--dp-danger); color: var(--dp-text-inverse); }
.btn:disabled { opacity: 0.55; cursor: not-allowed; }
.notice {
  border-radius: var(--dp-radius-sm);
  padding: 8px 12px;
  margin: 12px 0 0;
  display: inline-flex;
  gap: 8px;
  font: 600 12px var(--dp-font-body);
}
.notice.error { background: var(--dp-danger-soft, color-mix(in srgb, var(--dp-danger) 12%, transparent)); color: var(--dp-danger); }
.notice.success { background: var(--dp-success-soft, color-mix(in srgb, var(--dp-success) 12%, transparent)); color: var(--dp-success); margin: 0; }
.notice button { background: transparent; border: 0; color: inherit; cursor: pointer; text-decoration: underline; font: inherit; }
.danger-section { border-top-color: var(--dp-danger-soft); }
.danger-section h2 { color: var(--dp-ink); }
</style>
