import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'
import { primevueOptions } from './config/primevueTheme'
import './styles/design-tokens.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(PrimeVue, primevueOptions)
app.use(ConfirmationService)
app.use(ToastService)
useAuthStore(pinia)
app.mount('#app')
