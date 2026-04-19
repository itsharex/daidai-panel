<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, onActivated, computed, watch } from 'vue'
import { logApi } from '@/api/log'
import { taskApi } from '@/api/task'
import { ElMessage, ElMessageBox } from 'element-plus'
import { openAuthorizedEventStream, type EventStreamConnection } from '@/utils/sse'
import { usePageActivity } from '@/composables/usePageActivity'
import { useResponsive } from '@/composables/useResponsive'
import { extractError } from '@/utils/error'

const logs = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const statusFilter = ref<string>('')
const keyword = ref('')
const loading = ref(false)
const detailVisible = ref(false)
const detailContent = ref('')
const detailLog = ref<any>(null)
const selectedIds = ref<number[]>([])
const selectedIdSet = computed(() => new Set(selectedIds.value))
const autoRefresh = ref(true)
const { isMobile, dialogFullscreen } = useResponsive()
const { isPageActive } = usePageActivity()
let refreshTimer: ReturnType<typeof setInterval> | null = null
let logEventSource: EventStreamConnection | null = null
const logContentRef = ref<HTMLElement>()
let sseBuffer: string[] = []
let sseFlushRaf = 0

const showFileBrowser = ref(false)
const currentTaskId = ref<number>(0)
const logFiles = ref<any[]>([])
const logFilesLoading = ref(false)
const showFileContent = ref(false)
const fileContentData = ref('')
const fileContentName = ref('')
const hasRunningLogs = computed(() => logs.value.some(l => l.status === 2))
const runningOnPage = computed(() => logs.value.filter(l => l.status === 2).length)
const successOnPage = computed(() => logs.value.filter(l => l.status === 0).length)
const failedOnPage = computed(() => logs.value.filter(l => l.status === 1).length)
const allSelectedOnPage = computed(() => logs.value.length > 0 && logs.value.every(l => selectedIdSet.value.has(l.id)))
const someSelectedOnPage = computed(() => selectedIds.value.length > 0 && !allSelectedOnPage.value)

const detailLineCount = computed(() => {
  if (!detailContent.value) return 0
  return detailContent.value.split('\n').length
})
const detailByteLabel = computed(() => {
  if (!detailContent.value) return ''
  const bytes = new Blob([detailContent.value]).size
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
})

let mounted = false

async function loadLogs() {
  loading.value = true
  selectedIds.value = []
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (statusFilter.value !== '') params.status = statusFilter.value
    if (keyword.value) params.keyword = keyword.value
    const res = await logApi.list(params)
    logs.value = res.data
    total.value = res.total
  } catch (err) {
    ElMessage.error(extractError(err, '加载日志失败'))
  } finally {
    loading.value = false
    syncAutoRefresh()
  }
}

function startAutoRefresh() {
  stopAutoRefresh()
  refreshTimer = setInterval(async () => {
    if (!isPageActive.value || !autoRefresh.value) {
      stopAutoRefresh()
      return
    }
    await loadLogs()
    if (!hasRunningLogs.value) {
      stopAutoRefresh()
    }
  }, 5000)
}

function stopAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function syncAutoRefresh() {
  if (autoRefresh.value && hasRunningLogs.value && isPageActive.value) {
    if (!refreshTimer) {
      startAutoRefresh()
    }
    return
  }
  stopAutoRefresh()
}

watch([autoRefresh, hasRunningLogs, isPageActive], () => {
  syncAutoRefresh()
})

onMounted(async () => {
  mounted = true
  await loadLogs()
})

onActivated(() => {
  if (!mounted) {
    void loadLogs()
  }
  mounted = false
})

function handleSearch() {
  page.value = 1
  loadLogs()
}

function getStatusType(status: number | null) {
  if (status === 2) return 'warning'
  if (status === 0) return 'success'
  if (status === 1) return 'danger'
  return 'info'
}

function getStatusText(status: number | null) {
  if (status === 2) return '运行中'
  if (status === 0) return '成功'
  if (status === 1) return '失败'
  return '未知'
}

function formatDuration(d: number | null) {
  if (d == null) return '-'
  if (d < 60) return `${d.toFixed(1)}s`
  return `${Math.floor(d / 60)}m ${(d % 60).toFixed(0)}s`
}

function formatTime(t: string | null) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

async function viewDetail(log: any) {
  detailLog.value = log
  detailContent.value = ''
  detailVisible.value = true
  closeLogSSE()

  if (log.status === 2) {
    const url = `/api/v1/logs/${log.task_id}/stream`
    sseBuffer = []
    logEventSource = openAuthorizedEventStream(url, {
      onMessage(data) {
        sseBuffer.push(data)
        if (!sseFlushRaf) {
          sseFlushRaf = requestAnimationFrame(() => {
            detailContent.value += sseBuffer.join('\n') + '\n'
            sseBuffer = []
            sseFlushRaf = 0
            if (logContentRef.value) {
              logContentRef.value.scrollTop = logContentRef.value.scrollHeight
            }
          })
        }
      },
      onEvent(event) {
        if (event.event === 'done') {
          closeLogSSE()
          loadLogs()
        }
      },
      onError() {
        closeLogSSE()
      }
    })
  } else {
    try {
      const res = await logApi.detail(log.id)
      detailLog.value = res
      detailContent.value = res.content || '(无日志内容)'
    } catch (err) {
      ElMessage.error(extractError(err, '获取日志详情失败'))
    }
  }
}

function closeLogSSE() {
  if (logEventSource) {
    logEventSource.close()
    logEventSource = null
  }
}

function downloadCurrentLog() {
  if (!detailContent.value) {
    ElMessage.warning('暂无内容可下载')
    return
  }
  const taskName = detailLog.value?.task_name || 'log'
  const logId = detailLog.value?.id ?? 'detail'
  const filename = `${taskName}-${logId}.log`.replace(/[\\/:*?"<>|]/g, '_')
  const blob = new Blob([detailContent.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  ElMessage.success('已下载')
}

async function copyCurrentLog() {
  if (!detailContent.value) {
    ElMessage.warning('暂无内容可复制')
    return
  }
  try {
    await navigator.clipboard.writeText(detailContent.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    const ta = document.createElement('textarea')
    ta.value = detailContent.value
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    try { document.execCommand('copy'); ElMessage.success('已复制到剪贴板') }
    catch { ElMessage.error('复制失败，请切换 HTTPS 或手动复制') }
    document.body.removeChild(ta)
  }
}

async function handleDelete(log: any) {
  try {
    await ElMessageBox.confirm('确定删除此日志记录？', '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await logApi.delete(log.id)
    ElMessage.success('已删除')
    loadLogs()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '删除失败')
  }
}

async function handleClean() {
  let daysInput: string
  try {
    const res = await ElMessageBox.prompt('请输入保留天数（将清理该天数之前的日志）', '清理日志', {
      inputValue: '7',
      inputPattern: /^[1-9]\d*$/,
      inputErrorMessage: '请输入正整数',
      confirmButtonText: '清理',
      cancelButtonText: '取消',
      type: 'warning',
    })
    daysInput = res.value
  } catch {
    return
  }
  const days = parseInt(daysInput, 10)
  try {
    const res = await logApi.clean(days)
    ElMessage.success(res.message)
    loadLogs()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '清理失败')
  }
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

function toggleSelectAll(checked: boolean | string | number) {
  if (checked) {
    selectedIds.value = logs.value.map(l => l.id)
  } else {
    selectedIds.value = []
  }
}

function clearSelection() {
  selectedIds.value = []
}

async function handleBatchDelete() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${selectedIds.value.length} 条日志？`, '批量删除', { type: 'warning' })
    await logApi.batchDelete(selectedIds.value)
    ElMessage.success('批量删除成功')
    selectedIds.value = []
    loadLogs()
  } catch (err: any) {
    if (err !== 'cancel' && err?.toString() !== 'cancel') {
      ElMessage.error(err?.response?.data?.error || '批量删除失败')
    }
  }
}

function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    void loadLogs()
  } else {
    stopAutoRefresh()
  }
}

async function browseLogFiles(log: any) {
  currentTaskId.value = log.task_id
  logFiles.value = []
  showFileBrowser.value = true
  logFilesLoading.value = true
  try {
    const res = await taskApi.logFiles(log.task_id)
    logFiles.value = res || []
  } catch (err) {
    ElMessage.error(extractError(err, '获取日志文件列表失败'))
  } finally {
    logFilesLoading.value = false
  }
}

async function viewLogFile(file: any) {
  try {
    const res = await taskApi.logFileContent(currentTaskId.value, file.filename)
    fileContentData.value = res.content || '(空文件)'
    fileContentName.value = file.filename
    showFileContent.value = true
  } catch (err) {
    ElMessage.error(extractError(err, '读取日志文件失败'))
  }
}

async function deleteLogFile(file: any) {
  try {
    await ElMessageBox.confirm(`确定删除日志文件 ${file.filename}？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await taskApi.deleteLogFile(currentTaskId.value, file.filename)
    ElMessage.success('已删除')
    logFiles.value = logFiles.value.filter((f: any) => f.filename !== file.filename)
  } catch (err) {
    ElMessage.error(extractError(err, '删除失败'))
  }
}

function formatFileSize(size: number) {
  if (size < 1024) return size + ' B'
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB'
  return (size / 1024 / 1024).toFixed(1) + ' MB'
}

onBeforeUnmount(() => {
  stopAutoRefresh()
  closeLogSSE()
  if (sseFlushRaf) {
    cancelAnimationFrame(sseFlushRaf)
    sseFlushRaf = 0
  }
})
</script>

<template>
  <div class="logs-page">
    <!-- ======= Page hero ======= -->
    <header class="page-hero">
      <div class="page-hero-main">
        <div class="page-hero-title-row">
          <div class="page-hero-icon" aria-hidden="true">
            <el-icon :size="20"><Tickets /></el-icon>
          </div>
          <div>
            <h1 class="page-hero-title">执行日志</h1>
            <p class="page-hero-subtitle">所有定时任务的历史运行记录与实时输出</p>
          </div>
        </div>

        <div class="page-hero-stats">
          <div class="stat-chip">
            <span class="stat-chip-value">{{ total }}</span>
            <span class="stat-chip-label">总记录</span>
          </div>
          <div class="stat-chip stat-chip--running" v-if="runningOnPage > 0">
            <span class="stat-chip-dot" aria-hidden="true"></span>
            <span class="stat-chip-value">{{ runningOnPage }}</span>
            <span class="stat-chip-label">运行中</span>
          </div>
          <div class="stat-chip stat-chip--success" v-if="successOnPage > 0">
            <span class="stat-chip-value">{{ successOnPage }}</span>
            <span class="stat-chip-label">本页成功</span>
          </div>
          <div class="stat-chip stat-chip--failed" v-if="failedOnPage > 0">
            <span class="stat-chip-value">{{ failedOnPage }}</span>
            <span class="stat-chip-label">本页失败</span>
          </div>
        </div>
      </div>

      <div class="page-hero-actions">
        <el-button
          class="hero-btn hero-btn--refresh"
          :type="autoRefresh ? 'primary' : 'default'"
          @click="toggleAutoRefresh"
        >
          <el-icon><Refresh /></el-icon>
          <span>{{ autoRefresh ? '停止刷新' : '自动刷新' }}</span>
        </el-button>
        <el-button class="hero-btn" @click="handleClean">
          <el-icon><Delete /></el-icon>
          <span>清理日志</span>
        </el-button>
      </div>
    </header>

    <!-- ======= Filter pill ======= -->
    <div class="filter-bar">
      <div class="filter-search">
        <el-icon class="filter-search-icon"><Search /></el-icon>
        <input
          v-model="keyword"
          class="filter-search-input"
          placeholder="按任务名搜索..."
          @keyup.enter="handleSearch"
        />
        <button v-if="keyword" class="filter-search-clear" @click="(keyword = '', handleSearch())" aria-label="清除搜索">
          <el-icon :size="14"><Close /></el-icon>
        </button>
      </div>
      <div class="filter-status">
        <button
          class="filter-chip"
          :class="{ active: statusFilter === '' }"
          @click="(statusFilter = '', handleSearch())"
        >全部</button>
        <button
          class="filter-chip filter-chip--success"
          :class="{ active: statusFilter === '0' }"
          @click="(statusFilter = '0', handleSearch())"
        >
          <span class="filter-chip-dot"></span>成功
        </button>
        <button
          class="filter-chip filter-chip--failed"
          :class="{ active: statusFilter === '1' }"
          @click="(statusFilter = '1', handleSearch())"
        >
          <span class="filter-chip-dot"></span>失败
        </button>
        <button
          class="filter-chip filter-chip--running"
          :class="{ active: statusFilter === '2' }"
          @click="(statusFilter = '2', handleSearch())"
        >
          <span class="filter-chip-dot"></span>运行中
        </button>
      </div>
    </div>

    <!-- ======= List ======= -->
    <div class="logs-list" v-loading="loading">
      <div class="logs-list-header" v-if="!isMobile && logs.length > 0">
        <el-checkbox
          :model-value="allSelectedOnPage"
          :indeterminate="someSelectedOnPage"
          @update:model-value="toggleSelectAll"
        />
        <span class="logs-list-header-hint">{{ logs.length > 0 ? `本页 ${logs.length} 条` : '' }}</span>
      </div>

      <div v-if="!loading && logs.length === 0" class="logs-empty">
        <el-icon :size="32" class="logs-empty-icon"><Tickets /></el-icon>
        <div class="logs-empty-title">暂无执行日志</div>
        <div class="logs-empty-sub">任务执行后会自动产生记录，也可以刷新一下试试</div>
      </div>

      <div
        v-for="row in logs"
        :key="row.id"
        class="log-row"
        :class="{
          'log-row--running': row.status === 2,
          'log-row--failed': row.status === 1,
          'log-row--selected': isSelected(row.id)
        }"
      >
        <el-checkbox
          class="log-row-check"
          :model-value="isSelected(row.id)"
          @change="toggleSelected(row.id, $event)"
          @click.stop
        />
        <div class="log-row-status">
          <span class="status-indicator" :class="'status-indicator--' + getStatusType(row.status)">
            <span v-if="row.status === 2" class="status-indicator-pulse"></span>
          </span>
        </div>
        <div class="log-row-main" @click="viewDetail(row)">
          <div class="log-row-title-line">
            <span class="log-row-title">{{ row.task_name || `任务#${row.task_id}` }}</span>
            <span class="log-row-id">#{{ row.id }}</span>
            <span class="log-row-status-label" :class="'log-row-status-label--' + getStatusType(row.status)">
              {{ getStatusText(row.status) }}
            </span>
          </div>
          <div class="log-row-meta">
            <span class="log-row-meta-item">
              <el-icon :size="11"><Timer /></el-icon>
              {{ formatDuration(row.duration) }}
            </span>
            <span class="log-row-meta-item">
              <el-icon :size="11"><Calendar /></el-icon>
              {{ formatTime(row.started_at) }}
            </span>
            <span class="log-row-meta-item log-row-meta-item--muted" v-if="row.ended_at">
              → {{ formatTime(row.ended_at) }}
            </span>
          </div>
        </div>
        <div class="log-row-actions">
          <el-tooltip content="查看日志" placement="top">
            <button class="row-icon-btn row-icon-btn--primary" @click="viewDetail(row)" aria-label="查看日志">
              <el-icon :size="15"><View /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip content="日志文件" placement="top">
            <button class="row-icon-btn" @click="browseLogFiles(row)" aria-label="日志文件">
              <el-icon :size="15"><Folder /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip content="删除" placement="top">
            <button class="row-icon-btn row-icon-btn--danger" @click="handleDelete(row)" aria-label="删除">
              <el-icon :size="15"><Delete /></el-icon>
            </button>
          </el-tooltip>
        </div>
      </div>
    </div>

    <div class="pagination-bar" v-if="total > 0">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        :layout="isMobile ? 'prev, pager, next' : 'total, sizes, prev, pager, next'"
        background
        @current-change="loadLogs"
        @size-change="loadLogs"
      />
    </div>

    <!-- ======= Floating batch bar ======= -->
    <transition name="batch-bar">
      <div v-if="selectedIds.length > 0" class="batch-bar" role="toolbar">
        <div class="batch-bar-info">
          <span class="batch-bar-count">{{ selectedIds.length }}</span>
          <span>条已选中</span>
        </div>
        <div class="batch-bar-actions">
          <el-button @click="clearSelection">取消选择</el-button>
          <el-button type="danger" @click="handleBatchDelete">
            <el-icon><Delete /></el-icon>
            <span>批量删除</span>
          </el-button>
        </div>
      </div>
    </transition>

    <!-- ======= Detail dialog ======= -->
    <el-dialog
      v-model="detailVisible"
      width="820px"
      top="6vh"
      align-center
      :fullscreen="dialogFullscreen"
      :show-close="false"
      :close-on-click-modal="false"
      class="log-detail-dialog"
      destroy-on-close
      @close="closeLogSSE"
    >
      <template #header>
        <div class="detail-hero">
          <div class="detail-hero-main">
            <div class="detail-hero-title-row">
              <span
                v-if="detailLog"
                class="status-indicator"
                :class="'status-indicator--' + getStatusType(detailLog.status)"
              >
                <span v-if="detailLog.status === 2" class="status-indicator-pulse"></span>
              </span>
              <span class="detail-hero-title">{{ detailLog?.task_name || '日志详情' }}</span>
              <span v-if="detailLog" class="detail-hero-id">#{{ detailLog.id }}</span>
              <span
                v-if="detailLog"
                class="log-row-status-label"
                :class="'log-row-status-label--' + getStatusType(detailLog.status)"
              >{{ getStatusText(detailLog.status) }}</span>
            </div>
            <div v-if="detailLog" class="detail-hero-meta">
              <span class="detail-hero-meta-item">耗时 {{ formatDuration(detailLog.duration) }}</span>
              <span class="detail-hero-meta-item">开始 {{ formatTime(detailLog.started_at) }}</span>
              <span class="detail-hero-meta-item" v-if="detailLog.ended_at">结束 {{ formatTime(detailLog.ended_at) }}</span>
            </div>
          </div>
          <button class="detail-hero-close" @click="detailVisible = false" aria-label="关闭">
            <el-icon :size="16"><Close /></el-icon>
          </button>
        </div>
      </template>

      <div class="detail-body">
        <pre ref="logContentRef" class="detail-log dd-log-surface">{{ detailContent || '（正在加载日志...）' }}</pre>
        <div class="detail-status-bar">
          <div class="detail-status-group">
            <span class="detail-status-item">{{ detailLineCount }} 行</span>
            <span v-if="detailByteLabel" class="detail-status-item">{{ detailByteLabel }}</span>
          </div>
          <div class="detail-status-group">
            <span v-if="detailLog?.status === 2" class="detail-status-item detail-status-item--live">实时采集中</span>
            <span v-else class="detail-status-item">UTF-8</span>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="detail-footer">
          <el-button @click="copyCurrentLog" :disabled="!detailContent">
            <el-icon><DocumentCopy /></el-icon>
            <span>复制</span>
          </el-button>
          <el-button @click="downloadCurrentLog" :disabled="!detailContent">
            <el-icon><Download /></el-icon>
            <span>下载</span>
          </el-button>
          <el-button type="primary" @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- ======= Log files dialog ======= -->
    <el-dialog
      v-model="showFileBrowser"
      title="日志文件"
      width="680px"
      :fullscreen="dialogFullscreen"
      class="log-files-dialog"
    >
      <el-table :data="logFiles" v-loading="logFilesLoading" max-height="420px" size="small">
        <el-table-column prop="filename" label="文件名" min-width="220" />
        <el-table-column label="大小" width="110">
          <template #default="{ row }">{{ formatFileSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" text size="small" @click="viewLogFile(row)">查看</el-button>
            <el-button type="danger" text size="small" @click="deleteLogFile(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!logFilesLoading && logFiles.length === 0" description="暂无日志文件" />
    </el-dialog>

    <el-dialog v-model="showFileContent" :title="fileContentName" width="820px" :fullscreen="dialogFullscreen">
      <pre class="detail-log dd-log-surface">{{ fileContentData }}</pre>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.logs-page {
  --logs-accent: #22c55e;
  --logs-ai-accent: #6366f1;
  --logs-border-soft: color-mix(in srgb, var(--el-border-color-light) 85%, transparent);
  --logs-surface: var(--el-bg-color);
  --logs-surface-muted: color-mix(in srgb, var(--el-fill-color) 70%, transparent);

  padding: 0;
  font-family: var(--dd-font-ui);
  position: relative;
}

/* =============== Hero =============== */
.page-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  padding: 22px 24px;
  border-radius: 16px;
  margin-bottom: 16px;
  background:
    linear-gradient(135deg,
      color-mix(in srgb, var(--logs-accent) 12%, transparent) 0%,
      color-mix(in srgb, var(--logs-ai-accent) 10%, transparent) 50%,
      transparent 100%),
    var(--logs-surface);
  border: 1px solid var(--logs-border-soft);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

.page-hero-main {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
  flex: 1 1 420px;
}

.page-hero-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-hero-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #22c55e, #16a34a);
  box-shadow: 0 8px 18px -10px rgba(34, 197, 94, 0.55);
  flex-shrink: 0;
}

.page-hero-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  letter-spacing: 0.2px;
  color: var(--el-text-color-primary);
  line-height: 1.2;
}

.page-hero-subtitle {
  font-size: 13px;
  margin: 2px 0 0;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}

.page-hero-stats {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--el-fill-color) 90%, transparent);
  border: 1px solid var(--logs-border-soft);
  font-family: var(--dd-font-ui);
  line-height: 1.2;
}

.stat-chip-value {
  font-size: 16px;
  font-weight: 700;
  font-family: var(--dd-font-mono);
  color: var(--el-text-color-primary);
  letter-spacing: 0.3px;
}

.stat-chip-label {
  font-size: 11.5px;
  color: var(--el-text-color-secondary);
}

.stat-chip-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--el-color-warning);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-warning) 28%, transparent);
  animation: pulse 1.6s ease-in-out infinite;
}

.stat-chip--running {
  background: color-mix(in srgb, var(--el-color-warning) 12%, transparent);
  border-color: color-mix(in srgb, var(--el-color-warning) 35%, transparent);
}

.stat-chip--success {
  background: color-mix(in srgb, var(--logs-accent) 12%, transparent);
  border-color: color-mix(in srgb, var(--logs-accent) 30%, transparent);
  .stat-chip-value { color: color-mix(in srgb, var(--logs-accent) 80%, var(--el-text-color-primary)); }
}

.stat-chip--failed {
  background: color-mix(in srgb, var(--el-color-danger) 12%, transparent);
  border-color: color-mix(in srgb, var(--el-color-danger) 30%, transparent);
  .stat-chip-value { color: var(--el-color-danger); }
}

.page-hero-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.hero-btn {
  border-radius: 10px;
  padding: 0 14px;
  height: 34px;
  font-weight: 500;
}

.hero-btn--refresh {
  box-shadow: 0 4px 10px -6px color-mix(in srgb, var(--el-color-primary) 50%, transparent);
}

/* =============== Filter =============== */
.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  background: var(--logs-surface);
  border: 1px solid var(--logs-border-soft);
}

.filter-search {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1 1 280px;
  min-width: 240px;
  padding: 0 10px;
  height: 34px;
  border-radius: 8px;
  background: var(--logs-surface-muted);
  border: 1px solid transparent;
  transition: border-color 0.15s, background 0.15s;

  &:focus-within {
    border-color: color-mix(in srgb, var(--logs-accent) 55%, transparent);
    background: var(--logs-surface);
  }
}

.filter-search-icon {
  color: var(--el-text-color-placeholder);
  margin-right: 6px;
  flex-shrink: 0;
}

.filter-search-input {
  flex: 1;
  border: none;
  outline: none;
  background: transparent;
  font-size: 13.5px;
  font-family: var(--dd-font-ui);
  color: var(--el-text-color-primary);

  &::placeholder {
    color: var(--el-text-color-placeholder);
  }
}

.filter-search-clear {
  border: none;
  background: transparent;
  color: var(--el-text-color-placeholder);
  cursor: pointer;
  display: inline-flex;
  padding: 2px;
  border-radius: 50%;

  &:hover {
    color: var(--el-text-color-regular);
    background: var(--el-fill-color);
  }
}

.filter-status {
  display: inline-flex;
  gap: 4px;
  padding: 2px;
  background: var(--logs-surface-muted);
  border-radius: 10px;
}

.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0 12px;
  height: 30px;
  border-radius: 8px;
  border: none;
  background: transparent;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--el-text-color-regular);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;

  &:hover:not(.active) {
    background: color-mix(in srgb, var(--el-fill-color) 80%, transparent);
  }

  &.active {
    background: var(--el-bg-color);
    color: var(--el-text-color-primary);
    box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
  }
}

.filter-chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}

.filter-chip--success .filter-chip-dot { background: var(--logs-accent); }
.filter-chip--failed .filter-chip-dot { background: var(--el-color-danger); }
.filter-chip--running .filter-chip-dot { background: var(--el-color-warning); }

/* =============== List =============== */
.logs-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 200px;
}

.logs-list-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 12px;
  min-height: 28px;
}

.logs-list-header-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.logs-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
  border-radius: 14px;
  background: var(--logs-surface-muted);
  border: 1px dashed var(--logs-border-soft);
}

.logs-empty-icon {
  color: var(--el-text-color-placeholder);
  margin-bottom: 10px;
}

.logs-empty-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 4px;
}

.logs-empty-sub {
  font-size: 12.5px;
  color: var(--el-text-color-secondary);
}

.log-row {
  display: grid;
  grid-template-columns: 28px 14px 1fr auto;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 12px;
  background: var(--logs-surface);
  border: 1px solid var(--logs-border-soft);
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s, transform 0.15s;

  &:hover {
    border-color: color-mix(in srgb, var(--logs-accent) 30%, var(--logs-border-soft));
    box-shadow: 0 4px 10px -8px rgba(15, 23, 42, 0.12);
  }

  &--selected {
    border-color: color-mix(in srgb, var(--logs-accent) 55%, transparent);
    background: color-mix(in srgb, var(--logs-accent) 6%, var(--logs-surface));
  }

  &--running {
    border-color: color-mix(in srgb, var(--el-color-warning) 30%, var(--logs-border-soft));
  }

  &--failed {
    border-color: color-mix(in srgb, var(--el-color-danger) 22%, var(--logs-border-soft));
  }
}

.log-row-check {
  justify-self: center;
}

.log-row-status {
  justify-self: center;
}

.status-indicator {
  position: relative;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;

  &--success { background: var(--logs-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--logs-accent) 22%, transparent); }
  &--danger { background: var(--el-color-danger); box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-danger) 22%, transparent); }
  &--warning { background: var(--el-color-warning); box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-warning) 22%, transparent); }
  &--info { background: var(--el-text-color-placeholder); }
}

.status-indicator-pulse {
  position: absolute;
  inset: -3px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--el-color-warning) 50%, transparent);
  animation: orb-ripple 1.6s ease-out infinite;
}

.log-row-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  cursor: pointer;
}

.log-row-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.log-row-title {
  font-size: 14.5px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.log-row-id {
  font-size: 11px;
  font-family: var(--dd-font-mono);
  color: var(--el-text-color-placeholder);
  letter-spacing: 0.3px;
}

.log-row-status-label {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.5px;
  font-family: var(--dd-font-mono);
  border-radius: 999px;

  &--success { background: color-mix(in srgb, var(--logs-accent) 14%, transparent); color: color-mix(in srgb, var(--logs-accent) 80%, var(--el-text-color-primary)); }
  &--danger { background: color-mix(in srgb, var(--el-color-danger) 14%, transparent); color: var(--el-color-danger); }
  &--warning { background: color-mix(in srgb, var(--el-color-warning) 14%, transparent); color: var(--el-color-warning); }
  &--info { background: var(--el-fill-color); color: var(--el-text-color-secondary); }
}

.log-row-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-wrap: wrap;
}

.log-row-meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;

  &--muted {
    color: var(--el-text-color-placeholder);
  }
}

.log-row-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.row-icon-btn {
  width: 30px;
  height: 30px;
  padding: 0;
  border: 1px solid transparent;
  background: transparent;
  border-radius: 8px;
  color: var(--el-text-color-secondary);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s, color 0.15s, border-color 0.15s, transform 0.15s;

  &:hover {
    background: var(--el-fill-color);
    color: var(--el-text-color-primary);
    border-color: var(--logs-border-soft);
  }

  &--primary:hover {
    color: var(--logs-accent);
    border-color: color-mix(in srgb, var(--logs-accent) 40%, transparent);
    background: color-mix(in srgb, var(--logs-accent) 8%, transparent);
  }

  &--danger:hover {
    color: var(--el-color-danger);
    border-color: color-mix(in srgb, var(--el-color-danger) 40%, transparent);
    background: color-mix(in srgb, var(--el-color-danger) 8%, transparent);
  }
}

/* =============== Pagination =============== */
.pagination-bar {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

/* =============== Batch floating bar =============== */
.batch-bar {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 100;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 14px 10px 18px;
  border-radius: 999px;
  background: var(--el-bg-color);
  border: 1px solid var(--logs-border-soft);
  box-shadow: 0 12px 36px -12px rgba(15, 23, 42, 0.25), 0 4px 12px -4px rgba(15, 23, 42, 0.15);
}

.batch-bar-info {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.batch-bar-count {
  font-family: var(--dd-font-mono);
  font-weight: 700;
  font-size: 14px;
  color: var(--logs-accent);
}

.batch-bar-actions {
  display: inline-flex;
  gap: 6px;
}

.batch-bar-enter-active,
.batch-bar-leave-active {
  transition: all 0.22s ease;
}

.batch-bar-enter-from,
.batch-bar-leave-to {
  opacity: 0;
  transform: translate(-50%, 12px);
}

/* =============== Detail dialog =============== */
// 注意：.log-detail-dialog 通过 el-dialog 组件的 class 属性直接透传到 .el-dialog 元素自身，
// 两个 class 在同一个元素上，不是父子关系，所以 flex / max-height 等约束要直接写在 :deep(.log-detail-dialog) 层级。
:deep(.log-detail-dialog) {
  border-radius: 14px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  // 固定窗口尺寸：不随日志内容多少变化，内部日志区用 flex:1 + overflow:auto 在窗口内滚动。
  // 只写 max-height 会让日志少时 dialog 被塌陷成一条，所以 height 和 max-height 同时写，既固定又兜底。
  height: 88vh;
  max-height: 88vh;
  // align-center 模式下 el-overlay-dialog 是 flex 容器，用 margin: auto 让 dialog 垂直+水平居中；
  // 如果写成 margin: 0 auto 上下 margin 会变成 0，dialog 会贴到容器底部。
  margin: auto;

  .el-dialog__header {
    padding: 0;
    margin: 0;
    border-bottom: 1px solid var(--logs-border-soft);
    flex-shrink: 0;
  }

  .el-dialog__body {
    padding: 0;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .el-dialog__footer {
    padding: 12px 18px;
    border-top: 1px solid var(--logs-border-soft);
    flex-shrink: 0;
  }
}

.detail-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  background: linear-gradient(180deg,
    color-mix(in srgb, var(--logs-accent) 6%, transparent) 0%,
    transparent 100%);
}

.detail-hero-main {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.detail-hero-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.detail-hero-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-hero-id {
  font-family: var(--dd-font-mono);
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.detail-hero-meta {
  display: flex;
  gap: 16px;
  font-size: 12.5px;
  color: var(--el-text-color-secondary);
  flex-wrap: wrap;
}

.detail-hero-meta-item {
  font-family: var(--dd-font-ui);
}

.detail-hero-close {
  width: 34px;
  height: 34px;
  padding: 0;
  border: 1px solid transparent;
  background: transparent;
  border-radius: 10px;
  cursor: pointer;
  color: var(--el-text-color-secondary);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative;
  overflow: hidden;
  transition: color 0.25s, border-color 0.25s, transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.25s;

  .el-icon {
    position: relative;
    z-index: 1;
    transition: transform 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  &::before {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: inherit;
    background: linear-gradient(135deg, #ef4444, #dc2626);
    opacity: 0;
    transform: scale(0.55);
    transition: opacity 0.2s ease, transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  &:hover {
    color: #fff;
    border-color: transparent;
    transform: scale(1.06);
    box-shadow: 0 8px 20px -8px rgba(239, 68, 68, 0.55);

    &::before {
      opacity: 1;
      transform: scale(1);
    }

    .el-icon {
      transform: rotate(90deg);
    }
  }

  &:active {
    transform: scale(0.94);
  }

  &:focus-visible {
    outline: 2px solid color-mix(in srgb, #ef4444 60%, transparent);
    outline-offset: 2px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .detail-hero-close {
    transition: none;

    .el-icon,
    &::before {
      transition: none;
    }

    &:hover .el-icon {
      transform: none;
    }
  }
}

.detail-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.detail-log {
  margin: 0;
  flex: 1;
  min-height: 260px;
  overflow: auto;
  padding: 18px 22px;
  font-family: var(--dd-font-mono);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--dd-log-bg-color, #0f172a);
  color: var(--dd-log-text-color, #e2e8f0);
  border-radius: 0;
}

.detail-status-bar {
  display: flex;
  justify-content: space-between;
  padding: 6px 20px;
  font-family: var(--dd-font-mono);
  font-size: 11px;
  color: var(--el-text-color-placeholder);
  border-top: 1px solid var(--logs-border-soft);
  background: color-mix(in srgb, var(--el-fill-color-lighter) 60%, transparent);
}

.detail-status-group {
  display: inline-flex;
  gap: 14px;
}

.detail-status-item--live {
  color: var(--el-color-warning);

  &::before {
    content: '● ';
    animation: pulse 1.6s ease-in-out infinite;
  }
}

.detail-footer {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

/* =============== Animations =============== */
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

@keyframes orb-ripple {
  0% { transform: scale(0.65); opacity: 0.6; }
  100% { transform: scale(1.4); opacity: 0; }
}

@media (prefers-reduced-motion: reduce) {
  .status-indicator-pulse,
  .stat-chip-dot,
  .detail-status-item--live::before { animation: none; }
}

/* =============== Mobile =============== */
@media (max-width: 768px) {
  .page-hero {
    padding: 16px 18px;
  }

  .page-hero-title { font-size: 17px; }
  .page-hero-actions { width: 100%; }
  .page-hero-actions .hero-btn { flex: 1; }

  .log-row {
    grid-template-columns: 20px 12px 1fr;
    gap: 10px;
    padding: 12px;
  }

  .log-row-actions {
    grid-column: 1 / -1;
    border-top: 1px dashed var(--logs-border-soft);
    padding-top: 10px;
    margin-top: 4px;
    justify-content: stretch;
    gap: 6px;

    .row-icon-btn {
      flex: 1;
      border: 1px solid var(--logs-border-soft);
    }
  }

  .batch-bar {
    left: 12px;
    right: 12px;
    transform: none;
    bottom: 16px;
    justify-content: space-between;
    border-radius: 14px;
  }

  .batch-bar-enter-from,
  .batch-bar-leave-to {
    transform: translateY(12px);
  }

  .detail-hero {
    flex-direction: row;
    padding: 14px 16px;
  }

  .detail-hero-title { font-size: 15.5px; }
}
</style>
