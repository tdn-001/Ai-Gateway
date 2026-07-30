import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import './style.css'
import App from './App.vue'
import router from './router'

axios.defaults.baseURL = window.location.origin

axios.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

axios.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      if (window.location.pathname !== '/login' && window.location.pathname !== '/register') {
        ElMessage.error('登录已过期，请重新登录')
        router.push('/login')
      }
    }
    return Promise.reject(error)
  }
)

const app = createApp(App)
app.use(ElementPlus)
app.use(router)
app.mount('#app')
