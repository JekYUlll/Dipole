import axios from 'axios'

const api = axios.create({
  baseURL: '/',
  timeout: 10000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('dipole.web.token')
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
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
      localStorage.removeItem('dipole.web.token')
      localStorage.removeItem('dipole.web.user')
      // Only redirect if not already on login page
      if (!window.location.pathname.endsWith('/login')) {
        window.location.replace('/app/login')
      }
    }
    const msg = error.response?.data?.message || error.message || '网络错误'
    return Promise.reject(new Error(msg))
  }
)

export default api
