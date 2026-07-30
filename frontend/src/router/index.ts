import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import Register from '../views/Register.vue'
import AdminLayout from '../views/AdminLayout.vue'
import Dashboard from '../views/Dashboard.vue'
import Management from '../views/Management.vue'
import LogManagement from '../views/LogManagement.vue'
import APIKeyManagement from '../views/APIKeyManagement.vue'
import ModelKeyManagement from '../views/ModelKeyManagement.vue'
import NotFound from '../views/NotFound.vue'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login
  },
  {
    path: '/register',
    name: 'Register',
    component: Register
  },
  {
    path: '/admin',
    component: AdminLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: Dashboard
      },
      {
        path: 'management',
        name: 'Management',
        component: Management
      },
      {
        path: 'logs',
        name: 'Logs',
        component: LogManagement
      },
      {
        path: 'apikeys',
        name: 'APIKeys',
        component: APIKeyManagement
      },
      {
        path: 'modelkeys',
        name: 'ModelKeys',
        component: ModelKeyManagement
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: NotFound
  },
  {
    path: '/',
    redirect: '/login'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth)
  
  if (to.path === '/register') {
    next()
    return
  }
  
  if (requiresAuth && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/admin')
  } else {
    next()
  }
})

export default router