<script setup lang="ts">
// ChangePasswordDialog — 单独入口的修改密码弹窗。
//
// 从 Settings 弹窗打开；调用受保护的 /api/v1/auth/password，
// 成功后强制清理本地会话（terminateSession）并跳到登录。
// 与 ChatView 里"个人资料"弹窗共用同一份 API，但走独立 Dialog。

import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { useToast } from 'primevue/usetoast'
import api from '@/api'
import { useAuthStore } from '@/stores/auth'

const visible = defineModel<boolean>('visible', { default: false })
const emit = defineEmits<{ (e: 'changed'): void }>()
const auth = useAuthStore()
const toast = useToast()

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const busy = ref(false)
const error = ref('')

watch(visible, (open) => {
  if (!open) return
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  busy.value = false
  error.value = ''
})

async function submit() {
  error.value = ''
  if (currentPassword.value.length < 6 || newPassword.value.length < 6) {
    error.value = '密码长度需要 6-32 位'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的新密码不一致'
    return
  }
  if (currentPassword.value === newPassword.value) {
    error.value = '新密码需与当前密码不同'
    return
  }
  busy.value = true
  try {
    await api.patch('/api/v1/auth/password', {
      current_password: currentPassword.value,
      new_password: newPassword.value,
    })
    emit('changed')
    visible.value = false
    toast.add({ severity: 'success', summary: '密码已更新', detail: '当前会话已注销，请重新登录', life: 2500 })
    await auth.terminateSession(true)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '密码修改失败'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    :closable="!busy"
    :draggable="false"
    header="修改密码"
    :style="{ width: 'min(420px, 92vw)' }"
    class="dp-change-password"
  >
    <form class="cp-form" @submit.prevent="submit">
      <label>
        <span>当前密码</span>
        <InputText
          v-model="currentPassword"
          type="password"
          autocomplete="current-password"
          :disabled="busy"
          required
          minlength="6"
          maxlength="32"
        />
      </label>
      <label>
        <span>新密码</span>
        <InputText
          v-model="newPassword"
          type="password"
          autocomplete="new-password"
          :disabled="busy"
          required
          minlength="6"
          maxlength="32"
        />
      </label>
      <label>
        <span>再次输入新密码</span>
        <InputText
          v-model="confirmPassword"
          type="password"
          autocomplete="new-password"
          :disabled="busy"
          required
          minlength="6"
          maxlength="32"
        />
      </label>
      <p v-if="error" class="cp-error" role="alert">{{ error }}</p>
      <p class="cp-hint">修改成功后当前登录会自动注销。</p>
      <div class="cp-actions">
        <button type="button" class="cp-btn cp-btn--ghost" :disabled="busy" @click="visible = false">取消</button>
        <button type="submit" class="cp-btn cp-btn--primary" :disabled="busy">{{ busy ? '提交中...' : '修改密码' }}</button>
      </div>
    </form>
  </Dialog>
</template>

<style scoped>
.dp-change-password :deep(.p-dialog-header) {
  padding: 18px 24px 14px;
  border-bottom: 1px solid var(--dp-line);
}
.dp-change-password :deep(.p-dialog-content) {
  padding: 18px 24px 22px;
}

.cp-form { display: grid; gap: 14px; }
.cp-form label { display: grid; gap: 6px; font: 600 12px var(--dp-font-body); color: var(--dp-ink); }
.cp-form :deep(input) {
  width: 100%;
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  padding: 9px 10px;
  font: 500 13px var(--dp-font-body);
  background: var(--dp-surface);
  color: var(--dp-ink);
  transition: border-color 0.12s, box-shadow 0.12s;
}
.cp-form :deep(input:focus) {
  border-color: var(--dp-accent);
  outline: 2px solid var(--dp-accent);
  outline-offset: -1px;
}
.cp-form :deep(input:disabled) { background: var(--dp-surface-muted); }

.cp-hint { color: var(--dp-ink-soft); font: 500 12px var(--dp-font-body); margin: 0; }
.cp-error {
  color: var(--dp-danger);
  background: var(--dp-danger-soft, color-mix(in srgb, var(--dp-danger) 12%, transparent));
  padding: 8px 10px;
  border-radius: var(--dp-radius-sm);
  font: 600 12px var(--dp-font-body);
  margin: 0;
}
.cp-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 4px; }
.cp-btn {
  border: 1px solid transparent;
  border-radius: var(--dp-radius-sm);
  cursor: pointer;
  padding: 8px 16px;
  font: 700 12px var(--dp-font-body);
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}
.cp-btn--ghost {
  background: var(--dp-surface);
  color: var(--dp-ink);
  border-color: var(--dp-line);
}
.cp-btn--ghost:hover:not(:disabled) {
  background: var(--dp-surface-muted);
  border-color: var(--dp-ink-soft);
}
.cp-btn--primary {
  background: var(--dp-accent);
  color: var(--dp-text-inverse);
  border-color: var(--dp-accent);
}
.cp-btn--primary:hover:not(:disabled) {
  background: var(--dp-accent-strong);
  border-color: var(--dp-accent-strong);
}
.cp-btn:disabled { opacity: 0.55; cursor: not-allowed; }
</style>
