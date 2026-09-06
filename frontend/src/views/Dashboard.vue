<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="12">
        <el-card class="left-panel">
          <template #header>
            <div class="card-header">
              <span>Gateway 地址</span>
            </div>
          </template>
          <div class="gateway-url">
            <el-tooltip placement="top" content="请根据应用需要判断是否按 OpenAI 协议标准在后面拼上 /v1 或 /v1/chat/completions。仅支持 OpenAI 协议标准。" raw-content>
              <span class="help-icon">?</span>
            </el-tooltip>
            <el-tag type="success" size="large">{{ gatewayUrl }}</el-tag>
            <el-button size="small" @click="copyUrl">复制</el-button>
          </div>
          
          <el-divider />
          
          <div class="config-section">
            <h4>当前配置</h4>
            <el-descriptions :column="1" border size="small">
              <el-descriptions-item label="模型地址">{{ activeNodeUrl }}</el-descriptions-item>
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
              <el-col :span="3">
                <div class="stat-item">
                  <div class="stat-value">{{ stats.total_requests }}</div>
                  <div class="stat-label">总请求</div>
                </div>
              </el-col>
              <el-col :span="3">
                <div class="stat-item">
                  <div class="stat-value highlight">{{ stats.today_requests }}</div>
                  <div class="stat-label">今日请求</div>
                </div>
              </el-col>
              <el-col :span="3">
                <div class="stat-item">
                  <div class="stat-value">{{ stats.total_retries }}</div>
                  <div class="stat-label">总重试</div>
                </div>
              </el-col>
              <el-col :span="3">
                <div class="stat-item">
                  <div class="stat-value highlight">{{ stats.today_retries }}</div>
                  <div class="stat-label">今日重试</div>
                </div>
              </el-col>

              <el-col :span="3">
                <div class="stat-item">
                  <div class="stat-value">{{ stats.total_tokens || 0 }}</div>
                  <div class="stat-label">总Token</div>
                </div>
              </el-col>
              <el-col :span="3">
                <div class="stat-item">
                  <div class="stat-value highlight">{{ stats.today_tokens || 0 }}</div>
                  <div class="stat-label">今日Token</div>
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
          
          <el-table :data="mergedLogs" size="small" style="width: 100%; flex: 1; min-height: 0;">
            <el-table-column prop="time" label="时间" width="140" />
            <el-table-column label="方向" width="70">
              <template #default="scope">
                <el-tag :type="scope.row.dir === 'up' ? 'primary' : 'success'" size="small">
                  {{ scope.row.dir === 'up' ? '上游' : '下游' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="ip" label="IP" width="115" />
            <el-table-column label="位置" show-overflow-tooltip>
              <template #default="scope">
                {{ mergedLocations[scope.row.ip] || '加载中...' }}
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="70">
              <template #default="scope">
                <el-tag :type="scope.row.status === 200 ? 'success' : 'danger'" size="small">
                  {{ scope.row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="重试" width="60">
              <template #default="scope">
                <span>{{ scope.row.retry === undefined ? '-' : scope.row.retry }}</span>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="70">
              <template #default="scope">
                {{ scope.row.cost?.toFixed(1) || '-' }}s
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card class="token-trend-card">
          <template #header>
            <div class="card-header">
              <span>Token 消耗趋势</span>
              <div class="header-right">
                <span v-if="tokenTrendInterval === 'hour'" class="zoom-hint">
                  <template v-if="tokenHoursRange < 1440">
                    最近 {{ tokenTrendLabel() }} ·
                    <el-button link type="primary" size="small" @click="resetTokenTrendZoom">重置24小时</el-button>
                  </template>
                  <template v-else>最近 24 小时</template>
                </span>
                <el-radio-group v-model="tokenTrendInterval" size="small" @change="onTokenTrendIntervalChange">
                  <el-radio-button value="hour">每小时</el-radio-button>
                  <el-radio-button value="day">每天</el-radio-button>
                  <el-radio-button value="week">每周</el-radio-button>
                  <el-radio-button value="month">每月</el-radio-button>
                </el-radio-group>
              </div>
            </div>
          </template>
          <div ref="tokenChartRef" class="token-trend-chart"></div>
        </el-card>
      </el-col>
    </el-row>
    
    <el-card style="margin-top: 20px">
      <template #header>
        <div class="card-header">
          <span>请求趋势</span>
          <div class="header-right">
            <span v-if="trendInterval === 'hour'" class="zoom-hint">
              <template v-if="hoursRange < 1440">
                最近 {{ trendLabel() }} ·
                <el-button link type="primary" size="small" @click="resetTrendZoom">重置24小时</el-button>
              </template>
              <template v-else>最近 24 小时</template>
            </span>
            <el-radio-group v-model="trendInterval" size="small" @change="onTrendIntervalChange">
              <el-radio-button value="hour">每小时</el-radio-button>
              <el-radio-button value="day">每天</el-radio-button>
              <el-radio-button value="week">每周</el-radio-button>
              <el-radio-button value="month">每月</el-radio-button>
            </el-radio-group>
          </div>
        </div>
      </template>
      <div ref="chartRef" class="trend-chart"></div>
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

const activeNodeUrl = ref('')

const gatewayUrl = window.location.origin

const stats = ref({ total_requests: 0, today_requests: 0, total_keys: 0, total_retries: 0, today_retries: 0, total_tokens: 0, today_tokens: 0 })
const logs = ref<any[]>([])
const upstreamLogs = ref<any[]>([])
const mergedLogs = ref<any[]>([])
const mergedLocations = ref<Record<string, string>>({})
const activeIPs = ref<any[]>([])
const trendInterval = ref('hour')
const trendData = ref<any[]>([])
const chartRef = ref()
let chart: echarts.ECharts | null = null
let autoRefreshTimer: number | null = null
let trendWheelTimer: number | null = null
const autoRefreshInterval = ref(0)

// Token 消耗趋势
const tokenTrendInterval = ref('hour')
const tokenTrendData = ref<any[]>([])
const tokenChartRef = ref()
let tokenChart: echarts.ECharts | null = null
let tokenTrendWheelTimer: number | null = null
const tokenHoursRange = ref(24)

// 请求趋势滚轮缩放：仅"每小时"视图生效。
// 小时级：24h→12h→6h→3h→2h→1h；分钟级：45min→30min→15min→10min→5min→3min→1min。
const trendZoomWindows = [
  { minutes: 1440, label: '24 小时' },
  { minutes: 720, label: '12 小时' },
  { minutes: 360, label: '6 小时' },
  { minutes: 180, label: '3 小时' },
  { minutes: 120, label: '2 小时' },
  { minutes: 60, label: '1 小时' },
  { minutes: 45, label: '45 分钟' },
  { minutes: 30, label: '30 分钟' },
  { minutes: 15, label: '15 分钟' },
  { minutes: 10, label: '10 分钟' },
  { minutes: 5, label: '5 分钟' },
  { minutes: 3, label: '3 分钟' },
  { minutes: 1, label: '1 分钟' },
]
const hoursRange = ref(24)

const token = localStorage.getItem('token')
const headers = { Authorization: `Bearer ${token}` }

const buildMergedLogs = () => {
  const list: any[] = []
  ;(logs.value || []).forEach((l: any) => {
    list.push({
      dir: 'down',
      time: l.request_time,
      ip: l.client_ip,
      status: l.status,
      retry: l.retry_count,
      cost: l.cost
    })
  })
  ;(upstreamLogs.value || []).forEach((l: any) => {
    list.push({
      dir: 'up',
      time: l.request_time,
      ip: l.client_ip,
      status: l.status,
      retry: undefined,
      cost: l.cost
    })
  })
  list.sort((a, b) => (a.time < b.time ? 1 : -1))
  mergedLogs.value = list.slice(0, 12)
}

const loadMergedLocations = async () => {
  const ips = new Set(mergedLogs.value.map((l: any) => l.ip))
  for (const ip of ips) {
    if (mergedLocations.value[ip]) continue
    if (ip === '127.0.0.1' || ip === 'localhost') {
      mergedLocations.value[ip] = '本地'
      continue
    }
    try {
      const locRes = await axios.get(`/admin/location/${ip}`, { headers })
      const data = locRes.data
      if (data.status === 'success') {
        mergedLocations.value[ip] = `${data.country} ${data.regionName} ${data.city}`
      } else {
        mergedLocations.value[ip] = '未知'
      }
    } catch {
      mergedLocations.value[ip] = '未知'
    }
  }
}

const fetchConfig = async () => {
  try {
    const configRes = await axios.get('/admin/config', { headers })
    config.value = configRes.data
    const nodesRes = await axios.get('/admin/modelnodes', { headers })
    const nodes = nodesRes.data || []
    const enabled = nodes.find((n: any) => n.enabled)
    activeNodeUrl.value = enabled ? enabled.url : '-'
  } catch (error) {
    console.error('Failed to fetch config:', error)
  }
}

const fetchData = async () => {
  try {
    const [statsRes, logsRes, upLogsRes, activeRes] = await Promise.all([
      axios.get('/admin/stats', { headers }),
      axios.get('/admin/logs', { headers }),
      axios.get('/admin/upstream-logs', { headers }),
      axios.get('/admin/stats/active-ips', { headers })
    ])
    
    stats.value = statsRes.data
    logs.value = logsRes.data || []
    upstreamLogs.value = upLogsRes.data || []
    buildMergedLogs()
    loadMergedLocations()
    
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

// 轻量刷新：日志面板和趋势图属于高频变化数据，定时/手动刷新只更新这几类接口，
// 配置、统计等低频数据在页面加载时一次性获取。
const refreshLogsAndTrend = async () => {
  try {
    const params = trendHours()
    const tokenParams = tokenTrendHours()
    const [statsRes, logsRes, upLogsRes, trendRes, tokenTrendRes] = await Promise.all([
      axios.get('/admin/stats', { headers }),
      axios.get('/admin/logs', { headers }),
      axios.get('/admin/upstream-logs', { headers }),
      axios.get('/admin/stats/trend', { params, headers }),
      axios.get('/admin/stats/token-trend', { params: tokenParams, headers })
    ])
    stats.value = statsRes.data
    logs.value = logsRes.data || []
    upstreamLogs.value = upLogsRes.data || []
    buildMergedLogs()
    loadMergedLocations()
    trendData.value = trendRes.data || []
    tokenTrendData.value = tokenTrendRes.data || []
    renderChart()
    renderTokenChart()
  } catch (error) {
    console.error('Failed to refresh logs and trend:', error)
  }
}

const trendHours = () => {
  if (trendInterval.value !== 'hour') return { interval: 'hour', hours: 24 }
  const win = trendZoomWindows.find(w => w.minutes === hoursRange.value)
  if (!win) return { interval: 'hour', hours: 24 }
  if (hoursRange.value < 60) return { interval: 'minute', minutes: hoursRange.value }
  return { interval: 'hour', hours: Math.floor(hoursRange.value / 60) }
}

const fetchTrend = async () => {
  try {
    const params = trendHours()
    const response = await axios.get('/admin/stats/trend', { params, headers })
    trendData.value = response.data || []
    renderChart()
  } catch (error) {
    console.error('Failed to fetch trend:', error)
  }
}

// 滚轮放大/缩小：滚轮向上放大（窗口缩小），向下还原。
// 悬停即可触发，无需点击图表获取焦点。
const onTrendWheel = (e: WheelEvent) => {
  if (trendInterval.value !== 'hour') return
  // 无条件阻止页面滚动，确保悬浮即可缩放
  e.preventDefault()
  const idx = trendZoomWindows.findIndex(w => w.minutes === hoursRange.value)
  if (idx === -1) return
  const zoomIn = e.deltaY < 0
  const nextIdx = zoomIn ? idx + 1 : idx - 1
  if (nextIdx < 0 || nextIdx >= trendZoomWindows.length) return
  if (trendWheelTimer) {
    clearTimeout(trendWheelTimer)
  }
  trendWheelTimer = window.setTimeout(() => {
    hoursRange.value = trendZoomWindows[nextIdx].minutes
    fetchTrend()
  }, 120)
}

const resetTrendZoom = () => {
  if (hoursRange.value === 1440) return
  hoursRange.value = 1440
  fetchTrend()
}

const trendLabel = () => {
  const win = trendZoomWindows.find(w => w.minutes === hoursRange.value)
  return win ? win.label : '24 小时'
}

const renderChart = () => {
  if (!chartRef.value) return
  
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }
  
  const xData = trendData.value.map((item: any) => item.time)
  const yData = trendData.value.map((item: any) => item.count)
  
  // 分钟级数据点多时自动旋转标签避免重叠
  const isMinuteView = hoursRange.value < 60
  const labelRotate = isMinuteView && xData.length > 30 ? 45 : 0
  
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'category',
      data: xData,
      axisLabel: {
        rotate: labelRotate,
        interval: isMinuteView && xData.length > 30 ? 'auto' : 0
      }
    },
    yAxis: { type: 'value' },
    series: [{
      data: yData,
      type: 'line',
      smooth: true,
      areaStyle: { opacity: 0.3 },
      lineStyle: { color: '#87CEEB' },
      itemStyle: { color: '#87CEEB' }
    }]
  }, true)
}

// ===== Token 消耗趋势 =====
const tokenTrendHours = () => {
  if (tokenTrendInterval.value !== 'hour') return { interval: 'hour', hours: 24, points: 24 }
  if (tokenHoursRange.value < 60) return { interval: 'minute', minutes: tokenHoursRange.value, points: 24 }
  return { interval: 'hour', hours: Math.floor(tokenHoursRange.value / 60), points: 24 }
}

const fetchTokenTrend = async () => {
  try {
    const params = tokenTrendHours()
    const response = await axios.get('/admin/stats/token-trend', { params, headers })
    tokenTrendData.value = response.data || []
    renderTokenChart()
  } catch (error) {
    console.error('Failed to fetch token trend:', error)
  }
}

const renderTokenChart = () => {
  if (!tokenChartRef.value) return
  if (!tokenChart) {
    tokenChart = echarts.init(tokenChartRef.value)
  }
  const xData = tokenTrendData.value.map((item: any) => item.time)
  const yData = tokenTrendData.value.map((item: any) => item.count)
  const isMinuteView = tokenHoursRange.value < 60
  const labelRotate = isMinuteView && xData.length > 30 ? 45 : 0
  tokenChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'category',
      data: xData,
      axisLabel: { rotate: labelRotate, interval: isMinuteView && xData.length > 30 ? 'auto' : 0 }
    },
    yAxis: { type: 'value' },
    series: [{
      data: yData,
      type: 'line',
      smooth: true,
      areaStyle: { opacity: 0.3 },
      lineStyle: { color: '#E6A23C' },
      itemStyle: { color: '#E6A23C' }
    }]
  }, true)
}

const onTokenTrendWheel = (e: WheelEvent) => {
  if (tokenTrendInterval.value !== 'hour') return
  e.preventDefault()
  const idx = trendZoomWindows.findIndex(w => w.minutes === tokenHoursRange.value)
  if (idx === -1) return
  const zoomIn = e.deltaY < 0
  const nextIdx = zoomIn ? idx + 1 : idx - 1
  if (nextIdx < 0 || nextIdx >= trendZoomWindows.length) return
  if (tokenTrendWheelTimer) clearTimeout(tokenTrendWheelTimer)
  tokenTrendWheelTimer = window.setTimeout(() => {
    tokenHoursRange.value = trendZoomWindows[nextIdx].minutes
    fetchTokenTrend()
  }, 120)
}

const resetTokenTrendZoom = () => {
  if (tokenHoursRange.value === 1440) return
  tokenHoursRange.value = 1440
  fetchTokenTrend()
}

const tokenTrendLabel = () => {
  const win = trendZoomWindows.find(w => w.minutes === tokenHoursRange.value)
  return win ? win.label : '24 小时'
}

const onTokenTrendIntervalChange = () => {
  if (tokenTrendInterval.value !== 'hour' && tokenHoursRange.value < 1440) {
    tokenHoursRange.value = 1440
  }
  fetchTokenTrend()
}
// ===== End Token 消耗趋势 =====

const manualRefresh = async () => {
  await refreshLogsAndTrend()
  ElMessage.success('已刷新')
}

const startAutoRefresh = () => {
  stopAutoRefresh()
  if (autoRefreshInterval.value > 0) {
    autoRefreshTimer = window.setInterval(async () => {
      await refreshLogsAndTrend()
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
  if (trendInterval.value !== 'hour' && hoursRange.value < 1440) {
    hoursRange.value = 1440
  }
  fetchTrend()
}

const copyUrl = () => {
  navigator.clipboard.writeText(gatewayUrl)
  ElMessage.success('已复制到剪贴板')
}

onMounted(async () => {
  await fetchConfig()
  await fetchData()
  await fetchTrend()
  await fetchTokenTrend()
  await nextTick()
  renderChart()
  renderTokenChart()
  
  window.addEventListener('resize', () => {
    chart?.resize()
    tokenChart?.resize()
  })
  chartRef.value?.addEventListener('wheel', onTrendWheel)
  tokenChartRef.value?.addEventListener('wheel', onTokenTrendWheel)
})

onUnmounted(() => {
  stopAutoRefresh()
  if (trendWheelTimer) {
    clearTimeout(trendWheelTimer)
    trendWheelTimer = null
  }
  if (tokenTrendWheelTimer) {
    clearTimeout(tokenTrendWheelTimer)
    tokenTrendWheelTimer = null
  }
  chartRef.value?.removeEventListener('wheel', onTrendWheel)
  tokenChartRef.value?.removeEventListener('wheel', onTokenTrendWheel)
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header .header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.zoom-hint {
  font-size: 12px;
  color: #909399;
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.trend-chart {
  height: 300px;
}

.token-trend-card {
  margin-top: 0;
  flex-shrink: 0;
}

.token-trend-chart {
  height: 200px;
  min-height: 200px;
}

.dashboard :deep(.el-col) {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.left-panel {
  width: 100%;
}

.right-panel {
  width: 100%;
  flex: 1;
  min-height: 0;
}

.right-panel :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  padding: 0;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.right-panel :deep(.el-table) {
  flex: 1;
  min-height: 0;
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

.stat-value.highlight {
  color: #E6A23C;
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