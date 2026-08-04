<template>
  <div class="admin-layout">
    <el-container>
      <el-aside width="200px" class="admin-sidebar">
        <div class="sidebar-header">
          <img src="/logo.png" alt="Logo" class="sidebar-logo" />
          <h2>AI Gateway</h2>
        </div>
        <el-menu
          :default-active="activeMenu"
          class="sidebar-menu"
          background-color="#87CEEB"
          text-color="#fff"
          active-text-color="#fff"
          router
        >
          <el-menu-item index="/admin">
            <el-icon><HomeFilled /></el-icon>
            <span>首页</span>
          </el-menu-item>
          <el-menu-item index="/admin/management">
            <el-icon><Setting /></el-icon>
            <span>管理</span>
          </el-menu-item>
          <el-menu-item index="/admin/apikeys">
            <el-icon><Key /></el-icon>
            <span>API Keys</span>
          </el-menu-item>
          <el-menu-item index="/admin/modelkeys">
            <el-icon><Connection /></el-icon>
            <span>模型节点</span>
          </el-menu-item>
          <el-menu-item index="/admin/logs">
            <el-icon><List /></el-icon>
            <span>日志管理</span>
          </el-menu-item>
        </el-menu>
      </el-aside>
      
      <el-container>
        <el-header class="admin-header">
          <div class="header-left">
            <el-breadcrumb separator="/">
              <el-breadcrumb-item :to="{ path: '/admin' }">首页</el-breadcrumb-item>
              <el-breadcrumb-item v-if="currentRoute">{{ currentRoute }}</el-breadcrumb-item>
            </el-breadcrumb>
          </div>
          <div class="header-right">
            <span class="user-greeting">欢迎您，{{ username }}</span>
            <el-button type="danger" size="small" @click="handleLogout">退出登录</el-button>
          </div>
        </el-header>
        
        <el-main class="admin-main">
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { HomeFilled, Setting, List, Key, Connection } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const username = ref('')

const parseUsernameFromToken = (): string => {
  const token = localStorage.getItem('token')
  if (!token) return ''
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.username || ''
  } catch {
    return ''
  }
}

const handleLogout = () => {
  localStorage.removeItem('token')
  ElMessage.success('已退出登录')
  router.push('/login')
}

const activeMenu = computed(() => {
  const path = route.path
  if (path.startsWith('/admin/management')) return '/admin/management'
  if (path.startsWith('/admin/modelkeys')) return '/admin/modelkeys'
  if (path.startsWith('/admin/apikeys')) return '/admin/apikeys'
  if (path.startsWith('/admin/logs')) return '/admin/logs'
  return '/admin'
})

const currentRoute = computed(() => {
  const routeMap: Record<string, string> = {
    '/admin/management': '管理',
    '/admin/modelkeys': '模型节点',
    '/admin/apikeys': 'API Keys',
    '/admin/logs': '日志管理'
  }
  return routeMap[route.path] || ''
})

onMounted(() => {
  const token = localStorage.getItem('token')
  if (!token) {
    router.push('/login')
  }
  username.value = parseUsernameFromToken()
})
</script>

<style scoped>
.admin-layout {
  height: 100vh;
}

:deep(.el-container) {
  height: 100%;
}

.admin-sidebar {
  background-color: #87CEEB;
  color: white;
  box-shadow: 2px 0 10px rgba(0, 0, 0, 0.1);
  height: 100vh;
}

.sidebar-header {
  padding: 20px;
  text-align: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.sidebar-logo {
  width: 50px;
  height: 50px;
  border-radius: 50%;
  margin-bottom: 8px;
}

.sidebar-header h2 {
  color: white;
  font-size: 18px;
  margin: 0;
}

.sidebar-menu {
  border-right: none;
}

.admin-header {
  background: white;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
  padding: 0 20px;
}

.header-left {
  flex: 1;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-greeting {
  color: #333;
  font-size: 14px;
}
.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.username {
  color: #333;
  font-size: 14px;
}

.admin-main {
  background: #f5f7fa;
  padding: 20px;
}
</style>