<template>
  <div class="management">
    <h2>管理</h2>
    <div class="management-content">
      <div class="section">
        <h3>配置管理</h3>
        <div class="form-grid">
          <div class="form-group">
            <label>监听端口</label>
            <el-input v-model="config.listen_port" placeholder="3301" />
          </div>
          <div class="form-group">
            <label>上游地址</label>
            <el-input v-model="config.nginx_upstream_url" placeholder="http://127.0.0.1:8080" />
          </div>
          <div class="form-group">
            <label>应用请求超时(秒)</label>
            <el-input-number v-model="config.client_timeout" :min="30" :max="3600" />
            <div class="form-tip">应用请求Gateway的超时时间</div>
          </div>
          <div class="form-group">
            <label>上游请求超时(秒)</label>
            <el-input-number v-model="config.upstream_timeout" :min="30" :max="3600" />
            <div class="form-tip">Gateway请求Nginx上游的超时时间</div>
          </div>
          <div class="form-group">
            <label>最大重试次数</label>
            <el-input-number v-model="config.max_retry_times" :min="1" :max="20" />
          </div>
          <div class="form-group">
            <label>Session 过期(分钟)</label>
            <el-input-number v-model="config.session_expire_minute" :min="5" :max="1440" />
          </div>
          <div class="form-group">
            <label>日志保留(天)</label>
            <el-input-number v-model="config.log_keep_days" :min="1" :max="365" />
          </div>
          <div class="form-group">
            <label>SSE 断线恢复</label>
            <el-switch v-model="config.sse_recovery_enable" />
          </div>
          <div class="form-group">
            <label>
              流式缓冲模式
              <el-tooltip placement="top" :content="config.buffer_mode ? '开启：AI Gateway 先将上游返回内容完全缓存在内存中，等上游完整返回后再快速流式输出给应用。失败时客户端不会看到部分内容。输出完成后自动清理缓存。' : '关闭：上游一边返回内容，AI Gateway 一边实时转发给应用。上游中途出错时客户端可能看到部分内容后中断。'" raw-content>
                <span class="help-icon">?</span>
              </el-tooltip>
            </label>
            <el-switch v-model="config.buffer_mode" />
          </div>
          <div class="form-group">
            <label>默认恢复模式</label>
            <el-select v-model="config.default_recovery_mode">
              <el-option label="模式A-接续生成" value="A" />
              <el-option label="模式B-完整重生成" value="B" />
            </el-select>
          </div>
        </div>
        <el-button type="primary" @click="saveConfig" :loading="savingConfig">保存配置</el-button>
      </div>

      <div class="section">
        <h3>提示词管理</h3>
        <div class="toolbar">
          <el-select v-model="addPromptMode" placeholder="选择模式" style="width: 180px; margin-right: 12px;">
            <el-option label="模式A-接续生成" value="mode_a" />
            <el-option label="模式B-完整重生成" value="mode_b" />
          </el-select>
          <el-button type="primary" @click="openCreateDialog" :disabled="!addPromptMode">新增提示词</el-button>
        </div>
        <div class="prompt-tabs">
          <div class="tab" :class="{ active: activeTab === 'mode_a' }" @click="activeTab = 'mode_a'">模式A ({{ promptsByMode('mode_a').length }})</div>
          <div class="tab" :class="{ active: activeTab === 'mode_b' }" @click="activeTab = 'mode_b'">模式B ({{ promptsByMode('mode_b').length }})</div>
        </div>
        <el-table :data="promptsByMode(activeTab)" stripe style="width: 100%">
          <el-table-column prop="name" label="名称" width="200" />
          <el-table-column prop="enable" label="启用" width="80">
            <template #default="{ row }">
              <el-switch v-model="row.enable" @change="togglePrompt(row)" />
            </template>
          </el-table-column>
          <el-table-column prop="content" label="内容" show-overflow-tooltip />
          <el-table-column label="操作" width="160">
            <template #default="{ row }">
              <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deletePrompt(row.id)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑提示词' : '新增提示词'" width="600px">
      <el-form :model="currentPrompt" label-width="80px">
        <el-form-item label="模式">
          <el-select v-model="currentPrompt.mode" :disabled="isEditing">
            <el-option label="模式A-接续生成" value="mode_a" />
            <el-option label="模式B-完整重生成" value="mode_b" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="currentPrompt.name" placeholder="输入提示词名称" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="currentPrompt.enable" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="currentPrompt.content" type="textarea" :rows="10" placeholder="输入提示词内容" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePrompt" :loading="savingPrompt">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'

const config = ref<any>({
  listen_port: '3301',
  nginx_upstream_url: 'http://127.0.0.1:8080',
  client_timeout: 300,
  upstream_timeout: 300,
  sse_recovery_enable: true,
  default_recovery_mode: 'B',
  max_retry_times: 5,
  session_expire_minute: 30,
  log_keep_days: 5,
  buffer_mode: false,
})
const prompts = ref<any[]>([])
const savingConfig = ref(false)
const savingPrompt = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const activeTab = ref('mode_a')
const addPromptMode = ref('')
const currentPrompt = ref<any>({
  id: '',
  mode: 'mode_a',
  name: '',
  enable: true,
  content: '',
})

const promptsByMode = (mode: string) => {
  return prompts.value.filter((p: any) => p.mode === mode)
}

onMounted(async () => {
  try {
    const [configRes, promptsRes] = await Promise.all([
      axios.get('/admin/config'),
      axios.get('/admin/prompts'),
    ])
    config.value = configRes.data
    prompts.value = promptsRes.data
  } catch (err: any) {
    if (err.response?.status === 401) return
    ElMessage.error('加载数据失败')
  }
})

const saveConfig = async () => {
  savingConfig.value = true
  try {
    await axios.put('/admin/config', config.value)
    ElMessage.success('配置已保存')
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '保存失败')
  } finally {
    savingConfig.value = false
  }
}

const openCreateDialog = () => {
  isEditing.value = false
  currentPrompt.value = {
    id: '',
    mode: addPromptMode.value,
    name: '',
    enable: true,
    content: '',
  }
  dialogVisible.value = true
}

const openEditDialog = (prompt: any) => {
  isEditing.value = true
  currentPrompt.value = { ...prompt }
  dialogVisible.value = true
}

const savePrompt = async () => {
  if (!currentPrompt.value.name || !currentPrompt.value.content) {
    ElMessage.warning('请填写完整')
    return
  }
  savingPrompt.value = true
  try {
    if (isEditing.value) {
      await axios.put(`/admin/prompts/${currentPrompt.value.id}`, currentPrompt.value)
    } else {
      await axios.post('/admin/prompts', currentPrompt.value)
    }
    const res = await axios.get('/admin/prompts')
    prompts.value = res.data
    dialogVisible.value = false
    ElMessage.success(isEditing.value ? '更新成功' : '创建成功')
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '保存失败')
  } finally {
    savingPrompt.value = false
  }
}

const togglePrompt = async (prompt: any) => {
  try {
    await axios.put(`/admin/prompts/${prompt.id}`, prompt)
    ElMessage.success('更新成功')
  } catch (err: any) {
    prompt.enable = !prompt.enable
    ElMessage.error('更新失败')
  }
}

const deletePrompt = async (id: string) => {
  try {
    await ElMessageBox.confirm('确定删除该提示词？', '提示', { type: 'warning' })
    await axios.delete(`/admin/prompts/${id}`)
    prompts.value = prompts.value.filter((p: any) => p.id !== id)
    ElMessage.success('删除成功')
  } catch (err: any) {
    if (err !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}
</script>

<style scoped>
.management {
  padding: 20px;
}
.management h2 {
  margin: 0 0 20px 0;
  color: #303133;
}
.management-content {
  display: flex;
  flex-direction: column;
  gap: 30px;
}
.section {
  background: white;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.section h3 {
  margin: 0 0 16px 0;
  color: #303133;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}
.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: #606266;
}
.toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
.prompt-tabs {
  display: flex;
  gap: 0;
  margin-bottom: 16px;
  border-bottom: 2px solid #ebeef5;
}
.tab {
  padding: 10px 24px;
  cursor: pointer;
  font-size: 14px;
  color: #909399;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: all 0.2s;
}
.tab:hover {
  color: #409eff;
}
.tab.active {
  color: #409eff;
  border-bottom-color: #409eff;
  font-weight: 500;
}
.form-tip {
  font-size: 12px;
  color: #909399;
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