import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { PrivateUser } from '@/types'
import api, { setUnauthorizedHandler } from '@/api'
import { useChatStore } from '@/stores/chat'
import { BrowserSessionTerminator } from '@/session/sessionTermination'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('dipole.web.token') || '')
  const currentUser = ref<PrivateUser | null>(null)

  const _tryRestoreUser = () => {
    const raw = localStorage.getItem('dipole.web.user')
    if (raw) {
      try { currentUser.value = JSON.parse(raw) } catch { /* ignore */ }
    }
  }
  _tryRestoreUser()

  const login = async (telephone: string, password: string) => {
    const data = await api.post('/api/v1/auth/login', { telephone, password }) as { token: string; user: PrivateUser }
    await _setSession(data.token, data.user)
  }

  const register = async (nickname: string, telephone: string, password: string, email?: string) => {
    const data = await api.post('/api/v1/auth/register', { nickname, telephone, password, email }) as { token: string; user: PrivateUser }
    await _setSession(data.token, data.user)
  }

  const fetchMe = async () => {
    const data = await api.get('/api/v1/users/me') as PrivateUser
    currentUser.value = data
    localStorage.setItem('dipole.web.user', JSON.stringify(data))
  }

  const logout = async () => {
    try { await api.post('/api/v1/auth/logout') } catch { /* ignore */ }
    await terminateSession(false)
  }

  const _setSession = async (t: string, user: PrivateUser) => {
    await terminator.waitForCleanup()
    const previousUserUUID = currentUser.value?.uuid || _storedUserUUID()
    if (previousUserUUID && previousUserUUID !== user.uuid) {
      await terminator.terminate(previousUserUUID, false)
    }
    token.value = t
    currentUser.value = user
    localStorage.setItem('dipole.web.token', t)
    localStorage.setItem('dipole.web.user', JSON.stringify(user))
    useChatStore().myUUID = user.uuid
  }

  const _clearRuntime = () => {
    token.value = ''
    currentUser.value = null
    const chat = useChatStore()
    chat.myUUID = ''
    chat.resetRuntimeMessages()
  }

  const _storedUserUUID = () => {
    try {
      const user = JSON.parse(localStorage.getItem('dipole.web.user') || '') as { uuid?: unknown }
      return typeof user.uuid === 'string' ? user.uuid : ''
    } catch {
      return ''
    }
  }

  const terminator = new BrowserSessionTerminator(
    localStorage,
    _clearRuntime,
    userUUID => useChatStore().clearLocalMessages(userUUID),
    () => {
      if (!window.location.pathname.endsWith('/login')) window.location.replace('/app/login')
    },
  )

  const terminateSession = (redirectToLogin = true) => terminator.terminate(currentUser.value?.uuid || '', redirectToLogin)
  setUnauthorizedHandler(() => terminateSession(true))

  return { token, currentUser, login, register, fetchMe, logout, terminateSession }
})
