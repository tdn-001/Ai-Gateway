<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <img src="/logo.png" alt="Logo" class="logo" />
        <h1>AI Gateway</h1>
        <p>首次使用，请设置管理员账号</p>
      </div>
      
      <el-form ref="registerForm" :model="formData" :rules="rules" class="login-form">
        <el-form-item prop="username">
          <el-input 
            v-model="formData.username" 
            placeholder="请输入用户名（至少3个字符）"
            prefix-icon="User"
          />
        </el-form-item>
        
        <el-form-item prop="password">
          <el-input 
            v-model="formData.password" 
            type="password"
            placeholder="请输入密码（至少6个字符）"
            prefix-icon="Lock"
            show-password
          />
        </el-form-item>
        
        <el-form-item prop="confirmPassword">
          <el-input 
            v-model="formData.confirmPassword" 
            type="password"
            placeholder="请确认密码"
            prefix-icon="Lock"
            show-password
            @keyup.enter="handleRegister"
          />
        </el-form-item>
        
        <el-form-item>
          <el-button 
            type="primary" 
            class="login-button"
            :loading="loading"
            @click="handleRegister"
          >
            注册并进入系统
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import axios from 'axios'

const router = useRouter()
const registerForm = ref()
const loading = ref(false)

const formData = reactive({
  username: '',
  password: '',
  confirmPassword: ''
})

const validateConfirmPassword = (_rule: any, value: string, callback: any) => {
  if (value !== formData.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, message: '用户名至少3个字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6个字符', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const handleRegister = async () => {
  if (!registerForm.value) return
  
  await registerForm.value.validate(async (valid: boolean) => {
    if (valid) {
      loading.value = true
      try {
        const response = await axios.post('/admin/register', {
          username: formData.username,
          password: formData.password
        })
        const { token } = response.data
        localStorage.setItem('token', token)
        ElMessage.success('注册成功')
        router.push('/admin')
      } catch (error: any) {
        const message = error.response?.data?.error || '注册失败'
        ElMessage.error(message)
      } finally {
        loading.value = false
      }
    }
  })
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #87CEEB 0%, #B0E0E6 100%);
}

.login-card {
  width: 400px;
  padding: 40px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
}

.login-header {
  text-align: center;
  margin-bottom: 30px;
}

.logo {
  width: 80px;
  height: 80px;
  margin-bottom: 16px;
  border-radius: 50%;
}

.login-header h1 {
  font-size: 32px;
  color: #87CEEB;
  margin-bottom: 10px;
}

.login-header p {
  color: #666;
  font-size: 14px;
}

.login-form {
  width: 100%;
}

.login-button {
  width: 100%;
  height: 44px;
  font-size: 16px;
}
</style>