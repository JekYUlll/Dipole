<template>
  <div class="login-page">
    <aside class="brand-panel" aria-label="Dipole 平台介绍">
      <div class="orbit orbit-one" aria-hidden="true"></div>
      <div class="orbit orbit-two" aria-hidden="true"></div>
      <div class="brand-panel-copy">
        <img class="brand-mark brand-mark-panel" :src="dipoleAgentMark" alt="" aria-hidden="true" />
        <p class="brand-kicker">DIPOLE PLATFORM</p>
        <h1>让每一段协作<br />都有可靠的连接。</h1>
        <p class="brand-description">实时通信、可恢复任务与受控 Agent 能力，围绕同一条协作链路工作。</p>
      </div>
      <p class="brand-boundary">IM DATA PLANE / AGENT CONTROL PLANE</p>
    </aside>
    <main class="login-card" aria-labelledby="login-title">
      <header class="brand-lockup">
        <img class="brand-mark" :src="dipoleMark" alt="" aria-hidden="true" />
        <div>
          <p class="brand-kicker">WELCOME TO</p>
          <h2 id="login-title" class="brand">Dipole IM</h2>
        </div>
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
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import dipoleMark from '../../../docs/images/dipole-v3-im.svg'
import dipoleAgentMark from '../../../docs/images/dipole-v3-agent.svg'

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
  align-items: center;
  width: 100vw;
  min-height: 100vh;
  background: var(--dp-v3-ivory);
  color: var(--dp-v3-ink);
  font-family: var(--dp-font-body);
}
.brand-panel {
  position: relative;
  display: flex;
  width: min(48vw, 650px);
  align-self: stretch;
  overflow: hidden;
  padding: clamp(36px, 6vw, 92px);
  color: #fff;
  background: var(--dp-v3-navy);
}
.brand-panel::after {
  position: absolute;
  right: -64px;
  bottom: 10%;
  width: 150px;
  height: 150px;
  border: 18px solid var(--dp-v3-red);
  border-radius: 50%;
  content: '';
}
.brand-panel-copy { position: relative; z-index: 1; align-self: center; max-width: 420px; }
.brand-mark-panel { width: 94px; height: 94px; margin-bottom: 30px; }
.brand-panel h1 { margin: 12px 0 18px; font: 800 clamp(32px, 4vw, 56px)/1.08 var(--dp-font-display); letter-spacing: -.055em; }
.brand-description { max-width: 360px; color: #d4dfeb; font-size: 15px; line-height: 1.8; }
.brand-boundary { position: absolute; bottom: 34px; z-index: 1; margin: 0; color: #9eb4c9; font: 700 9px/1.4 var(--dp-font-data); letter-spacing: .12em; }
.orbit { position: absolute; width: 640px; height: 260px; border: 1px solid rgba(244, 176, 0, .7); border-radius: 50%; transform: rotate(-33deg); }
.orbit-one { top: -105px; right: -300px; }
.orbit-two { top: -69px; right: -324px; width: 720px; height: 320px; opacity: .45; }
.orbit-one::after { position: absolute; top: 48%; left: 15%; width: 12px; height: 12px; border-radius: 50%; background: var(--dp-v3-gold); content: ''; }
.login-card {
  width: min(100%, 475px);
  margin: 0 auto;
  padding: 48px 48px 44px;
  background: var(--dp-v3-paper);
  border: 1px solid rgba(9, 37, 69, .08);
  border-radius: 0;
  box-shadow: var(--dp-v3-shadow);
  color: var(--dp-v3-ink);
}
.brand-lockup {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  margin: 0 0 34px;
  text-align: left;
}
.brand-mark {
  width: 52px;
  height: 52px;
  flex: 0 0 52px;
}
.brand {
  font-size: 30px;
  line-height: 1;
  font-weight: 700;
  color: var(--dp-v3-navy);
  font-family: var(--dp-font-display);
  margin: 0;
}
.brand-kicker {
  margin: 0;
  color: var(--dp-v3-red);
  font: 800 9px/1.2 var(--dp-font-data);
  letter-spacing: .12em;
  text-transform: uppercase;
}
.tabs {
  display: flex;
  border-bottom: 1px solid var(--dp-v3-line);
  margin-bottom: 24px;
}
.tab {
  flex: 1;
  padding: 8px 0;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  color: var(--dp-v3-muted);
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.tab.active {
  color: var(--dp-v3-navy);
  border-bottom-color: var(--dp-v3-red);
  font-weight: 600;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form input {
  padding: 10px 12px;
  border: 1px solid var(--dp-v3-line);
  border-radius: 6px;
  font-size: 14px;
  color: var(--dp-v3-ink);
  background: #fff;
  font-family: inherit;
  outline: none;
  transition: border-color 0.2s;
}
.form input:focus { border-color: var(--dp-v3-focus); box-shadow: 0 0 0 3px rgba(242, 38, 42, .14); }
.form button {
  padding: 10px;
  background: var(--dp-v3-red);
  color: var(--dp-text-inverse);
  border: none;
  border-radius: 6px;
  font-size: 15px;
  cursor: pointer;
  margin-top: 4px;
}
.form button:hover:not(:disabled) { background: var(--dp-v3-red-strong); }
.form button:disabled { opacity: 0.6; cursor: not-allowed; }
.error {
  margin-top: 12px;
  color: var(--dp-v3-red-strong);
  font-size: 13px;
  text-align: center;
}
@media (max-width: 780px) {
  .login-page { display: block; }
  .brand-panel { width: 100%; min-height: 245px; padding: 32px 28px; }
  .brand-mark-panel { width: 54px; height: 54px; margin-bottom: 16px; }
  .brand-panel h1 { font-size: 32px; }
  .brand-description, .brand-boundary { display: none; }
  .login-card { width: 100%; min-height: calc(100vh - 245px); padding: 36px 28px 48px; box-shadow: none; }
}
</style>
