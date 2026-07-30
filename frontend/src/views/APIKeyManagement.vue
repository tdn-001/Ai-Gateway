<template>
  <div class="apikey-management">
    <div class="page-header">
      <h1 class="page-title">API Keys 管理</h1>
      <el-button type="primary" @click="showCreateDialog">
        新增 API Key
      </el-button>
    </div>
    
    <el-card>
      <el-table :data="apiKeys || []" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" label="名称" width="150" />
        <el-table-column prop="key" label="API Key" min-width="280">
          <template #default="scope">
            <div class="key-cell">
              <span class="key-text" v-if="visibleKeys[scope.row.key]">{{ scope.row.key }}</span>
              <span class="key-text" v-else>{{ scope.row.key.substring(0, 8) }}****</span>
              <el-button size="small" link @click="toggleKeyVisibility(scope.row.key)">
                <el-icon><View v-if="!visibleKeys[scope.row.key]"/><Hide v-else/></el-icon>
              </el-button>
              <el-button size="small" link @click="copyKey(scope.row.key)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="状态" width="100">
          <template #default="scope">
            <el-switch 
              v-model="scope.row.enabled" 
              @change="handleToggle(scope.row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="request_count" label="调用次数" width="100" />
        <el-table-column prop="last_used_at" label="最后使用" width="160" />
        <el-table-column prop="created_at" label="创建时间" width="160" />
        <el-table-column label="操作" width="150">
          <template #default="scope">
            <el-button size="small" @click="showUsageLog(scope.row)">
              调用记录
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(scope.row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    
    <el-dialog v-model="dialogVisible" title="新增 API Key" width="400px">
      <el-form label-width="100px">
        <el-form-item label="名称">
          <el-input v-model="keyName" placeholder="请输入Key名称" />
        </el-form-item>
        <el-form-item label="自定义Key">
          <el-input v-model="customKey" placeholder="留空则自动生成" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">确定</el-button>
      </template>
    </el-dialog>
    
    <el-dialog v-model="usageLogVisible" title="调用记录" width="800px">
      <el-table :data="usageLogs" style="width: 100%" max-height="400">
        <el-table-column prop="request_id" label="请求ID" width="180" />
        <el-table-column prop="client_ip" label="客户端IP" width="120" />
        <el-table-column prop="model" label="模型" width="120" />
        <el-table-column prop="status" label="状态" width="80">
          <template #default="scope">
            <el-tag :type="scope.row.status === 200 ? 'success' : 'danger'">
              {{ scope.row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cost" label="耗时(秒)" width="80" />
        <el-table-column prop="request_time" label="请求时间" width="160" />
      </el-table>
      <el-pagination
        v-model:current-page="logPage"
        :page-size="20"
        :total="logTotal"
        layout="prev, pager, next"
        @current-change="fetchUsageLogs"
      />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { View, Hide, CopyDocument } from '@element-plus/icons-vue'
import axios from 'axios'

interface APIKey {
  key: string
  name: string
  created_at: string
  last_used_at: string
  request_count: number
  enabled: boolean
}

const apiKeys = ref<APIKey[]>([])
const dialogVisible = ref(false)
const usageLogVisible = ref(false)
const visibleKeys = ref<Record<string, boolean>>({})
const keyName = ref('')
const customKey = ref('')
const creating = ref(false)
const loading = ref(false)
const usageLogs = ref([])
const logPage = ref(1)
const logTotal = ref(0)
const currentKey = ref('')

const fetchAPIKeys = async () => {
  loading.value = true
  try {
    const token = localStorage.getItem('token')
    const response = await axios.get('/admin/apikeys', {
      headers: { Authorization: `Bearer ${token}` }
    })
    if (Array.isArray(response.data)) {
      apiKeys.value = response.data
    } else {
      console.error('API Keys response is not an array:', response.data)
      apiKeys.value = []
    }
  } catch (error) {
    console.error('获取API Keys失败:', error)
    apiKeys.value = []
  } finally {
    loading.value = false
  }
}

const showCreateDialog = () => {
  keyName.value = ''
  customKey.value = ''
  dialogVisible.value = true
}

const handleCreate = async () => {
  if (!keyName.value.trim()) {
    ElMessage.warning('请输入Key名称')
    return
  }
  
  creating.value = true
  try {
    const token = localStorage.getItem('token')
    const response = await axios.post('/admin/apikeys', { 
      name: keyName.value,
      key: customKey.value 
    }, {
      headers: { Authorization: `Bearer ${token}` }
    })
    console.log('创建成功:', response.data)
    ElMessage.success('创建成功')
    dialogVisible.value = false
    fetchAPIKeys()
  } catch (error: any) {
    console.error('创建失败:', error)
    ElMessage.error(error.response?.data?.error || '创建失败')
  } finally {
    creating.value = false
  }
}

const handleDelete = async (row: APIKey) => {
  try {
    await ElMessageBox.confirm('确定要删除这个API Key吗？', '确认删除', { type: 'warning' })
    const token = localStorage.getItem('token')
    await axios.delete(`/admin/apikeys/${row.key}`, {
      headers: { Authorization: `Bearer ${token}` }
    })
    ElMessage.success('删除成功')
    fetchAPIKeys()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const handleToggle = async (row: APIKey) => {
  try {
    const token = localStorage.getItem('token')
    await axios.put(`/admin/apikeys/${row.key}/toggle`, { enabled: row.enabled }, {
      headers: { Authorization: `Bearer ${token}` }
    })
    ElMessage.success('更新成功')
  } catch (error) {
    ElMessage.error('更新失败')
    row.enabled = !row.enabled
  }
}

const toggleKeyVisibility = (key: string) => {
  visibleKeys.value[key] = !visibleKeys.value[key]
}

const copyKey = (key: string) => {
  navigator.clipboard.writeText(key)
  ElMessage.success('已复制到剪贴板')
}

const showUsageLog = async (row: APIKey) => {
  currentKey.value = row.key
  logPage.value = 1
  await fetchUsageLogs()
  usageLogVisible.value = true
}

const fetchUsageLogs = async () => {
  try {
    const token = localStorage.getItem('token')
    const response = await axios.get(`/admin/apikeys/${currentKey.value}/usage`, {
      params: { page: logPage.value, page_size: 20 },
      headers: { Authorization: `Bearer ${token}` }
    })
    usageLogs.value = response.data.logs || []
    logTotal.value = response.data.total || 0
  } catch (error) {
    ElMessage.error('获取调用记录失败')
  }
}

onMounted(() => {
  fetchAPIKeys()
})
</script>

<style scoped>
.key-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.key-text {
  font-family: monospace;
  font-size: 13px;
}

.el-pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>