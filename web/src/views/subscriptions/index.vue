<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { subscriptionApi } from '@/api/subscription'
import { sshKeyApi } from '@/api/notification'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'

const subList = ref<any[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const selectedIds = ref<number[]>([])

const showEditDialog = ref(false)
const showLogDialog = ref(false)
const isCreate = ref(true)

const editForm = ref({
  id: 0,
  name: '',
  type: 'git-repo',
  url: '',
  branch: '',
  schedule: '',
  whitelist: '',
  blacklist: '',
  depend_on: '',
  auto_add_task: false,
  auto_del_task: false,
  save_dir: '',
  ssh_key_id: null as number | null,
  alias: ''
})

const sshKeys = ref<any[]>([])
const logList = ref<any[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logSubId = ref(0)
const logLoading = ref(false)

const showPullLog = ref(false)
const pullLogLines = ref<string[]>([])
const pullRunning = ref(false)
let pullEventSource: EventSource | null = null
const pullLogRef = ref<HTMLElement>()

async function loadData() {
  loading.value = true
  try {
    const res = await subscriptionApi.list({
      keyword: keyword.value || undefined,
      page: page.value,
      page_size: pageSize.value
    })
    subList.value = res.data || []
    total.value = res.total || 0
  } catch {
    ElMessage.error('加载订阅列表失败')
  } finally {
    loading.value = false
  }
}

async function loadSSHKeys() {
  try {
    const res = await sshKeyApi.list()
    sshKeys.value = res.data || []
  } catch { /* ignore */ }
}

onMounted(() => {
  loadData()
  loadSSHKeys()
})

onBeforeUnmount(() => {
  closePullStream()
})

function handleSearch() {
  page.value = 1
  loadData()
}

function openCreate() {
  isCreate.value = true
  editForm.value = {
    id: 0, name: '', type: 'git-repo', url: '', branch: '', schedule: '',
    whitelist: '', blacklist: '', depend_on: '', auto_add_task: false,
    auto_del_task: false, save_dir: '', ssh_key_id: null, alias: ''
  }
  showEditDialog.value = true
}

function openEdit(row: any) {
  isCreate.value = false
  editForm.value = {
    id: row.id, name: row.name, type: row.type, url: row.url,
    branch: row.branch || '', schedule: row.schedule || '',
    whitelist: row.whitelist || '', blacklist: row.blacklist || '',
    depend_on: row.depend_on || '', auto_add_task: row.auto_add_task,
    auto_del_task: row.auto_del_task, save_dir: row.save_dir || '',
    ssh_key_id: row.ssh_key_id, alias: row.alias || ''
  }
  showEditDialog.value = true
}

async function handleSave() {
  if (!editForm.value.name.trim() || !editForm.value.url.trim()) {
    ElMessage.warning('名称和 URL 不能为空')
    return
  }
  try {
    const data = { ...editForm.value }
    if (isCreate.value) {
      await subscriptionApi.create(data)
      ElMessage.success('创建成功')
    } else {
      await subscriptionApi.update(data.id, data)
      ElMessage.success('更新成功')
    }
    showEditDialog.value = false
    loadData()
  } catch {
    ElMessage.error(isCreate.value ? '创建失败' : '更新失败')
  }
}

async function handleDelete(id: number) {
  try {
    await ElMessageBox.confirm('确定要删除该订阅吗？', '确认删除', { type: 'warning' })
    await subscriptionApi.delete(id)
    ElMessage.success('删除成功')
    loadData()
  } catch { /* cancelled */ }
}

async function handleToggle(row: any) {
  try {
    if (row.enabled) {
      await subscriptionApi.disable(row.id)
    } else {
      await subscriptionApi.enable(row.id)
    }
    loadData()
  } catch {
    ElMessage.error('操作失败')
  }
}

async function handlePull(id: number) {
  try {
    await subscriptionApi.pull(id)
    pullLogLines.value = []
    pullRunning.value = true
    showPullLog.value = true
    connectPullStream(id)
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '拉取失败')
  }
}

function connectPullStream(id: number) {
  closePullStream()
  const auth = useAuthStore()
  const base = import.meta.env.VITE_API_BASE || '/api/v1'
  const url = `${base}/subscriptions/${id}/pull-stream?token=${auth.accessToken}`
  pullEventSource = new EventSource(url)
  pullEventSource.onmessage = (e) => {
    pullLogLines.value.push(e.data)
    nextTick(() => {
      if (pullLogRef.value) pullLogRef.value.scrollTop = pullLogRef.value.scrollHeight
    })
  }
  pullEventSource.addEventListener('done', () => {
    pullRunning.value = false
    closePullStream()
    loadData()
  })
  pullEventSource.onerror = () => {
    pullRunning.value = false
    closePullStream()
  }
}

function closePullStream() {
  if (pullEventSource) {
    pullEventSource.close()
    pullEventSource = null
  }
}

async function handleBatchDelete() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(`确定要删除选中的 ${selectedIds.value.length} 个订阅吗？`, '批量删除', { type: 'warning' })
    await subscriptionApi.batchDelete(selectedIds.value)
    ElMessage.success('批量删除成功')
    selectedIds.value = []
    loadData()
  } catch { /* cancelled */ }
}

function handleSelectionChange(rows: any[]) {
  selectedIds.value = rows.map(r => r.id)
}

async function openLogs(subId: number) {
  logSubId.value = subId
  logPage.value = 1
  showLogDialog.value = true
  await loadLogs()
}

async function loadLogs() {
  logLoading.value = true
  try {
    const res = await subscriptionApi.logs(logSubId.value, { page: logPage.value, page_size: 10 })
    logList.value = res.data || []
    logTotal.value = res.total || 0
  } catch {
    ElMessage.error('加载日志失败')
  } finally {
    logLoading.value = false
  }
}

function getStatusTag(status: number) {
  return status === 0 ? 'success' : 'danger'
}

function getStatusText(status: number) {
  return status === 0 ? '正常' : '失败'
}
</script>

<template>
  <div class="subscriptions-page">
    <div class="page-header">
      <div class="header-left">
        <el-input
          v-model="keyword"
          placeholder="搜索订阅名称或 URL"
          clearable
          style="width: 260px"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>新建
        </el-button>
        <el-button @click="handleBatchDelete" :disabled="selectedIds.length === 0">
          <el-icon><Delete /></el-icon>批量删除
        </el-button>
      </div>
    </div>

    <el-table
      :data="subList"
      v-loading="loading"
      @selection-change="handleSelectionChange"
      stripe
    >
      <el-table-column type="selection" width="50" />
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column prop="type" label="类型" width="120">
        <template #default="{ row }">
          <el-tag size="small" :type="row.type === 'git-repo' ? '' : 'warning'">
            {{ row.type === 'git-repo' ? 'Git 仓库' : '单文件' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="url" label="URL" min-width="250" show-overflow-tooltip />
      <el-table-column prop="branch" label="分支" width="100" />
      <el-table-column label="状态" width="80" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="getStatusTag(row.status)">{{ getStatusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80" align="center">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" size="small" @change="handleToggle(row)" />
        </template>
      </el-table-column>
      <el-table-column prop="last_pull_at" label="最后拉取" width="170">
        <template #default="{ row }">
          {{ row.last_pull_at ? new Date(row.last_pull_at).toLocaleString() : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <div class="action-group">
            <el-tooltip content="拉取" placement="top">
              <el-button size="small" type="success" plain circle @click="handlePull(row.id)">
                <el-icon><Download /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="日志" placement="top">
              <el-button size="small" type="info" plain circle @click="openLogs(row.id)">
                <el-icon><Tickets /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="编辑" placement="top">
              <el-button size="small" type="primary" plain circle @click="openEdit(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="删除" placement="top">
              <el-button size="small" type="danger" plain circle @click="handleDelete(row.id)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-container" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="loadData"
        @size-change="() => { page = 1; loadData() }"
      />
    </div>

    <el-dialog v-model="showEditDialog" :title="isCreate ? '新建订阅' : '编辑订阅'" width="600px">
      <el-form :model="editForm" label-width="100px">
        <el-form-item label="名称">
          <el-input v-model="editForm.name" placeholder="订阅名称" />
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="editForm.type">
            <el-radio value="git-repo">Git 仓库</el-radio>
            <el-radio value="single-file">单文件</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="URL">
          <el-input v-model="editForm.url" placeholder="仓库地址或文件下载链接" />
        </el-form-item>
        <el-form-item v-if="editForm.type === 'git-repo'" label="分支">
          <el-input v-model="editForm.branch" placeholder="默认分支 (留空使用默认)" />
        </el-form-item>
        <el-form-item label="定时拉取">
          <el-input v-model="editForm.schedule" placeholder="cron 表达式 (留空不自动拉取)" />
        </el-form-item>
        <el-form-item label="保存目录">
          <el-input v-model="editForm.save_dir" placeholder="保存到 scripts 下的子目录" />
        </el-form-item>
        <el-form-item label="别名">
          <el-input v-model="editForm.alias" placeholder="目录/文件别名" />
        </el-form-item>
        <el-form-item v-if="editForm.type === 'git-repo'" label="SSH 密钥">
          <el-select v-model="editForm.ssh_key_id" placeholder="选择 SSH 密钥 (可选)" clearable style="width: 100%">
            <el-option v-for="key in sshKeys" :key="key.id" :label="key.name" :value="key.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="白名单">
          <el-input v-model="editForm.whitelist" placeholder="文件名白名单 (逗号分隔)" />
        </el-form-item>
        <el-form-item label="黑名单">
          <el-input v-model="editForm.blacklist" placeholder="文件名黑名单 (逗号分隔)" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSave">{{ isCreate ? '创建' : '保存' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showLogDialog" title="拉取日志" width="700px">
      <el-table :data="logList" v-loading="logLoading" max-height="400px">
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 0 ? 'success' : 'danger'">
              {{ row.status === 0 ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="content" label="内容" show-overflow-tooltip />
        <el-table-column prop="duration" label="耗时" width="100">
          <template #default="{ row }">{{ row.duration.toFixed(1) }}s</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
        </el-table-column>
      </el-table>
      <div class="pagination-container" v-if="logTotal > 10" style="margin-top: 12px">
        <el-pagination
          v-model:current-page="logPage"
          :total="logTotal"
          :page-size="10"
          layout="prev, pager, next"
          @current-change="loadLogs"
        />
      </div>
    </el-dialog>

    <el-dialog v-model="showPullLog" title="拉取日志" width="700px" :close-on-click-modal="false" @close="closePullStream">
      <div ref="pullLogRef" class="pull-log-content">
        <div v-for="(line, i) in pullLogLines" :key="i" class="pull-log-line">{{ line }}</div>
        <div v-if="pullRunning" class="pull-log-line pull-running">拉取中...</div>
        <el-empty v-if="!pullRunning && pullLogLines.length === 0" description="暂无输出" :image-size="60" />
      </div>
      <template #footer>
        <el-tag v-if="pullRunning" type="warning" effect="plain" size="small" style="margin-right: auto">运行中</el-tag>
        <el-tag v-else-if="pullLogLines.length > 0" type="success" effect="plain" size="small" style="margin-right: auto">已完成</el-tag>
        <el-button @click="showPullLog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.subscriptions-page {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

.pagination-container {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.action-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pull-log-content {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: var(--dd-font-mono, monospace);
  font-size: 13px;
  line-height: 1.6;
  padding: 12px 16px;
  border-radius: 6px;
  max-height: 400px;
  overflow-y: auto;
}

.pull-log-line {
  white-space: pre-wrap;
  word-break: break-all;
}

.pull-running {
  color: #e6a23c;
  animation: blink 1s infinite;
}

@keyframes blink {
  50% { opacity: 0.5; }
}
</style>
