<template>
  <div class="login-page">
    <div class="login-card">
      <header class="brand-lockup">
        <img class="brand-logo" :src="dipoleLogo" alt="Dipole IM" />
      </header>
      <div class="tabs">
        <button :class="['tab', { active: mode === 'login' }]" @click="mode = 'login'">登录</button>
        <button :class="['tab', { active: mode === 'register' }]" @click="mode = 'register'">注册</button>
      </div>

      <form v-if="mode === 'login'" @submit.prevent="handleLogin" class="form">
        <input v-model="telephone" placeholder="手机号" maxlength="11" required />
        <input v-model="password" type="password" placeholder="密码" minlength="6" required />
        <button type="submit" :disabled="loading">{{ loading ? '登录中...' : '登录' }}</button>
      </form>

      <form v-else @submit.prevent="handleRegister" class="form">
        <input v-model="nickname" placeholder="昵称" minlength="2" maxlength="20" required />
        <input v-model="telephone" placeholder="手机号" maxlength="11" required />
        <input v-model="password" type="password" placeholder="密码" minlength="6" required />
        <input v-model="email" type="email" placeholder="邮箱（可选）" />
        <button type="submit" :disabled="loading">{{ loading ? '注册中...' : '注册' }}</button>
      </form>

      <p v-if="error" class="error">{{ error }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import dipoleLogo from '../../../docs/images/dipole-v3-im-traced.svg'

const router = useRouter()
const auth = useAuthStore()

const mode = ref<'login' | 'register'>('login')
const telephone = ref('')
const password = ref('')
const nickname = ref('')
const email = ref('')
const loading = ref(false)
const error = ref('')

const handleLogin = async () => {
  error.value = ''
  loading.value = true
  try {
    await auth.login(telephone.value, password.value)
    router.push({ name: 'chat' })
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}

const handleRegister = async () => {
  error.value = ''
  loading.value = true
  try {
    await auth.register(nickname.value, telephone.value, password.value, email.value || undefined)
    router.push({ name: 'chat' })
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '注册失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100vw;
  height: 100vh;
  background: var(--dp-canvas);
}
.login-card {
  background: var(--dp-surface);
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-md);
  padding: 40px 36px;
  width: 320px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.1);
  color: var(--dp-ink);
  font-family: var(--dp-font-body);
}
.brand-lockup {
  display: block;
  justify-content: center;
  margin: 0 0 24px;
  text-align: center;
}
.brand-logo {
  display: block;
  width: min(100%, 270px);
  height: auto;
  margin: 0 auto;
}
.tabs {
  display: flex;
  border-bottom: 1px solid var(--dp-line);
  margin-bottom: 20px;
}
.tab {
  flex: 1;
  padding: 8px 0;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  color: var(--dp-ink-soft);
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.tab.active {
  color: var(--dp-accent-strong);
  border-bottom-color: var(--dp-accent);
  font-weight: 600;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form input {
  padding: 10px 12px;
  border: 1px solid var(--dp-line);
  border-radius: var(--dp-radius-sm);
  font-size: 14px;
  color: var(--dp-ink);
  background: var(--dp-surface);
  font-family: inherit;
  outline: none;
  transition: border-color 0.2s;
}
.form input:focus { border-color: var(--dp-accent); }
.form button {
  padding: 10px;
  background: var(--dp-accent);
  color: var(--dp-text-inverse);
  border: none;
  border-radius: var(--dp-radius-sm);
  font-size: 15px;
  cursor: pointer;
  margin-top: 4px;
}
.form button:disabled { opacity: 0.6; cursor: not-allowed; }
.error {
  margin-top: 12px;
  color: var(--dp-danger);
  font-size: 13px;
  text-align: center;
}
</style>
