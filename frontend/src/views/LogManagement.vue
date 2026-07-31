<template>
  <div class="log-management">
    <div class="page-header">
      <h1 class="page-title">日志管理</h1>
      <el-button type="danger" @click="handleClearLogs">
        清空日志
      </el-button>
    </div>

    <el-row :gutter="16">
      <el-col :span="12">
        <el-card class="log-card">
          <template #header>
            <div class="card-header">
              <span>上游日志（客户端 → AI Gateway）</span>
            </div>
          </template>
          <el-table :data="upstreamLogs" style="width: 100%" v-loading="loading" height="calc(100vh - 280px)">
            <el-table-column prop="request_id" label="请求ID" min-width="150" show-overflow-tooltip />
            <el-table-column prop="client_ip" label="IP" width="115" />
            <el-table-column label="位置" min-width="130">
              <template #default="scope">
                <span v-if="ipLocations[scope.row.client_ip]">
                  {{ ipLocations[scope.row.client_ip] }}
                </span>
                <span v-else class="loading-text">加载中...</span>
              </template>
            </el-table-column>
            <el-table-column prop="request_time" label="时间" min-width="170" />
            <el-table-column prop="status" label="状态" width="75">
              <template #default="scope">
                <el-tag :type="getStatusType(scope.row.status)" size="small">
                  {{ scope.row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="model" label="模型" min-width="100" show-overflow-tooltip />
            <el-table-column prop="cost" label="耗时" width="70">
              <template #default="scope">
                {{ scope.row.cost?.toFixed(1) || '-' }}s
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="log-card">
          <template #header>
            <div class="card-header">
              <span>下游日志（AI Gateway → 上游）</span>
            </div>
          </template>
          <el-table :data="logs" style="width: 100%" v-loading="loading" height="calc(100vh - 280px)">
            <el-table-column prop="request_id" label="请求ID" min-width="150" show-overflow-tooltip />
            <el-table-column prop="request_time" label="时间" min-width="170" />
            <el-table-column prop="client_ip" label="IP" min-width="100" show-overflow-tooltip />
            <el-table-column prop="status" label="状态" width="75">
              <template #default="scope">
                <el-tag :type="getStatusType(scope.row.status)" size="small">
                  {{ scope.row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="recover" label="恢复" width="70">
              <template #default="scope">
                <el-tag :type="scope.row.recover ? 'warning' : 'info'" size="small">
                  {{ scope.row.recover ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="retry_count" label="重试" width="60" />
            <el-table-column prop="error_phase" label="错误阶段" width="90">
              <template #default="scope">
                <el-tag v-if="scope.row.error_phase === 'connect'" type="danger" size="small">连接</el-tag>
                <el-tag v-else-if="scope.row.error_phase === 'stream_init'" type="warning" size="small">流初始化</el-tag>
                <el-tag v-else-if="scope.row.error_phase === 'stream'" type="warning" size="small">流式中断</el-tag>
                <el-tag v-else-if="scope.row.error_phase === 'recovery'" type="info" size="small">恢复</el-tag>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column prop="error" label="错误信息" min-width="150" show-overflow-tooltip />
            <el-table-column prop="cost" label="耗时" width="70">
              <template #default="scope">
                {{ scope.row.cost?.toFixed(1) || '-' }}s
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
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

interface UpstreamLogEntry {
  request_id: string
  client_ip: string
  request_time: string
  cost: number
  status: number
  model: string
  stream: boolean
  error: string
}

const logs = ref<LogEntry[]>([])
const upstreamLogs = ref<UpstreamLogEntry[]>([])
const loading = ref(false)
const ipLocations = ref<Record<string, string>>({})

const fetchLogs = async () => {
  loading.value = true
  try {
    const token = localStorage.getItem('token')
    const headers = { Authorization: `Bearer ${token}` }

    const [downRes, upRes] = await Promise.all([
      axios.get('/admin/logs', { headers }),
      axios.get('/admin/upstream-logs', { headers })
    ])

    logs.value = (downRes.data || []).slice().reverse()
    upstreamLogs.value = (upRes.data || []).slice().reverse()

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

  const allIps = new Set<string>()
  logs.value.forEach(l => allIps.add(l.client_ip))
  upstreamLogs.value.forEach(l => allIps.add(l.client_ip))

  for (const ip of allIps) {
    if (ipLocations.value[ip]) continue
    if (ip === '127.0.0.1' || ip === 'localhost') {
      ipLocations.value[ip] = '本地'
      continue
    }

    try {
      const response = await axios.get(`/admin/location/${ip}`, { headers })
      const data = response.data
      if (data.status === 'success') {
        ipLocations.value[ip] = `${data.country} ${data.regionName} ${data.city}`
      } else {
        ipLocations.value[ip] = '未获取IP信息'
      }
    } catch {
      ipLocations.value[ip] = '未获取IP信息'
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
    upstreamLogs.value = []
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

.card-header {
  font-size: 14px;
  font-weight: 600;
}

.log-card :deep(.el-card__body) {
  padding: 10px;
}

.loading-text {
  color: #999;
  font-size: 12px;
}
</style>
