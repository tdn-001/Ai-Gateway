<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="12">
        <el-card class="left-panel">
          <template #header>
            <div class="card-header">
              <span>Gateway 地址</span>
              <el-tooltip placement="top" content="请根据应用需要判断是否按 OpenAI 协议标准在后面拼上 /v1 或 /v1/chat/completions。仅支持 OpenAI 协议标准。" raw-content>
                <span class="help-icon">?</span>
              </el-tooltip>
            </div>
          </template>
          <div class="gateway-url">
            <el-tag type="success" size="large">{{ gatewayUrl }}</el-tag>
            <el-button size="small" @click="copyUrl">复制</el-button>
          </div>
          
          <el-divider />
          
          <div class="config-section">
            <h4>当前配置</h4>
            <el-descriptions :column="1" border size="small">
              <el-descriptions-item label="上游地址">{{ config.nginx_upstream_url }}</el-descriptions-item>
              <el-descriptions-item label="监听端口">{{ config.listen_port }}</el-descriptions-item>
              <el-descriptions-item label="应用请求超时">{{ config.client_timeout }}秒</el-descriptions-item>
              <el-descriptions-item label="上游请求超时">{{ config.upstream_timeout }}秒</el-descriptions-item>
              <el-descriptions-item label="SSE恢复">
                <el-tag :type="config.sse_recovery_enable ? 'success' : 'info'" size="small">
                  {{ config.sse_recovery_enable ? '启用' : '禁用' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="缓冲模式">
                <el-tag :type="config.buffer_mode ? 'primary' : 'default'" size="small">
                  {{ config.buffer_mode ? '缓冲后输出' : '实时转发' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="恢复模式">{{ config.default_recovery_mode }}</el-descriptions-item>
              <el-descriptions-item label="最大重试">{{ config.max_retry_times }}次</el-descriptions-item>
            </el-descriptions>
            <el-button type="primary" link @click="$router.push('/admin/management')">修改配置</el-button>
          </div>
          
          <el-divider />
          
          <div class="stats-section">
            <h4>系统统计</h4>
            <el-row :gutter="16">
              <el-col :span="8">
                <div class="stat-item">
                  <div class="stat-value">{{ stats.total_requests }}</div>
                  <div class="stat-label">总请求</div>
                </div>
              </el-col>
              <el-col :span="8">
                <div class="stat-item">
                  <div class="stat-value">{{ stats.total_keys }}</div>
                  <div class="stat-label">API Keys</div>
                </div>
              </el-col>
              <el-col :span="8">
                <div class="stat-item">
                  <div class="stat-value">{{ activeIPs.length }}</div>
                  <div class="stat-label">活跃IP</div>
                </div>
              </el-col>
            </el-row>
          </div>
          
          <el-divider />
          
          <div class="active-ips">
            <h4>活跃连接 IP</h4>
            <el-table :data="activeIPs" size="small" max-height="200">
              <el-table-column prop="ip" label="IP地址" />
              <el-table-column prop="location" label="位置" />
              <el-table-column prop="active" label="状态" width="80">
                <template #default="scope">
                  <el-tag :type="scope.row.active ? 'success' : 'info'" size="small">
                    {{ scope.row.active ? '活跃' : '离线' }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-card>
      </el-col>
      
      <el-col :span="12">
        <el-card class="right-panel">
          <template #header>
            <div class="card-header">
              <span>最近日志</span>
              <div class="header-right">
                <el-button type="primary" link @click="manualRefresh">手动刷新</el-button>
                <el-select v-model="autoRefreshInterval" size="small" style="width: 120px; margin-left: 8px;" @change="onRefreshIntervalChange">
                  <el-option label="10秒刷新" :value="10000" />
                  <el-option label="30秒刷新" :value="30000" />
                  <el-option label="1分钟刷新" :value="60000" />
                  <el-option label="不刷新" :value="0" />
                </el-select>
                <el-button type="primary" link style="margin-left: 8px;" @click="$router.push('/admin/logs')">查看全部</el-button>
              </div>
            </div>
          </template>
          
          <el-table :data="logs" style="width: 100%" size="small">
            <el-table-column prop="request_id" label="请求ID" width="140" show-overflow-tooltip />
            <el-table-column prop="client_ip" label="IP" width="120" />
            <el-table-column prop="status" label="状态" width="70">
              <template #default="scope">
                <el-tag :type="scope.row.status === 200 ? 'success' : 'danger'" size="small">
                  {{ scope.row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="recover" label="恢复" width="60">
              <template #default="scope">
                <el-tag v-if="scope.row.recover" type="warning" size="small">是</el-tag>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column prop="cost" label="耗时" width="70">
              <template #default="scope">
                {{ scope.row.cost?.toFixed(1) || '-' }}s
              </template>
            </el-table-column>
            <el-table-column prop="request_time" label="时间" width="140" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
    
    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>请求趋势</span>
          <el-radio-group v-model="trendInterval" size="small" @change="onTrendIntervalChange">
            <el-radio-button value="hour">每小时</el-radio-button>
            <el-radio-button value="day">每天</el-radio-button>
            <el-radio-button value="week">每周</el-radio-button>
            <el-radio-button value="month">每月</el-radio-button>
          </el-radio-group>
        </div>
      </template>
      <div ref="chartRef" style="height: 300px"></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import * as echarts from 'echarts'

const config = ref({
  listen_port: '3301',
  nginx_upstream_url: '',
  client_timeout: 300,
  upstream_timeout: 300,
  sse_recovery_enable: true,
  default_recovery_mode: 'B',
  max_retry_times: 5,
  buffer_mode: false
})

const gatewayUrl = window.location.origin

const stats = ref({ total_requests: 0, total_keys: 0 })
const logs = ref<any[]>([])
const activeIPs = ref<any[]>([])
const trendInterval = ref('hour')
const trendData = ref<any[]>([])
const chartRef = ref()
let chart: echarts.ECharts | null = null
let autoRefreshTimer: number | null = null
const autoRefreshInterval = ref(0)

const token = localStorage.getItem('token')
const headers = { Authorization: `Bearer ${token}` }

const fetchData = async () => {
  try {
    const [configRes, statsRes, logsRes, activeRes] = await Promise.all([
      axios.get('/admin/config', { headers }),
      axios.get('/admin/stats', { headers }),
      axios.get('/admin/logs', { headers }),
      axios.get('/admin/stats/active-ips', { headers })
    ])
    
    config.value = configRes.data
    stats.value = statsRes.data
    const allLogs = logsRes.data || []
    logs.value = allLogs.slice(-15).reverse()
    
    const ips = activeRes.data || []
    const ipsWithLocation = await Promise.all(ips.map(async (item: any) => {
      try {
        const locRes = await axios.get(`/admin/location/${item.ip}`, { headers })
        const data = locRes.data
        if (data.status === 'success') {
          return { ...item, location: `${data.country} ${data.regionName} ${data.city}` }
        }
        return { ...item, location: '未知' }
      } catch {
        return { ...item, location: '未知' }
      }
    }))
    activeIPs.value = ipsWithLocation
  } catch (error) {
    console.error('Failed to fetch data:', error)
  }
}

const fetchTrend = async () => {
  try {
    const response = await axios.get('/admin/stats/trend', {
      params: { interval: trendInterval.value, hours: 24 },
      headers
    })
    trendData.value = response.data || []
    renderChart()
  } catch (error) {
    console.error('Failed to fetch trend:', error)
  }
}

const renderChart = () => {
  if (!chartRef.value) return
  
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }
  
  const xData = trendData.value.map((item: any) => item.time)
  const yData = trendData.value.map((item: any) => item.count)
  
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: { type: 'category', data: xData },
    yAxis: { type: 'value' },
    series: [{
      data: yData,
      type: 'line',
      smooth: true,
      areaStyle: { opacity: 0.3 },
      lineStyle: { color: '#87CEEB' },
      itemStyle: { color: '#87CEEB' }
    }]
  })
}

const manualRefresh = async () => {
  await Promise.all([fetchData(), fetchTrend()])
  ElMessage.success('已刷新')
}

const startAutoRefresh = () => {
  stopAutoRefresh()
  if (autoRefreshInterval.value > 0) {
    autoRefreshTimer = window.setInterval(async () => {
      await Promise.all([fetchData(), fetchTrend()])
    }, autoRefreshInterval.value)
  }
}

const stopAutoRefresh = () => {
  if (autoRefreshTimer !== null) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

const onRefreshIntervalChange = () => {
  startAutoRefresh()
}

const onTrendIntervalChange = () => {
  fetchTrend()
}

const copyUrl = () => {
  navigator.clipboard.writeText(gatewayUrl)
  ElMessage.success('已复制到剪贴板')
}

onMounted(async () => {
  await fetchData()
  await fetchTrend()
  await nextTick()
  renderChart()
  
  window.addEventListener('resize', () => chart?.resize())
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.gateway-url {
  display: flex;
  align-items: center;
  gap: 12px;
}

.config-section h4,
.stats-section h4,
.active-ips h4 {
  margin: 12px 0 8px;
  font-size: 14px;
  color: #333;
}

.stat-item {
  text-align: center;
  padding: 8px;
  background: #f5f7fa;
  border-radius: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #87CEEB;
}

.stat-label {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
}
.help-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 1px solid #909399;
  color: #909399;
  font-size: 11px;
  font-weight: 600;
  cursor: help;
  margin-left: 4px;
  line-height: 1;
}
</style>