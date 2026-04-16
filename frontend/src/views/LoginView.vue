<template>
  <div class="login-page">
    <div class="login-card">
      <h1 class="brand">Dipole</h1>
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
  background: #ededed;
}
.login-card {
  background: #fff;
  border-radius: 8px;
  padding: 40px 36px;
  width: 320px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.1);
}
.brand {
  text-align: center;
  font-size: 28px;
  font-weight: 700;
  color: #07c160;
  margin: 0 0 24px;
}
.tabs {
  display: flex;
  border-bottom: 1px solid #e0e0e0;
  margin-bottom: 20px;
}
.tab {
  flex: 1;
  padding: 8px 0;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  color: #888;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.tab.active {
  color: #07c160;
  border-bottom-color: #07c160;
  font-weight: 600;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form input {
  padding: 10px 12px;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s;
}
.form input:focus { border-color: #07c160; }
.form button {
  padding: 10px;
  background: #07c160;
  color: #fff;
  border: none;
  border-radius: 4px;
  font-size: 15px;
  cursor: pointer;
  margin-top: 4px;
}
.form button:disabled { opacity: 0.6; cursor: not-allowed; }
.error {
  margin-top: 12px;
  color: #e74c3c;
  font-size: 13px;
  text-align: center;
}
</style>
