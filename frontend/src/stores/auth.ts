import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { PrivateUser } from '@/types'
import api from '@/api'

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
    _setSession(data.token, data.user)
  }

  const register = async (nickname: string, telephone: string, password: string, email?: string) => {
    const data = await api.post('/api/v1/auth/register', { nickname, telephone, password, email }) as { token: string; user: PrivateUser }
    _setSession(data.token, data.user)
  }

  const fetchMe = async () => {
    const data = await api.get('/api/v1/users/me') as PrivateUser
    currentUser.value = data
    localStorage.setItem('dipole.web.user', JSON.stringify(data))
  }

  const logout = async () => {
    try { await api.post('/api/v1/auth/logout') } catch { /* ignore */ }
    _clearSession()
  }

  const _setSession = (t: string, user: PrivateUser) => {
    token.value = t
    currentUser.value = user
    localStorage.setItem('dipole.web.token', t)
    localStorage.setItem('dipole.web.user', JSON.stringify(user))
  }

  const _clearSession = () => {
    token.value = ''
    currentUser.value = null
    localStorage.removeItem('dipole.web.token')
    localStorage.removeItem('dipole.web.user')
    localStorage.removeItem('dipole.web.lastOfflineID')
  }

  return { token, currentUser, login, register, fetchMe, logout }
})
