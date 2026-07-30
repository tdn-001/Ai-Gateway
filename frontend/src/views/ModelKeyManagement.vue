<template>
  <div class="modelkey-management">
    <div class="page-header">
      <h1 class="page-title">模型节点 Keys</h1>
      <el-button type="primary" @click="showCreateDialog">
        新增 Key
      </el-button>
    </div>
    
    <el-alert
      type="info"
      show-icon
      :closable="false"
      style="margin-bottom: 16px;"
    >
      模型节点Key用于调用Nginx上游（FreeLLM）时的身份验证。启用的Key会通过 <code>Authorization: Bearer xxx</code> 传递给上游节点。多个启用的Key将按顺序轮询使用。
    </el-alert>
    
    <el-card>
      <el-table :data="modelKeys" style="width: 100%">
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
        <el-table-column prop="enabled" label="启用" width="100">
          <template #default="scope">
            <el-switch 
              v-model="scope.row.enabled" 
              @change="handleToggle(scope.row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160" />
        <el-table-column label="操作" width="100">
          <template #default="scope">
            <el-button size="small" type="danger" @click="handleDelete(scope.row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    
    <el-dialog v-model="dialogVisible" title="新增模型节点 Key" width="400px">
      <el-form label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="newKey.name" placeholder="如：FreeLLM主节点" />
        </el-form-item>
        <el-form-item label="Key">
          <el-input v-model="newKey.key" placeholder="输入节点的API Key" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">确定</el-button>
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

const modelKeys = ref<ModelKey[]>([])
const dialogVisible = ref(false)
const visibleKeys = ref<Record<string, boolean>>({})
const creating = ref(false)
const newKey = ref({ name: '', key: '' })

const fetchModelKeys = async () => {
  try {
    const response = await axios.get('/admin/modelkeys')
    modelKeys.value = response.data || []
  } catch (error) {
    console.error('获取模型Keys失败:', error)
  }
}

const showCreateDialog = () => {
  newKey.value = { name: '', key: '' }
  dialogVisible.value = true
}

const handleCreate = async () => {
  if (!newKey.value.name.trim() || !newKey.value.key.trim()) {
    ElMessage.warning('请填写完整')
    return
  }
  
  creating.value = true
  try {
    await axios.post('/admin/modelkeys', newKey.value)
    ElMessage.success('创建成功')
    dialogVisible.value = false
    fetchModelKeys()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '创建失败')
  } finally {
    creating.value = false
  }
}

const handleDelete = async (row: ModelKey) => {
  try {
    await ElMessageBox.confirm('确定要删除这个Key吗？', '确认删除', { type: 'warning' })
    await axios.delete(`/admin/modelkeys/${row.key}`)
    ElMessage.success('删除成功')
    fetchModelKeys()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const handleToggle = async (row: ModelKey) => {
  try {
    await axios.put(`/admin/modelkeys/${row.key}/toggle`, { enabled: row.enabled })
    ElMessage.success('更新成功')
    fetchModelKeys()
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

onMounted(() => {
  fetchModelKeys()
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
</style>
