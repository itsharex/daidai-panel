<template>
  <div class="deps-page">
    <div class="page-header">
      <div>
        <h2>依赖管理</h2>
        <span class="page-subtitle">安装和管理 Node.js、Python3、Linux 软件包依赖</span>
      </div>
    </div>
    <el-tabs v-model="activeTab" @tab-change="loadData">
      <el-tab-pane label="Node.js" name="nodejs" />
      <el-tab-pane label="Python3" name="python" />
      <el-tab-pane label="Linux" name="linux" />
    </el-tabs>
    <div class="deps-toolbar">
      <el-button type="primary" @click="createType = activeTab; showCreateDialog = true">
        <el-icon><Plus /></el-icon>新建依赖
      </el-button>
      <el-button @click="loadData" :loading="loading">
        <el-icon><Refresh /></el-icon>刷新
      </el-button>
      <el-button type="danger" plain @click="handleBatchDelete" :disabled="selectedIds.length === 0">
        <el-icon><Delete /></el-icon>批量卸载
      </el-button>
      <el-button @click="openMirrorDialog">
        <el-icon><Setting /></el-icon>镜像源设置
      </el-button>
      <div class="deps-stats">
        <div class="stat-item" :class="{ active: activeTab === 'nodejs' }" @click="activeTab = 'nodejs'; loadData()">
          <span class="stat-label">Node.js</span>
          <span class="stat-value">{{ nodejsCount }}</span>
        </div>
        <div class="stat-item" :class="{ active: activeTab === 'python' }" @click="activeTab = 'python'; loadData()">
          <span class="stat-label">Python</span>
          <span class="stat-value">{{ pythonCount }}</span>
        </div>
        <div class="stat-item" :class="{ active: activeTab === 'linux' }" @click="activeTab = 'linux'; loadData()">
          <span class="stat-label">Linux</span>
          <span class="stat-value">{{ linuxCount }}</span>
        </div>
      </div>
    </div>
    <div v-if="isMobile" class="dd-mobile-list">
      <div
        v-for="(row, index) in depsList"
        :key="row.id"
        class="dd-mobile-card"
      >
        <div class="dd-mobile-card__header">
          <div class="dd-mobile-card__title-wrap">
            <div class="deps-card__title-row">
              <div class="dd-mobile-card__selection">
                <el-checkbox :model-value="isSelected(row.id)" @change="toggleSelected(row.id, $event)" />
                <span class="dd-mobile-card__title">{{ row.name }}</span>
              </div>
              <span class="dd-mobile-card__subtitle">#{{ index + 1 }}</span>
            </div>
          </div>
        </div>
        <div class="dd-mobile-card__body">
          <div class="dd-mobile-card__grid">
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">状态</span>
              <div class="dd-mobile-card__value">
                <el-tag :type="statusType(row.status)" size="small" effect="light">{{ statusLabel(row.status) }}</el-tag>
              </div>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">创建时间</span>
              <span class="dd-mobile-card__value">{{ new Date(row.created_at).toLocaleString('zh-CN') }}</span>
            </div>
          </div>
          <div class="dd-mobile-card__actions deps-card__actions">
            <el-button size="small" type="primary" plain @click="viewLog(row)">日志</el-button>
            <el-button
              v-if="row.status === 'installing' || row.status === 'removing'"
              size="small"
              type="warning"
              plain
              @click="handleCancel(row)"
            >
              取消
            </el-button>
            <el-button
              size="small"
              type="warning"
              plain
              @click="handleReinstall(row)"
              :disabled="row.status === 'installing' || row.status === 'removing'"
            >
              重装
            </el-button>
            <el-button
              size="small"
              type="danger"
              plain
              @click="handleDelete(row)"
              :disabled="row.status === 'installing' || row.status === 'removing'"
            >
              卸载
            </el-button>
            <el-button
              size="small"
              type="danger"
              @click="handleForceDelete(row)"
              :disabled="row.status === 'installing' || row.status === 'removing'"
            >
              强制卸载
            </el-button>
          </div>
        </div>
      </div>

      <el-empty v-if="!loading && depsList.length === 0" description="暂无依赖" />
    </div>

    <el-table v-else :data="depsList" v-loading="loading" border size="small" @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="40" />
      <el-table-column label="#" width="55" align="center">
        <template #default="{ $index }">{{ $index + 1 }}</template>
      </el-table-column>
      <el-table-column prop="name" label="名称" min-width="200" />
      <el-table-column label="状态" width="120" align="center">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small" effect="light">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
        <template #default="{ row }">{{ new Date(row.created_at).toLocaleString('zh-CN') }}</template>
      </el-table-column>
      <el-table-column label="操作" width="250" align="center">
        <template #default="{ row }">
          <el-button type="primary" link size="small" @click="viewLog(row)">日志</el-button>
          <el-button v-if="row.status === 'installing' || row.status === 'removing'" type="warning" link size="small" @click="handleCancel(row)">取消</el-button>
          <el-button type="warning" link size="small" @click="handleReinstall(row)" :disabled="row.status === 'installing' || row.status === 'removing'">重装</el-button>
          <el-button type="danger" link size="small" @click="handleDelete(row)" :disabled="row.status === 'installing' || row.status === 'removing'">卸载</el-button>
          <el-button type="danger" link size="small" @click="handleForceDelete(row)" :disabled="row.status === 'installing' || row.status === 'removing'">强制卸载</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-dialog v-model="showCreateDialog" title="新建依赖" width="500px" :fullscreen="dialogFullscreen">
      <el-form label-width="80px">
        <el-form-item label="类型">
          <el-radio-group v-model="createType">
            <el-radio value="nodejs">Node.js</el-radio>
            <el-radio value="python">Python3</el-radio>
            <el-radio value="linux">Linux</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="createNames" type="textarea" :rows="5" placeholder="每行一个依赖名称，支持换行/空格/逗号分隔" />
        </el-form-item>
        <el-form-item label="自动拆分">
          <el-switch v-model="autoSplit" />
          <span style="margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary)">开启后自动按换行、空格、逗号拆分为多个依赖</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">安装</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="showLogDialog" title="安装日志" width="70%" :fullscreen="dialogFullscreen">
      <div class="log-dialog-toolbar">
        <div>
          <el-tag v-if="!logDone" type="warning" size="small" class="running-tag">
            <span class="spinner"></span> 执行中
          </el-tag>
          <el-tag v-else-if="currentLogRow?.status === 'cancelled'" type="info" size="small">已取消</el-tag>
          <el-tag v-else type="success" size="small">已完成</el-tag>
        </div>
        <el-button
          v-if="currentLogRow && !logDone"
          type="warning"
          plain
          size="small"
          @click="handleCancel(currentLogRow)"
        >
          取消当前任务
        </el-button>
      </div>
      <pre ref="logContainerRef" class="log-content">{{ logContent || '暂无日志' }}</pre>
    </el-dialog>
    <el-dialog v-model="showMirrorDialog" title="软件包镜像源设置" width="560px" :fullscreen="dialogFullscreen">
      <el-form label-width="110px" v-loading="mirrorLoading">
        <el-form-item label="Python (pip)">
          <el-input v-model="mirrorForm.pip_mirror" placeholder="留空恢复默认加速源" clearable>
            <template #append>
              <el-dropdown @command="(v: string) => mirrorForm.pip_mirror = v" trigger="click">
                <el-button>快捷选择</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="https://pypi.tuna.tsinghua.edu.cn/simple">清华大学</el-dropdown-item>
                    <el-dropdown-item command="https://mirrors.aliyun.com/pypi/simple">阿里云</el-dropdown-item>
                    <el-dropdown-item command="https://pypi.doubanio.com/simple">豆瓣</el-dropdown-item>
                    <el-dropdown-item command="https://mirrors.cloud.tencent.com/pypi/simple">腾讯云</el-dropdown-item>
                    <el-dropdown-item command="https://repo.huaweicloud.com/repository/pypi/simple">华为云</el-dropdown-item>
                    <el-dropdown-item command="">恢复默认加速源</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="Node.js (npm)">
          <el-input v-model="mirrorForm.npm_mirror" placeholder="留空恢复默认加速源" clearable>
            <template #append>
              <el-dropdown @command="(v: string) => mirrorForm.npm_mirror = v" trigger="click">
                <el-button>快捷选择</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="https://registry.npmmirror.com">淘宝 (npmmirror)</el-dropdown-item>
                    <el-dropdown-item command="https://mirrors.cloud.tencent.com/npm/">腾讯云</el-dropdown-item>
                    <el-dropdown-item command="https://repo.huaweicloud.com/repository/npm/">华为云</el-dropdown-item>
                    <el-dropdown-item command="">恢复默认加速源</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item :label="linuxMirrorLabel">
          <el-input
            v-model="mirrorForm.linux_mirror"
            :placeholder="linuxMirrorSupported ? '留空恢复默认加速源' : '当前包管理器暂不支持镜像设置'"
            :disabled="!linuxMirrorSupported"
            clearable
          >
            <template #append>
              <el-dropdown @command="(v: string) => mirrorForm.linux_mirror = v" trigger="click" :disabled="!linuxMirrorSupported || linuxMirrorOptions.length === 0">
                <el-button :disabled="!linuxMirrorSupported || linuxMirrorOptions.length === 0">快捷选择</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="option in linuxMirrorOptions"
                      :key="option.value"
                      :command="option.value"
                    >
                      {{ option.label }}
                    </el-dropdown-item>
                    <el-dropdown-item command="">恢复默认加速源</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-input>
          <div class="mirror-hint">
            当前检测：{{ linuxMirrorManagerText }}
            <span v-if="linuxMirrorDistributionText"> / {{ linuxMirrorDistributionText }}</span>
            <span v-if="linuxMirrorMessage">。{{ linuxMirrorMessage }}</span>
          </div>
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon>
          依赖管理默认优先使用加速源；清空输入框并保存，会恢复到内置的默认加速源配置。
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="showMirrorDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSaveMirrors" :loading="mirrorSaving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, computed } from 'vue'
import { depsApi, type MirrorsResponse } from '@/api/deps'
import { ElMessage, ElMessageBox } from 'element-plus'
import { openAuthorizedEventStream, type EventStreamConnection } from '@/utils/sse'
import { useResponsive } from '@/composables/useResponsive'

const activeTab = ref('nodejs')
const depsList = ref<any[]>([])
const loading = ref(false)
const showCreateDialog = ref(false)
const showLogDialog = ref(false)
const logContent = ref('')
const logDone = ref(true)
const currentLogRow = ref<any | null>(null)
let eventSource: EventStreamConnection | null = null
const logContainerRef = ref<HTMLElement>()
let depsLogBuffer: string[] = []
let depsLogFlushRaf = 0
const createType = ref('nodejs')
const createNames = ref('')
const autoSplit = ref(true)
const creating = ref(false)
const selectedIds = ref<number[]>([])
const selectedIdSet = computed(() => new Set(selectedIds.value))
let refreshTimer: ReturnType<typeof setInterval> | null = null
const { isMobile, dialogFullscreen } = useResponsive()

const showMirrorDialog = ref(false)
const mirrorLoading = ref(false)
const mirrorSaving = ref(false)
const mirrorForm = ref({ pip_mirror: '', npm_mirror: '', linux_mirror: '' })
const mirrorMeta = ref<MirrorsResponse>({
  pip_mirror: '',
  npm_mirror: '',
  linux_mirror: '',
  linux_package_manager: '',
  linux_distribution: '',
  linux_mirror_supported: false,
  linux_mirror_label: 'Linux',
  linux_mirror_message: '',
})

const nodejsCount = ref(0)
const pythonCount = ref(0)
const linuxCount = ref(0)

function statusType(status: string) {
  switch (status) {
    case 'installed': return 'success'
    case 'installing': return 'warning'
    case 'removing': return 'warning'
    case 'cancelled': return 'info'
    case 'failed': return 'danger'
    default: return 'info'
  }
}

function statusLabel(status: string) {
  switch (status) {
    case 'installed': return '已安装'
    case 'installing': return '安装中'
    case 'removing': return '卸载中'
    case 'cancelled': return '已取消'
    case 'failed': return '失败'
    default: return status
  }
}

const linuxMirrorLabel = computed(() => mirrorMeta.value.linux_mirror_label || 'Linux')
const linuxMirrorSupported = computed(() => mirrorMeta.value.linux_mirror_supported)
const linuxMirrorMessage = computed(() => mirrorMeta.value.linux_mirror_message || '')
const linuxMirrorManagerText = computed(() => mirrorMeta.value.linux_package_manager || '未识别')
const linuxMirrorDistributionText = computed(() => mirrorMeta.value.linux_distribution || '')
const linuxMirrorOptions = computed(() => {
  const manager = mirrorMeta.value.linux_package_manager
  const distro = mirrorMeta.value.linux_distribution

  if (manager === 'apk') {
    return [
      { label: '清华大学', value: 'https://mirrors.tuna.tsinghua.edu.cn/alpine' },
      { label: '阿里云', value: 'https://mirrors.aliyun.com/alpine' },
      { label: '腾讯云', value: 'https://mirrors.cloud.tencent.com/alpine' },
      { label: '华为云', value: 'https://repo.huaweicloud.com/alpine' },
      { label: '中科大', value: 'https://mirrors.ustc.edu.cn/alpine' },
    ]
  }

  if (manager === 'apt') {
    if (distro === 'debian') {
      return [
        { label: '清华大学 Debian', value: 'https://mirrors.tuna.tsinghua.edu.cn/debian' },
        { label: '阿里云 Debian', value: 'https://mirrors.aliyun.com/debian' },
        { label: '腾讯云 Debian', value: 'https://mirrors.cloud.tencent.com/debian' },
      ]
    }
    return [
      { label: '清华大学 Ubuntu', value: 'https://mirrors.tuna.tsinghua.edu.cn/ubuntu' },
      { label: '阿里云 Ubuntu', value: 'https://mirrors.aliyun.com/ubuntu' },
      { label: '腾讯云 Ubuntu', value: 'https://mirrors.cloud.tencent.com/ubuntu' },
      { label: '华为云 Ubuntu', value: 'https://repo.huaweicloud.com/ubuntu' },
    ]
  }

  return []
})

async function loadData() {
  loading.value = true
  try {
    const res = await depsApi.list(activeTab.value)
    depsList.value = res.data || []
    const countMap: Record<string, (v: number) => void> = {
      nodejs: (v) => nodejsCount.value = v,
      python: (v) => pythonCount.value = v,
      linux: (v) => linuxCount.value = v,
    }
    countMap[activeTab.value]?.(depsList.value.length)
    checkPending()
  } catch {
    depsList.value = []
  } finally {
    loading.value = false
  }
}

function checkPending() {
  const hasPending = depsList.value.some(d => d.status === 'installing' || d.status === 'removing')
  if (hasPending && !refreshTimer) {
    refreshTimer = setInterval(loadData, 3000)
  } else if (!hasPending && refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function parseNames(text: string): string[] {
  if (!autoSplit.value) return [text.trim()].filter(Boolean)
  return text.split(/[\n,\s]+/).map(s => s.trim()).filter(Boolean)
}

async function handleCreate() {
  const names = parseNames(createNames.value)
  if (names.length === 0) { ElMessage.warning('请输入依赖名称'); return }
  creating.value = true
  try {
    await depsApi.create(createType.value, names)
    ElMessage.success(`已提交 ${names.length} 个依赖安装`)
    showCreateDialog.value = false
    createNames.value = ''
    activeTab.value = createType.value
    loadData()
  } catch { ElMessage.error('提交安装失败') }
  finally { creating.value = false }
}

function handleSelectionChange(rows: any[]) {
  selectedIds.value = rows.map(r => r.id)
}

function isSelected(id: number) {
  return selectedIdSet.value.has(id)
}

function toggleSelected(id: number, checked: boolean | string | number) {
  const next = new Set(selectedIds.value)
  if (checked) {
    next.add(id)
  } else {
    next.delete(id)
  }
  selectedIds.value = [...next]
}

async function handleBatchDelete() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(`确定批量卸载选中的 ${selectedIds.value.length} 个依赖？`, '批量卸载', { type: 'warning' })
    await depsApi.batchDelete(selectedIds.value)
    ElMessage.success('批量卸载已提交')
    selectedIds.value = []
    loadData()
  } catch (err: any) {
    if (err !== 'cancel' && err?.toString() !== 'cancel') {
      ElMessage.error(err?.response?.data?.error || '批量卸载失败')
    }
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(`确认卸载 ${row.name}？`, '提示', { type: 'warning' })
    await depsApi.delete(row.id)
    ElMessage.success('卸载中')
    loadData()
  } catch {}
}

async function handleForceDelete(row: any) {
  try {
    await ElMessageBox.confirm(`确认强制卸载 ${row.name}？\n强制卸载会跳过依赖检查直接删除`, '强制卸载', { type: 'warning' })
    await depsApi.delete(row.id, true)
    ElMessage.success('强制卸载中')
    loadData()
  } catch {}
}

async function handleReinstall(row: any) {
  try { await depsApi.reinstall(row.id); ElMessage.success('重新安装中'); loadData() }
  catch { ElMessage.error('操作失败') }
}

async function handleCancel(row: any) {
  try {
    await depsApi.cancel(row.id)
    ElMessage.success('取消请求已提交')
    loadData()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || '取消失败')
  }
}

function viewLog(row: any) {
  currentLogRow.value = row
  logContent.value = ''
  logDone.value = !(row.status === 'installing' || row.status === 'removing')
  showLogDialog.value = true

  closeSSE()

  if (logDone.value) {
    depsApi.getStatus(row.id).then(res => {
      logContent.value = res.data?.log || '暂无日志'
    }).catch(() => { logContent.value = '获取日志失败' })
    return
  }

  const url = `/api/v1/deps/${row.id}/log-stream`
  eventSource = openAuthorizedEventStream(url, {
    onMessage(data) {
      depsLogBuffer.push(data)
      if (!depsLogFlushRaf) {
        depsLogFlushRaf = requestAnimationFrame(() => {
          logContent.value += depsLogBuffer.join('\n') + '\n'
          depsLogBuffer = []
          depsLogFlushRaf = 0
          if (logContainerRef.value) {
            logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
          }
        })
      }
    },
    onEvent(event) {
      if (event.event === 'done') {
        logDone.value = true
        closeSSE()
        loadData()
      }
    },
    onError() {
      logDone.value = true
      closeSSE()
      loadData()
    }
  })
}

function closeSSE() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

watch(showLogDialog, (val) => {
  if (!val) {
    closeSSE()
    currentLogRow.value = null
  }
})

async function openMirrorDialog() {
  showMirrorDialog.value = true
  mirrorLoading.value = true
  try {
    const res = await depsApi.getMirrors()
    mirrorMeta.value = res
    mirrorForm.value.pip_mirror = res.pip_mirror || ''
    mirrorForm.value.npm_mirror = res.npm_mirror || ''
    mirrorForm.value.linux_mirror = res.linux_mirror || ''
  } catch { ElMessage.error('获取镜像源配置失败') }
  finally { mirrorLoading.value = false }
}

async function handleSaveMirrors() {
  if (!linuxMirrorSupported.value && mirrorForm.value.linux_mirror.trim()) {
    ElMessage.warning(linuxMirrorMessage.value || '当前系统暂不支持 Linux 镜像设置')
    return
  }
  mirrorSaving.value = true
  try {
    await depsApi.setMirrors(mirrorForm.value)
    ElMessage.success('镜像源设置成功')
    showMirrorDialog.value = false
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || '设置失败')
  } finally { mirrorSaving.value = false }
}

onMounted(async () => {
  createType.value = activeTab.value
  loadData()
  const types = ['nodejs', 'python', 'linux'] as const
  const countRefs = { nodejs: nodejsCount, python: pythonCount, linux: linuxCount }
  for (const t of types) {
    if (t !== activeTab.value) {
      depsApi.list(t).then(res => { countRefs[t].value = (res.data || []).length }).catch(() => {})
    }
  }
})
onBeforeUnmount(() => {
  closeSSE()
  if (refreshTimer) clearInterval(refreshTimer)
  if (depsLogFlushRaf) { cancelAnimationFrame(depsLogFlushRaf); depsLogFlushRaf = 0 }
})
</script>

<style scoped lang="scss">
.deps-page { padding: 0; }

.page-header {
  margin-bottom: 16px;

  h2 { margin: 0; font-size: 20px; font-weight: 700; color: var(--el-text-color-primary); }

  .page-subtitle {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    display: block;
    margin-top: 2px;
  }
}

.deps-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.deps-stats {
  display: flex;
  align-items: center;
  gap: 1px;
  margin-left: auto;
  background: var(--el-border-color-lighter);
  border-radius: 6px;
  overflow: hidden;

  .stat-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 6px 16px;
    background: var(--el-bg-color);
    cursor: pointer;
    transition: all 0.2s;
    min-width: 64px;

    &:hover { background: var(--el-color-primary-light-9); }
    &.active {
      background: var(--el-color-primary-light-9);
      .stat-value { color: var(--el-color-primary); }
    }
  }

  .stat-label {
    font-size: 11px;
    color: var(--el-text-color-secondary);
    line-height: 1;
  }

  .stat-value {
    font-size: 18px;
    font-weight: 600;
    line-height: 1.4;
    color: var(--el-text-color-primary);
  }
}

.deps-card__title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.deps-card__actions > * {
  flex: 1 1 calc(50% - 4px);
}
.log-content {
  background: #1e1e1e;
  color: #d4d4d4;
  border-radius: 6px;
  padding: 16px;
  font-family: var(--dd-font-mono);
  font-size: 13px;
  line-height: 1.6;
  min-height: 200px;
  max-height: 60vh;
  overflow-y: auto;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-dialog-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.mirror-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
  margin-top: 6px;
}

.running-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.spinner {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid rgba(230, 162, 60, 0.3);
  border-top-color: #e6a23c;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@media (max-width: 768px) {
  .deps-toolbar {
    flex-wrap: wrap;
  }

  .deps-stats {
    margin-left: 0;
    width: 100%;
    justify-content: stretch;
  }

  .deps-stats .stat-item {
    flex: 1 1 0;
  }

  .deps-card__title-row {
    flex-direction: column;
  }
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
</style>
