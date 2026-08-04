<template>
  <div class="modelkey-management">
    <div class="page-header">
      <h1 class="page-title">模型节点</h1>
      <el-button type="primary" @click="showCreateNodeDialog">
        新增节点
      </el-button>
    </div>

    <el-alert
      type="info"
      show-icon
      :closable="false"
      style="margin-bottom: 16px;"
    >
      管理模型服务节点。每个节点对应一个模型地址，可配置多个 Key。启用的 Key 通过
      <code>Authorization: Bearer xxx</code>
      传递给上游。启用的节点地址用于转发请求，同一节点内的多个 Key 将按顺序轮询使用。同一请求内重试 3 次后自动切换到下一个 Key，总重试次数受配置限制。
    </el-alert>

    <el-card v-for="node in nodes" :key="node.id" class="node-card" :class="{ 'node-enabled': node.enabled }">
      <template #header>
        <div class="node-header">
          <div class="node-title">
            <el-switch
              :model-value="node.enabled"
              :disabled="node.enabled && nodes.filter(n => n.enabled).length <= 1"
              @change="(val: boolean) => handleToggleNode(node, val)"
              style="margin-right: 12px;"
            />
            <span class="node-name" :class="{ enabled: node.enabled }">{{ node.name }}</span>
            <el-tag v-if="node.enabled" type="success" size="small">启用中</el-tag>
            <el-tag v-else type="info" size="small">禁用</el-tag>
          </div>
          <div class="node-actions">
            <el-button size="small" @click="showEditNodeDialog(node)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDeleteNode(node)">删除</el-button>
          </div>
        </div>
        <div class="node-url">{{ node.url }}</div>
      </template>

      <div class="keys-section">
        <div class="keys-header">
          <span>Keys（{{ (node.keys || []).length }}）</span>
          <el-button size="small" type="primary" @click="showAddKeyDialog(node)">添加 Key</el-button>
        </div>
        <el-table :data="node.keys || []" style="width: 100%" v-if="(node.keys || []).length > 0">
          <el-table-column prop="name" label="名称" width="140">
            <template #default="{ row }">
              {{ row.name || '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="key" label="Key" min-width="280">
            <template #default="{ row }">
              <div class="key-cell">
                <span class="key-text" v-if="visibleKeys[row.key]">{{ row.key }}</span>
                <span class="key-text" v-else>{{ row.key.substring(0, 12) }}****</span>
                <el-button size="small" link @click="toggleKeyVisibility(row.key)">
                  <el-icon><View v-if="!visibleKeys[row.key]"/><Hide v-else/></el-icon>
                </el-button>
                <el-button size="small" link @click="copyKey(row.key)">
                  <el-icon><CopyDocument /></el-icon>
                </el-button>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="enabled" label="启用" width="80">
            <template #default="{ row }">
              <el-switch
                v-model="row.enabled"
                @change="handleToggleKey(node, row)"
              />
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="150" />
          <el-table-column label="操作" width="80">
            <template #default="{ row }">
              <el-button size="small" type="danger" @click="handleDeleteKey(node, row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="暂无 Key" :image-size="48" />
      </div>
    </el-card>

    <el-empty v-if="nodes.length === 0" description="暂无模型节点" />

    <!-- 新增/编辑节点对话框 -->
    <el-dialog v-model="nodeDialogVisible" :title="isEditingNode ? '编辑节点' : '新增节点'" width="480px">
      <el-form label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="nodeForm.name" placeholder="如：FreeLLM主节点" />
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="nodeForm.url" placeholder="http://127.0.0.1:8080" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nodeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingNode" @click="handleSaveNode">确定</el-button>
      </template>
    </el-dialog>

    <!-- 添加 Key 对话框 -->
    <el-dialog v-model="keyDialogVisible" title="添加 Key" width="480px">
      <el-form label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="keyForm.name" placeholder="如：主Key" />
        </el-form-item>
        <el-form-item label="Key">
          <el-input v-model="keyForm.key" placeholder="输入 API Key" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="keyDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingKey" @click="handleSaveKey">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { View, Hide, CopyDocument } from '@element-plus/icons-vue'
import axios from 'axios'

interface ModelKey {
  key: string
  name: string
  enabled: boolean
  created_at: string
}

interface ModelNode {
  id: string
  name: string
  url: string
  enabled: boolean
  keys: ModelKey[]
}

const nodes = ref<ModelNode[]>([])
const visibleKeys = ref<Record<string, boolean>>({})

// 节点对话框
const nodeDialogVisible = ref(false)
const isEditingNode = ref(false)
const savingNode = ref(false)
const nodeForm = ref({ id: '', name: '', url: '' })
const editingNodeId = ref('')

// Key 对话框
const keyDialogVisible = ref(false)
const savingKey = ref(false)
const keyForm = ref({ name: '', key: '' })
const addingKeyNodeId = ref('')

const fetchNodes = async () => {
  try {
    const response = await axios.get('/admin/modelnodes')
    nodes.value = response.data || []
  } catch (error) {
    console.error('获取节点失败:', error)
  }
}

// 节点操作
const showCreateNodeDialog = () => {
  isEditingNode.value = false
  nodeForm.value = { id: '', name: '', url: '' }
  nodeDialogVisible.value = true
}

const showEditNodeDialog = (node: ModelNode) => {
  isEditingNode.value = true
  editingNodeId.value = node.id
  nodeForm.value = { id: node.id, name: node.name, url: node.url }
  nodeDialogVisible.value = true
}

const handleSaveNode = async () => {
  if (!nodeForm.value.name.trim() || !nodeForm.value.url.trim()) {
    ElMessage.warning('请填写完整')
    return
  }
  savingNode.value = true
  try {
    if (isEditingNode.value) {
      const node = nodes.value.find(n => n.id === editingNodeId.value)
      await axios.put(`/admin/modelnodes/${editingNodeId.value}`, {
        name: nodeForm.value.name,
        url: nodeForm.value.url,
        keys: node?.keys || [],
      })
      ElMessage.success('更新成功')
    } else {
      await axios.post('/admin/modelnodes', { name: nodeForm.value.name, url: nodeForm.value.url })
      ElMessage.success('创建成功')
    }
    nodeDialogVisible.value = false
    fetchNodes()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '操作失败')
  } finally {
    savingNode.value = false
  }
}

const handleDeleteNode = async (node: ModelNode) => {
  try {
    await ElMessageBox.confirm(`确定要删除节点「${node.name}」吗？`, '确认删除', { type: 'warning' })
    await axios.delete(`/admin/modelnodes/${node.id}`)
    ElMessage.success('删除成功')
    fetchNodes()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const handleToggleNode = async (node: ModelNode, enabled: boolean) => {
  try {
    await axios.put(`/admin/modelnodes/${node.id}/toggle`, { enabled })
    ElMessage.success(enabled ? '已启用' : '已禁用')
    fetchNodes()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '操作失败')
    fetchNodes()
  }
}

// Key 操作
const showAddKeyDialog = (node: ModelNode) => {
  addingKeyNodeId.value = node.id
  keyForm.value = { name: '', key: '' }
  keyDialogVisible.value = true
}

const handleSaveKey = async () => {
  if (!keyForm.value.key.trim()) {
    ElMessage.warning('请填写 Key')
    return
  }
  savingKey.value = true
  try {
    await axios.post(`/admin/modelnodes/${addingKeyNodeId.value}/keys`, keyForm.value)
    ElMessage.success('添加成功')
    keyDialogVisible.value = false
    fetchNodes()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '添加失败')
  } finally {
    savingKey.value = false
  }
}

const handleDeleteKey = async (node: ModelNode, row: ModelKey) => {
  try {
    await ElMessageBox.confirm('确定要删除这个 Key 吗？', '确认删除', { type: 'warning' })
    await axios.delete(`/admin/modelnodes/${node.id}/keys/${row.key}`)
    ElMessage.success('删除成功')
    fetchNodes()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const handleToggleKey = async (node: ModelNode, row: ModelKey) => {
  try {
    await axios.put(`/admin/modelnodes/${node.id}/keys/${row.key}/toggle`, { enabled: row.enabled })
    ElMessage.success('更新成功')
    fetchNodes()
  } catch (error) {
    ElMessage.error('更新失败')
    fetchNodes()
  }
}

const toggleKeyVisibility = (key: string) => {
  visibleKeys.value[key] = !visibleKeys.value[key]
}

const copyKey = (key: string) => {
  navigator.clipboard.writeText(key)
  ElMessage.success('已复制到剪贴板')
}

onMounted(() => {
  fetchNodes()
})
</script>

<style scoped>
.node-card {
  margin-bottom: 16px;
}
.node-enabled {
  border-color: #67c23a;
}
.node-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.node-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
.node-name {
  font-size: 16px;
  font-weight: 600;
}
.node-name.enabled {
  color: #67c23a;
}
.node-url {
  margin-top: 4px;
  font-size: 13px;
  color: #909399;
  font-family: monospace;
}
.keys-section {
  margin-top: 8px;
}
.keys-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-size: 14px;
  color: #606266;
}
.key-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}
.key-text {
  font-family: monospace;
  font-size: 13px;
}
.node-actions {
  display: flex;
  gap: 8px;
}
</style>
