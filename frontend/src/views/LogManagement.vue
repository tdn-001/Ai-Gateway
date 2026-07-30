<template>
  <div class="log-management">
    <div class="page-header">
      <h1 class="page-title">日志管理</h1>
      <el-button type="danger" @click="handleClearLogs">
        清空日志
      </el-button>
    </div>
    
    <el-card>
      <el-table :data="logs" style="width: 100%" v-loading="loading">
        <el-table-column prop="request_id" label="请求ID" width="150" show-overflow-tooltip />
        <el-table-column prop="client_ip" label="客户端IP" width="120" />
        <el-table-column label="位置" width="150">
          <template #default="scope">
            <span v-if="ipLocations[scope.row.client_ip]">
              {{ ipLocations[scope.row.client_ip] }}
            </span>
            <span v-else class="loading-text">加载中...</span>
          </template>
        </el-table-column>
        <el-table-column prop="request_time" label="请求时间" width="160" />
        <el-table-column prop="status" label="HTTP状态" width="100">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">
              {{ scope.row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="recover" label="是否恢复" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.recover ? 'warning' : 'info'">
              {{ scope.row.recover ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="recover_count" label="恢复次数" width="90" />
        <el-table-column prop="retry_count" label="重试次数" width="90" />
        <el-table-column prop="error_phase" label="错误阶段" width="100">
          <template #default="scope">
            <el-tag v-if="scope.row.error_phase === 'connect'" type="danger" size="small">连接阶段</el-tag>
            <el-tag v-else-if="scope.row.error_phase === 'stream_init'" type="warning" size="small">流初始化</el-tag>
            <el-tag v-else-if="scope.row.error_phase === 'stream'" type="warning" size="small">流式输出中</el-tag>
            <el-tag v-else-if="scope.row.error_phase === 'recovery'" type="info" size="small">恢复请求</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="error" label="错误信息" show-overflow-tooltip />
        <el-table-column prop="partial_output" label="部分内容" show-overflow-tooltip>
          <template #default="scope">
            <span v-if="scope.row.partial_output">{{ scope.row.partial_output.substring(0, 50) }}...</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="cost" label="耗时" width="80">
          <template #default="scope">
            {{ scope.row.cost?.toFixed(1) || '-' }}s
          </template>
        </el-table-column>
      </el-table>
      
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[20, 50, 100, 200]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="fetchLogs"
        @current-change="fetchLogs"
        style="margin-top: 16px; justify-content: flex-end"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'

interface LogEntry {
  request_id: string
  client_ip: string
  request_time: string
  status: number
  recover: boolean
  recover_count: number
  retry_count: number
  error_phase: string
  cost: number
  error: string
  partial_output: string
  result: string
}

const logs = ref<LogEntry[]>([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const ipLocations = ref<Record<string, string>>({})

const fetchLogs = async () => {
  loading.value = true
  try {
    const token = localStorage.getItem('token')
    const response = await axios.get('/admin/logs', {
      headers: { Authorization: `Bearer ${token}` }
    })
    const allLogs = response.data || []
    total.value = allLogs.length
    
    const start = (currentPage.value - 1) * pageSize.value
    const end = start + pageSize.value
    logs.value = allLogs.slice().reverse().slice(start, end)
    
    loadIPLocations()
  } catch (error) {
    ElMessage.error('获取日志失败')
  } finally {
    loading.value = false
  }
}

const loadIPLocations = async () => {
  const token = localStorage.getItem('token')
  const headers = { Authorization: `Bearer ${token}` }
  
  for (const log of logs.value) {
    if (ipLocations.value[log.client_ip]) continue
    if (log.client_ip === '127.0.0.1' || log.client_ip === 'localhost') {
      ipLocations.value[log.client_ip] = '本地'
      continue
    }
    
    try {
      const response = await axios.get(`/admin/location/${log.client_ip}`, { headers })
      const data = response.data
      if (data.status === 'success') {
        ipLocations.value[log.client_ip] = `${data.country} ${data.region} ${data.city}`
      } else {
        ipLocations.value[log.client_ip] = '未获取IP信息'
      }
    } catch {
      ipLocations.value[log.client_ip] = '未获取IP信息'
    }
  }
}

const getStatusType = (status: number) => {
  if (status >= 200 && status < 300) return 'success'
  if (status >= 400 && status < 500) return 'warning'
  return 'danger'
}

const handleClearLogs = async () => {
  try {
    await ElMessageBox.confirm('确定要清空所有日志吗？此操作不可恢复。', '确认清空', {
      type: 'warning'
    })
    
    const token = localStorage.getItem('token')
    await axios.delete('/admin/logs', {
      headers: { Authorization: `Bearer ${token}` }
    })
    
    ElMessage.success('日志已清空')
    logs.value = []
    total.value = 0
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('清空日志失败')
    }
  }
}

onMounted(() => {
  fetchLogs()
})
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.loading-text {
  color: #999;
  font-size: 12px;
}
</style>