import axios from 'axios'
import { getDeviceID } from '@/device'

type UnauthorizedHandler = () => void | Promise<void>

let unauthorizedHandler: UnauthorizedHandler | undefined

export function setUnauthorizedHandler(handler: UnauthorizedHandler) {
  unauthorizedHandler = handler
}

function fallbackUnauthorizedCleanup() {
  for (const key of ['dipole.web.token', 'dipole.web.user', 'dipole.web.lastOfflineID']) {
    try { localStorage.removeItem(key) } catch { /* keep clearing the remaining keys */ }
  }
  if (!window.location.pathname.endsWith('/login')) {
    try { window.location.replace('/app/login') } catch { /* credentials remain locally revoked */ }
  }
}

const api = axios.create({
  baseURL: '/',
  timeout: 10000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('dipole.web.token')
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  if (config.headers) {
    config.headers['X-Device-ID'] = getDeviceID()
  }
  return config
})

api.interceptors.response.use(
  (response) => {
    const { code, data, message } = response.data
    if (code !== 0) {
      return Promise.reject(new Error(message || '请求失败'))
    }
    return data
  },
  (error) => {
    if (error.response?.status === 401) {
      if (unauthorizedHandler) {
        try { void Promise.resolve(unauthorizedHandler()).catch(() => {}) } catch { fallbackUnauthorizedCleanup() }
      } else {
        fallbackUnauthorizedCleanup()
      }
    }
    const msg = error.response?.data?.message || error.message || '网络错误'
    return Promise.reject(new Error(msg))
  }
)

export default api
