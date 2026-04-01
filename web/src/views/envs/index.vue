<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, computed, watch, type CSSProperties } from 'vue'
import { envApi } from '@/api/env'
import { ElMessage, ElMessageBox } from 'element-plus'
import EnvBatchCreateDialog from './components/EnvBatchCreateDialog.vue'
import EnvBatchGroupDialog from './components/EnvBatchGroupDialog.vue'
import EnvBatchRenameDialog from './components/EnvBatchRenameDialog.vue'
import EnvEditDialog from './components/EnvEditDialog.vue'
import EnvImportDialog from './components/EnvImportDialog.vue'
import { useResponsive } from '@/composables/useResponsive'

const envTableDensityStorageKey = 'daidai-env-table-density'
const { isMobile } = useResponsive()

type EnvFormModel = {
  id: number
  name: string
  value: string
  remarks: string
  group: string
}

const envList = ref<any[]>([])
const loading = ref(true)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const currentGroup = ref('')
const groupFilter = ref('')
const groups = ref<string[]>([])
const selectedIds = ref<number[]>([])
const selectedIdSet = computed(() => new Set(selectedIds.value))
const selectedCountInCurrentPage = computed(() =>
  envList.value.filter((item) => selectedIdSet.value.has(item.id)).length
)
const pinnedCountInCurrentPage = computed(() =>
  envList.value.filter((item) => isTopPinned(item)).length
)
const tableDensity = ref<'comfortable' | 'compact'>(
  typeof window !== 'undefined' && window.localStorage.getItem(envTableDensityStorageKey) === 'compact'
    ? 'compact'
    : 'comfortable'
)

const showEditDialog = ref(false)
const editDialogMode = ref<'create' | 'edit'>('create')
const currentEditEnv = ref<EnvFormModel | null>(null)

const showBatchDialog = ref(false)

const showImportDialog = ref(false)

const showExportDialog = ref(false)
const exportFormat = ref('shell')
const exportContent = ref('')
const exportScopeText = computed(() =>
  selectedIds.value.length > 0 ? `已选中的 ${selectedIds.value.length} 项环境变量` : '当前列表中的全部已启用环境变量'
)

const showBatchRenameDialog = ref(false)
const showBatchGroupDialog = ref(false)

const tableRef = ref()
const desktopTableReady = ref(false)
const showDesktopLoadingPlaceholder = computed(
  () => !isMobile.value && (!desktopTableReady.value || (loading.value && envList.value.length === 0))
)
const showDesktopEmptyState = computed(
  () => !isMobile.value && desktopTableReady.value && !loading.value && envList.value.length === 0
)
let sortableInstance: any = null
let sortableLoader: Promise<any> | null = null
let dragPointerY = 0
let dragAutoScrollFrame = 0
let sortableInitFrame = 0
let desktopTableReadyFrame = 0
const groupBadgeStyleCache = new Map<string, CSSProperties>()

function loadSortable() {
  if (!sortableLoader) {
    sortableLoader = import('sortablejs').then((mod) => mod.default)
  }
  return sortableLoader
}

function clearTableSelection() {
  selectedIds.value = []
  tableRef.value?.clearSelection?.()
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

function handleDensityChange(value: 'comfortable' | 'compact') {
  tableDensity.value = value
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(envTableDensityStorageKey, value)
  }
}

function buildStringHue(value: string) {
  let hash = 0
  for (const char of value) {
    hash = ((hash << 5) - hash + char.charCodeAt(0)) | 0
  }
  return Math.abs(hash) % 360
}

function getGroupBadgeStyle(group: string): CSSProperties {
  const cacheKey = group || 'default'
  const cached = groupBadgeStyleCache.get(cacheKey)
  if (cached) {
    return cached
  }

  const style = {
    '--group-hue': String(buildStringHue(cacheKey))
  } as CSSProperties
  groupBadgeStyleCache.set(cacheKey, style)
  return style
}

function updateDragPointer(evt: any) {
  const pointerEvent = evt?.originalEvent || evt
  if (typeof pointerEvent?.clientY === 'number') {
    dragPointerY = pointerEvent.clientY
    return
  }

  const touch = pointerEvent?.touches?.[0] || pointerEvent?.changedTouches?.[0]
  if (touch && typeof touch.clientY === 'number') {
    dragPointerY = touch.clientY
  }
}

function startDragAutoScroll() {
  stopDragAutoScroll()

  const tick = () => {
    const bodyWrapper = (
      isMobile.value
        ? document.querySelector('.env-mobile-scroll')
        : document.querySelector('.env-table .el-table__body-wrapper')
    ) as HTMLElement | null
    const tableThreshold = 72
    const viewportThreshold = 88
    const tableScrollStep = 22
    const pageScrollStep = 18

    if (bodyWrapper) {
      const rect = bodyWrapper.getBoundingClientRect()
      const canScrollTable = bodyWrapper.scrollHeight > bodyWrapper.clientHeight + 4
      if (canScrollTable) {
        if (dragPointerY < rect.top + tableThreshold && bodyWrapper.scrollTop > 0) {
          bodyWrapper.scrollTop -= tableScrollStep
        } else if (
          dragPointerY > rect.bottom - tableThreshold &&
          bodyWrapper.scrollTop + bodyWrapper.clientHeight < bodyWrapper.scrollHeight
        ) {
          bodyWrapper.scrollTop += tableScrollStep
        }
      }
    }

    if (dragPointerY < viewportThreshold) {
      window.scrollBy({ top: -pageScrollStep, behavior: 'auto' })
    } else if (dragPointerY > window.innerHeight - viewportThreshold) {
      window.scrollBy({ top: pageScrollStep, behavior: 'auto' })
    }

    dragAutoScrollFrame = window.requestAnimationFrame(tick)
  }

  dragAutoScrollFrame = window.requestAnimationFrame(tick)
}

function stopDragAutoScroll() {
  if (dragAutoScrollFrame) {
    window.cancelAnimationFrame(dragAutoScrollFrame)
    dragAutoScrollFrame = 0
  }
}

function clearQueuedSortableInit() {
  if (sortableInitFrame) {
    window.cancelAnimationFrame(sortableInitFrame)
    sortableInitFrame = 0
  }
}

function queueSortableInit() {
  if (typeof window === 'undefined') return
  clearQueuedSortableInit()
  sortableInitFrame = window.requestAnimationFrame(() => {
    sortableInitFrame = 0
    void initSortable()
  })
}

function clearDesktopTableReadyQueue() {
  if (desktopTableReadyFrame) {
    window.cancelAnimationFrame(desktopTableReadyFrame)
    desktopTableReadyFrame = 0
  }
}

function queueDesktopTableReady() {
  if (typeof window === 'undefined' || isMobile.value || desktopTableReady.value) return
  clearDesktopTableReadyQueue()
  desktopTableReadyFrame = window.requestAnimationFrame(() => {
    desktopTableReadyFrame = 0
    desktopTableReady.value = true
  })
}

async function loadData() {
  loading.value = true
  selectedIds.value = []
  try {
    const group = groupFilter.value || currentGroup.value || undefined
    const res = await envApi.list({
      keyword: keyword.value || undefined,
      group,
      page: page.value,
      page_size: pageSize.value
    })
    envList.value = res.data || []
    total.value = res.total || 0
  } catch {
    ElMessage.error('加载环境变量失败')
  } finally {
    loading.value = false
  }

  await nextTick()
  queueDesktopTableReady()
  clearTableSelection()
  queueSortableInit()
}

async function loadGroups() {
  try {
    const res = await envApi.groups()
    groups.value = res.data || []
  } catch {
    // ignore
  }
}

onMounted(() => {
  queueDesktopTableReady()
  void loadData()
  void loadGroups()
})

watch(isMobile, () => {
  nextTick(() => {
    queueDesktopTableReady()
    queueSortableInit()
  })
})

onBeforeUnmount(() => {
  stopDragAutoScroll()
  clearQueuedSortableInit()
  clearDesktopTableReadyQueue()
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
})

async function initSortable() {
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
  if (loading.value || envList.value.length < 2) return
  const el = document.querySelector(
    isMobile.value
      ? '.env-mobile-list'
      : '.env-table .el-table__body-wrapper tbody'
  )
  if (!el) return
  try {
    const Sortable = await loadSortable()
    sortableInstance = Sortable.create(el as HTMLElement, {
      animation: 150,
      handle: '.drag-handle',
      ghostClass: 'sortable-ghost',
      chosenClass: 'sortable-chosen',
      dragClass: 'sortable-drag',
      forceFallback: true,
      fallbackOnBody: true,
      scroll: true,
      bubbleScroll: true,
      scrollSensitivity: 100,
      scrollSpeed: 18,
      onStart: (evt: any) => {
        updateDragPointer(evt)
        startDragAutoScroll()
      },
      onMove: (evt: any) => {
        updateDragPointer(evt)
      },
      onEnd: async (evt: any) => {
        stopDragAutoScroll()
        updateDragPointer(evt)

        const { oldIndex, newIndex } = evt
        if (oldIndex === newIndex) return

        const sourceItem = envList.value[oldIndex]
        if (!sourceItem) return

        const movedItem = envList.value.splice(oldIndex, 1)[0]
        envList.value.splice(newIndex, 0, movedItem)
        const nextItem = envList.value[newIndex + 1]
        const sourceSortOrder = Number(sourceItem.sort_order || 0)
        const targetSortOrder = nextItem ? Number(nextItem.sort_order || 0) : sourceSortOrder

        if (targetSortOrder !== sourceSortOrder) {
          ElMessage.warning('置顶区和普通区请分别排序，跨区移动请使用置顶按钮')
          void loadData()
          return
        }

        try {
          await envApi.sort(sourceItem.id, nextItem?.id)
        } catch (err: any) {
          ElMessage.error(err?.response?.data?.error || err?.message || '排序失败')
          void loadData()
        }
      }
    })
  } catch {
    ElMessage.error('拖拽排序组件加载失败')
  }
}

function handleSearch() {
  page.value = 1
  void loadData()
}

function handleGroupSelect() {
  page.value = 1
  void loadData()
}

function handleGroupFilter(group: string) {
  currentGroup.value = currentGroup.value === group ? '' : group
  groupFilter.value = ''
  page.value = 1
  void loadData()
}

function handlePageChange(newPage: number) {
  page.value = newPage
  void loadData()
}

function handlePageSizeChange(newSize: number) {
  pageSize.value = newSize
  page.value = 1
  void loadData()
}

function openCreate() {
  editDialogMode.value = 'create'
  currentEditEnv.value = { id: 0, name: '', value: '', remarks: '', group: '' }
  showEditDialog.value = true
}

function openDuplicate(row: any) {
  editDialogMode.value = 'create'
  currentEditEnv.value = {
    id: 0,
    name: row.name || '',
    value: '',
    remarks: row.remarks || '',
    group: row.group || ''
  }
  showEditDialog.value = true
}

function openEdit(row: any) {
  editDialogMode.value = 'edit'
  currentEditEnv.value = {
    id: row.id,
    name: row.name || '',
    value: row.value || '',
    remarks: row.remarks || '',
    group: row.group || ''
  }
  showEditDialog.value = true
}

async function handleSave(form: EnvFormModel) {
  try {
    if (editDialogMode.value === 'create') {
      await envApi.create(form)
      ElMessage.success('创建成功')
    } else {
      await envApi.update(form.id, {
        name: form.name,
        value: form.value,
        remarks: form.remarks,
        group: form.group
      })
      ElMessage.success('更新成功')
    }
    showEditDialog.value = false
    void loadData()
    void loadGroups()
  } catch {
    ElMessage.error(editDialogMode.value === 'create' ? '创建失败' : '更新失败')
  }
}

function isTopPinned(row: any) {
  return Number(row.sort_order || 0) > 0
}

function getRowClassName({ row }: { row: any }) {
  return isTopPinned(row) ? 'env-row-pinned' : ''
}

async function handleToggleTop(row: any) {
  try {
    if (isTopPinned(row)) {
      await envApi.cancelTop(row.id)
      ElMessage.success('已取消置顶')
    } else {
      await envApi.moveToTop(row.id)
      ElMessage.success('已置顶')
    }
    void loadData()
  } catch {
    ElMessage.error('操作失败')
  }
}

async function handleBatchCreate(items: { name: string; value: string }[]) {
  try {
    await envApi.create(items as any)
    ElMessage.success(`批量创建 ${items.length} 个变量成功`)
    showBatchDialog.value = false
    void loadData()
    void loadGroups()
  } catch {
    ElMessage.error('批量创建失败')
  }
}

async function handleDelete(id: number) {
  try {
    await ElMessageBox.confirm('确定要删除该环境变量吗？', '确认删除', { type: 'warning' })
    await envApi.delete(id)
    ElMessage.success('删除成功')
    void loadData()
  } catch {
    // cancelled
  }
}

async function handleToggle(row: any) {
  try {
    const enabling = !row.enabled
    await ElMessageBox.confirm(
      enabling
        ? `确认启用环境变量 ${row.name} 吗？`
        : `确认禁用环境变量 ${row.name} 吗？禁用后脚本将无法读取该变量。`,
      enabling ? '启用确认' : '禁用确认',
      { type: enabling ? 'info' : 'warning' }
    )
    if (row.enabled) {
      await envApi.disable(row.id)
    } else {
      await envApi.enable(row.id)
    }
    ElMessage.success(row.enabled ? '已禁用' : '已启用')
    void loadData()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error('操作失败')
  }
}

async function handleBatchDelete() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(`确定要删除选中的 ${selectedIds.value.length} 个环境变量吗？`, '批量删除', { type: 'warning' })
    await envApi.batchDelete(selectedIds.value)
    ElMessage.success('批量删除成功')
    clearTableSelection()
    void loadData()
  } catch {
    // cancelled
  }
}

async function handleBatchGroup() {
  if (selectedIds.value.length === 0) return
  showBatchGroupDialog.value = true
}

function handleBatchRename() {
  if (selectedIds.value.length === 0) return
  showBatchRenameDialog.value = true
}

async function confirmBatchRename(payload: { name: string }) {
  try {
    const res = await envApi.batchRename(selectedIds.value, payload.name)
    ElMessage.success(res.message || '批量改名成功')
    showBatchRenameDialog.value = false
    clearTableSelection()
    void loadData()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || err?.message || '批量改名失败')
  }
}

async function confirmBatchGroup(group: string) {
  try {
    await envApi.batchSetGroup(selectedIds.value, group)
    ElMessage.success('批量分组成功')
    showBatchGroupDialog.value = false
    clearTableSelection()
    void loadData()
    void loadGroups()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || err?.message || '批量分组失败')
  }
}

async function handleBatchEnable() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(`确认启用选中的 ${selectedIds.value.length} 个环境变量吗？`, '批量启用', { type: 'info' })
    await envApi.batchEnable(selectedIds.value)
    ElMessage.success('批量启用成功')
    void loadData()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error('批量启用失败')
  }
}

async function handleBatchDisable() {
  if (selectedIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(`确认禁用选中的 ${selectedIds.value.length} 个环境变量吗？`, '批量禁用', { type: 'warning' })
    await envApi.batchDisable(selectedIds.value)
    ElMessage.success('批量禁用成功')
    void loadData()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error('批量禁用失败')
  }
}

function handleSelectionChange(rows: any[]) {
  selectedIds.value = rows.map(r => r.id)
}

async function handleImport(payload: { envs: any[]; mode: string }) {
  try {
    const res = await envApi.import(payload.envs, payload.mode)
    ElMessage.success(res.message)
    showImportDialog.value = false
    void loadData()
    void loadGroups()
  } catch {
    ElMessage.error('导入失败')
  }
}

async function handleExportAll() {
  try {
    const exportIds = selectedIds.value.length > 0 ? [...selectedIds.value] : undefined
    const res = await envApi.exportAll(exportIds)
    const json = JSON.stringify(res.data, null, 2)
    const blob = new Blob([json], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = exportIds?.length ? `env_vars_selected_${exportIds.length}.json` : 'env_vars.json'
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.error('导出失败')
  }
}

async function handleExportFiles() {
  showExportDialog.value = true
  try {
    const exportIds = selectedIds.value.length > 0 ? [...selectedIds.value] : undefined
    const enabledOnly = exportIds == null
    const res = await envApi.exportFiles(exportFormat.value, enabledOnly, exportIds)
    exportContent.value = res.data[exportFormat.value] || ''
  } catch {
    ElMessage.error('导出失败')
  }
}

async function refreshExport() {
  try {
    const exportIds = selectedIds.value.length > 0 ? [...selectedIds.value] : undefined
    const enabledOnly = exportIds == null
    const res = await envApi.exportFiles(exportFormat.value, enabledOnly, exportIds)
    exportContent.value = res.data[exportFormat.value] || ''
  } catch {
    // ignore
  }
}

function copyExport() {
  navigator.clipboard.writeText(exportContent.value)
  ElMessage.success('已复制到剪贴板')
}

function copyEnvValue(value: string) {
  navigator.clipboard.writeText(value)
  ElMessage.success('已复制到剪贴板')
}

function formatDateTime(t: string | null) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}  ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<template>
  <div class="envs-page">
    <div class="page-title-bar">
      <h2>环境变量</h2>
      <span class="page-subtitle">管理运行时使用的全局环境变量配置</span>
    </div>
    <div class="page-header">
      <div class="header-left">
        <el-input
          v-model="keyword"
          placeholder="搜索变量名或备注"
          clearable
          style="width: 240px"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="groupFilter" placeholder="分组筛选" clearable style="width: 150px" @change="handleGroupSelect">
          <el-option v-for="g in groups" :key="g" :label="g" :value="g" />
        </el-select>
      </div>
      <div class="header-right">
        <div class="table-density-switch">
          <span class="density-label">排版</span>
          <el-radio-group v-model="tableDensity" size="small" @change="handleDensityChange">
            <el-radio-button value="comfortable">舒展</el-radio-button>
            <el-radio-button value="compact">紧凑</el-radio-button>
          </el-radio-group>
        </div>
        <div class="env-toolbar-actions env-toolbar-actions--entry">
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>新建
          </el-button>
          <el-button @click="showBatchDialog = true">
            <el-icon><DocumentAdd /></el-icon>批量添加
          </el-button>
        </div>
        <div class="env-toolbar-actions env-toolbar-actions--batch">
          <el-button @click="handleBatchRename" :disabled="selectedIds.length === 0">
            <el-icon><Edit /></el-icon>批量改名
          </el-button>
          <el-button @click="handleBatchEnable" :disabled="selectedIds.length === 0">
            <el-icon><Check /></el-icon>批量启用
          </el-button>
          <el-button @click="handleBatchDisable" :disabled="selectedIds.length === 0">
            <el-icon><Close /></el-icon>批量禁用
          </el-button>
          <el-button @click="handleBatchDelete" :disabled="selectedIds.length === 0">
            <el-icon><Delete /></el-icon>批量删除
          </el-button>
          <el-button @click="handleBatchGroup" :disabled="selectedIds.length === 0">
            <el-icon><FolderAdd /></el-icon>批量分组
          </el-button>
        </div>
        <div class="env-toolbar-actions env-toolbar-actions--io">
          <el-dropdown class="env-toolbar-actions__dropdown" trigger="click">
            <el-button>
              <el-icon><Download /></el-icon>导出
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="handleExportAll">导出 JSON</el-dropdown-item>
                <el-dropdown-item @click="exportFormat = 'shell'; handleExportFiles()">导出 Shell</el-dropdown-item>
                <el-dropdown-item @click="exportFormat = 'js'; handleExportFiles()">导出 JS</el-dropdown-item>
                <el-dropdown-item @click="exportFormat = 'python'; handleExportFiles()">导出 Python</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button @click="showImportDialog = true">
            <el-icon><Upload /></el-icon>导入
          </el-button>
        </div>
      </div>
    </div>

    <div class="env-hint-bar">
      <div class="env-hint-chip">
        <el-icon><Rank /></el-icon>
        拖拽只调整同一区域顺序，置顶请使用图钉按钮
      </div>
      <div class="env-hint-chip" v-if="pinnedCountInCurrentPage > 0">
        <el-icon><Top /></el-icon>
        当前页有 {{ pinnedCountInCurrentPage }} 项置顶
      </div>
    </div>

    <div v-if="isMobile" class="env-mobile-scroll">
      <div class="dd-mobile-list env-mobile-list">
        <div
          v-for="row in envList"
          :key="row.id"
          class="dd-mobile-card env-card"
          :class="{ 'env-card--pinned': isTopPinned(row) }"
        >
          <div class="dd-mobile-card__header">
            <div class="dd-mobile-card__title-wrap">
              <div class="env-card__title-row">
                <div class="dd-mobile-card__selection">
                  <el-checkbox :model-value="isSelected(row.id)" @change="toggleSelected(row.id, $event)" />
                  <div class="env-name-wrap">
                    <span class="env-name">{{ row.name }}</span>
                    <span v-if="isTopPinned(row)" class="pinned-chip">
                      <el-icon><Top /></el-icon>
                      置顶
                    </span>
                  </div>
                </div>
                <div class="env-card__tools">
                  <el-icon class="drag-handle"><Rank /></el-icon>
                  <span v-if="row.group" class="group-pill" :style="getGroupBadgeStyle(row.group)">
                    <span class="group-dot" />
                    {{ row.group }}
                  </span>
                  <span v-else class="env-empty-text">未分组</span>
                </div>
              </div>
            </div>
          </div>

          <div class="dd-mobile-card__body">
            <div class="dd-mobile-card__grid">
              <div class="dd-mobile-card__field dd-mobile-card__field--full">
                <span class="dd-mobile-card__label">值</span>
                <div class="dd-mobile-card__value env-value-cell">
                  <span class="env-value-text">{{ row.value || '-' }}</span>
                  <el-button v-if="row.value" size="small" link @click.stop="copyEnvValue(row.value)">
                    <el-icon :size="14"><CopyDocument /></el-icon>
                  </el-button>
                </div>
              </div>
              <div class="dd-mobile-card__field dd-mobile-card__field--full">
                <span class="dd-mobile-card__label">备注</span>
                <span class="dd-mobile-card__value env-remarks-text">{{ row.remarks || '-' }}</span>
              </div>
              <div class="dd-mobile-card__field">
                <span class="dd-mobile-card__label">状态</span>
                <div class="dd-mobile-card__value env-status-inline">
                  <el-switch :model-value="row.enabled" size="small" @change="handleToggle(row)" />
                  <span class="env-status-text" :class="{ enabled: row.enabled }">
                    {{ row.enabled ? '启用' : '禁用' }}
                  </span>
                </div>
              </div>
              <div class="dd-mobile-card__field">
                <span class="dd-mobile-card__label">更新时间</span>
                <span class="dd-mobile-card__value time-text">{{ formatDateTime(row.updated_at) }}</span>
              </div>
            </div>

            <div class="dd-mobile-card__actions env-card__actions">
              <el-button size="small" type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button size="small" @click="openDuplicate(row)">复制</el-button>
              <el-button
                size="small"
                :type="isTopPinned(row) ? 'info' : 'warning'"
                @click="handleToggleTop(row)"
              >
                {{ isTopPinned(row) ? '取消置顶' : '置顶' }}
              </el-button>
              <el-button size="small" type="danger" plain @click="handleDelete(row.id)">删除</el-button>
            </div>
          </div>
        </div>

        <el-empty v-if="!loading && envList.length === 0" description="暂无环境变量" />
      </div>
    </div>

    <div v-else-if="showDesktopLoadingPlaceholder" class="env-desktop-state env-desktop-state--loading" aria-hidden="true">
      <div class="env-skeleton env-skeleton--title" />
      <div class="env-skeleton env-skeleton--toolbar" />
      <div v-for="n in 6" :key="`env-skeleton-${n}`" class="env-skeleton env-skeleton--row" />
    </div>

    <div v-else-if="showDesktopEmptyState" class="env-desktop-state">
      <el-empty description="暂无环境变量">
        <template #description>
          <div class="env-empty-copy">
            <strong>暂无环境变量</strong>
            <span>可以直接新建变量，或导入已有的 JSON 配置。</span>
          </div>
        </template>
        <div class="env-empty-actions">
          <el-button type="primary" @click="openCreate">新建环境变量</el-button>
          <el-button @click="showImportDialog = true">导入 JSON</el-button>
        </div>
      </el-empty>
    </div>

    <el-table
      v-else
      ref="tableRef"
      :data="envList"
      v-loading="loading"
      @selection-change="handleSelectionChange"
      :row-class-name="getRowClassName"
      stripe
      :class="['env-table', `env-table--${tableDensity}`]"
      row-key="id"
    >
      <el-table-column type="selection" width="44" />
      <el-table-column label="#" width="58" align="center">
        <template #default="{ $index }">
          <span class="row-index">{{ (page - 1) * pageSize + $index + 1 }}</span>
        </template>
      </el-table-column>
      <el-table-column width="44" align="center">
        <template #default>
          <el-icon class="drag-handle"><Rank /></el-icon>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="变量名" min-width="188">
        <template #header>
          <div class="table-header-label is-primary">
            <span>变量名</span>
            <small>唯一键名</small>
          </div>
        </template>
        <template #default="{ row }">
          <div class="env-name-wrap">
            <span class="env-name">{{ row.name }}</span>
            <span v-if="isTopPinned(row)" class="pinned-chip">
              <el-icon><Top /></el-icon>
              置顶
            </span>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="value" label="值" min-width="280">
        <template #header>
          <div class="table-header-label is-primary">
            <span>值</span>
            <small>主信息区域</small>
          </div>
        </template>
        <template #default="{ row }">
          <div class="env-value-cell">
            <span class="env-value-text" :title="row.value || ''">{{ row.value || '-' }}</span>
            <el-tooltip v-if="row.value" content="复制" placement="top">
              <el-button class="env-copy-btn" size="small" link @click.stop="copyEnvValue(row.value)">
                <el-icon :size="14"><CopyDocument /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="remarks" label="备注" min-width="180">
        <template #header>
          <div class="table-header-label">
            <span>备注</span>
            <small>用途说明</small>
          </div>
        </template>
        <template #default="{ row }">
          <span class="env-remarks-text" :title="row.remarks || ''">{{ row.remarks || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="group" label="分组" width="120" align="center">
        <template #header>
          <div class="table-header-label is-aux">
            <span>分组</span>
            <small>归类标签</small>
          </div>
        </template>
        <template #default="{ row }">
          <span v-if="row.group" class="group-pill" :style="getGroupBadgeStyle(row.group)">
            <span class="group-dot" />
            {{ row.group }}
          </span>
          <span v-else class="env-empty-text">未分组</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="92" align="center">
        <template #header>
          <div class="table-header-label is-aux">
            <span>状态</span>
            <small>启用开关</small>
          </div>
        </template>
        <template #default="{ row }">
          <div class="env-status-cell">
            <el-switch
              :model-value="row.enabled"
              size="small"
              @change="handleToggle(row)"
            />
            <span class="env-status-text" :class="{ enabled: row.enabled }">
              {{ row.enabled ? '启用' : '禁用' }}
            </span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="168" align="center">
        <template #header>
          <div class="table-header-label is-aux">
            <span>更新时间</span>
            <small>最近修改</small>
          </div>
        </template>
        <template #default="{ row }">
          <span class="time-text">{{ formatDateTime(row.updated_at) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="176" align="center">
        <template #default="{ row }">
          <div class="action-group">
            <el-tooltip content="编辑" placement="top">
              <el-button size="small" type="primary" plain circle @click="openEdit(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="复制同名变量" placement="top">
              <el-button size="small" plain circle @click="openDuplicate(row)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="isTopPinned(row) ? '取消置顶' : '置顶'" placement="top">
              <el-button
                size="small"
                :type="isTopPinned(row) ? 'info' : 'warning'"
                :class="{ 'top-action-active': isTopPinned(row) }"
                plain
                circle
                @click="handleToggleTop(row)"
              >
                <el-icon><Top /></el-icon>
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

    <div v-if="total > pageSize || selectedCountInCurrentPage > 0" class="table-footer-bar">
      <div class="selection-summary" :class="{ active: selectedCountInCurrentPage > 0 }">
        <span v-if="selectedCountInCurrentPage > 0">
          已选择当前页 {{ selectedCountInCurrentPage }} 项，批量操作仅作用于当前页勾选的数据。
        </span>
        <span v-else>
          批量操作仅作用于当前页勾选的数据。
        </span>
        <el-button v-if="selectedCountInCurrentPage > 0" text type="primary" @click="clearTableSelection">
          清空选择
        </el-button>
      </div>

      <div class="pagination-container" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @current-change="handlePageChange"
          @size-change="handlePageSizeChange"
        />
      </div>
    </div>

    <el-dialog v-model="showExportDialog" title="导出环境变量" width="600px" :fullscreen="isMobile">
      <div class="export-format-switch">
        <el-radio-group v-model="exportFormat" @change="refreshExport">
          <el-radio-button value="shell">Shell</el-radio-button>
          <el-radio-button value="js">JavaScript</el-radio-button>
          <el-radio-button value="python">Python</el-radio-button>
        </el-radio-group>
        <el-button size="small" @click="copyExport">
          <el-icon><CopyDocument /></el-icon>复制
        </el-button>
      </div>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
        :title="`导出范围：${exportScopeText}`"
      />
      <pre class="export-preview">{{ exportContent }}</pre>
    </el-dialog>

    <EnvEditDialog
      v-model="showEditDialog"
      :mode="editDialogMode"
      :initial-data="currentEditEnv"
      :groups="groups"
      @save="handleSave"
    />

    <EnvImportDialog
      v-model="showImportDialog"
      @import="handleImport"
    />

    <EnvBatchCreateDialog
      v-model="showBatchDialog"
      @create="handleBatchCreate"
    />

    <EnvBatchRenameDialog
      v-model="showBatchRenameDialog"
      @confirm="confirmBatchRename"
    />

    <EnvBatchGroupDialog
      v-model="showBatchGroupDialog"
      :groups="groups"
      @confirm="confirmBatchGroup"
    />
  </div>
</template>

<style scoped lang="scss">
.envs-page {
  padding: 0;
}

.page-title-bar {
  margin-bottom: 16px;

  h2 { margin: 0; font-size: 20px; font-weight: 700; color: var(--el-text-color-primary); }

  .page-subtitle {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    display: block;
    margin-top: 2px;
  }
}

.page-header {
  display: flex;
  justify-content: flex-start;
  align-items: stretch;
  flex-direction: column;
  margin-bottom: 16px;
  gap: 12px;

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
    min-width: 0;
  }

  .header-right {
    display: flex;
    align-items: flex-start;
    align-content: flex-start;
    gap: 10px 12px;
    flex-wrap: wrap;
    justify-content: flex-start;
    min-width: 0;
  }
}

.env-toolbar-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;

  > * {
    min-width: 0;
  }
}

.env-toolbar-actions__dropdown {
  min-width: 0;
}

.table-density-switch {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--el-fill-color-light) 88%, white);
  align-self: flex-start;
}

.density-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--el-text-color-secondary);
}

.env-hint-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 14px;
}

.env-hint-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  background: color-mix(in srgb, var(--el-color-primary) 7%, var(--el-bg-color));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--el-color-primary) 14%, transparent);
}

.env-mobile-scroll {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.env-desktop-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 360px;
  padding: 28px 24px;
  border: 1px solid color-mix(in srgb, var(--el-border-color) 70%, transparent);
  border-radius: 18px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--el-fill-color-lighter) 62%, white) 0%, var(--el-bg-color) 100%);
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.05);
}

.env-desktop-state--loading {
  display: grid;
  align-content: start;
  justify-items: stretch;
  gap: 16px;
}

.env-skeleton {
  position: relative;
  overflow: hidden;
  border-radius: 14px;
  background: color-mix(in srgb, var(--el-fill-color) 82%, white);
}

.env-skeleton::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.6) 48%,
    transparent 100%
  );
  animation: env-skeleton-shimmer 1.35s ease-in-out infinite;
}

.env-skeleton--title {
  width: min(320px, 32%);
  height: 22px;
}

.env-skeleton--toolbar {
  width: min(460px, 48%);
  height: 40px;
  margin-bottom: 8px;
}

.env-skeleton--row {
  width: 100%;
  height: 58px;
}

.env-empty-copy {
  display: inline-flex;
  flex-direction: column;
  gap: 6px;
  color: var(--el-text-color-secondary);
}

.env-empty-copy strong {
  font-size: 16px;
  color: var(--el-text-color-primary);
}

.env-empty-copy span {
  font-size: 13px;
}

.env-empty-actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.env-card__title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.env-card__tools {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.env-card__actions > * {
  flex: 1 1 calc(33.33% - 6px);
}

.env-card--pinned {
  border-color: rgba(245, 166, 35, 0.28);
  box-shadow:
    inset 4px 0 0 #f5a623,
    0 10px 28px rgba(15, 23, 42, 0.07);
}

.action-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.table-header-label {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  line-height: 1.1;
}

.table-header-label span {
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.table-header-label small {
  font-size: 11px;
  font-weight: 500;
  color: var(--el-text-color-placeholder);
}

.table-header-label.is-primary span {
  color: var(--el-color-primary);
}

.table-header-label.is-aux span {
  color: var(--el-text-color-regular);
}

.env-name-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.env-name {
  min-width: 0;
  flex: 1;
  font-family: var(--dd-font-mono);
  font-size: 13px;
  color: var(--el-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pinned-chip {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: #8a4b00;
  background: linear-gradient(135deg, #fff1bf 0%, #ffd66b 100%);
  box-shadow: inset 0 0 0 1px rgba(196, 118, 0, 0.18);
}

.env-value-text,
.env-remarks-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.env-value-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.env-value-text {
  font-family: var(--dd-font-mono);
  color: var(--el-text-color-primary);
}

.env-copy-btn {
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.2s;
  color: var(--el-text-color-secondary);
  padding: 2px;
}

:deep(.el-table__row:hover) .env-copy-btn {
  opacity: 1;
}

.env-remarks-text {
  color: var(--el-text-color-regular);
}

.env-empty-text {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.group-pill {
  --group-hue: 210;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: hsl(var(--group-hue) 55% 32%);
  background: hsl(var(--group-hue) 85% 96%);
  box-shadow: inset 0 0 0 1px hsl(var(--group-hue) 72% 78% / 0.7);
}

.group-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: hsl(var(--group-hue) 72% 48%);
  flex-shrink: 0;
}

.row-index {
  font-family: var(--dd-font-mono);
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.env-status-cell {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.env-status-inline {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.env-status-text {
  font-size: 11px;
  line-height: 1;
  color: var(--el-text-color-placeholder);
}

.env-status-text.enabled {
  color: var(--el-color-success);
}

.time-text {
  font-family: var(--dd-font-mono);
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.drag-handle {
  cursor: grab;
  color: var(--el-text-color-placeholder);
  font-size: 16px;
  &:hover { color: var(--el-color-primary); }
  &:active { cursor: grabbing; }
}

.top-action-active {
  box-shadow: 0 0 0 1px rgba(245, 166, 35, 0.2);
}

.sortable-ghost {
  opacity: 0.4;
  background: var(--el-color-primary-light-9) !important;
}

.sortable-chosen {
  background: color-mix(in srgb, var(--el-color-primary) 10%, var(--el-bg-color)) !important;
}

.sortable-drag {
  opacity: 0.96 !important;
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.16);
}

.env-table {
  width: 100%;
}

:deep(.env-table .el-table__cell) {
  padding-top: 12px;
  padding-bottom: 12px;
}

:deep(.env-table th.el-table__cell) {
  white-space: nowrap;
  background: color-mix(in srgb, var(--el-fill-color-lighter) 70%, white);
}

:deep(.env-table--compact .el-table__cell) {
  padding-top: 8px;
  padding-bottom: 8px;
}

:deep(.env-table--compact .env-value-text),
:deep(.env-table--compact .env-remarks-text),
:deep(.env-table--compact .env-name) {
  font-size: 12px;
}

:deep(.env-table--compact .time-text) {
  font-size: 11px;
}

:deep(.env-table--compact .pinned-chip),
:deep(.env-table--compact .group-pill) {
  padding-top: 2px;
  padding-bottom: 2px;
  font-size: 11px;
}

.table-footer-bar {
  margin-top: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  position: sticky;
  bottom: 0;
  z-index: 12;
  padding: 14px 16px 8px;
  border-radius: 14px 14px 0 0;
  background: color-mix(in srgb, var(--el-bg-color) 96%, white);
  box-shadow: 0 -8px 24px rgba(15, 23, 42, 0.08);
  border-top: 1px solid color-mix(in srgb, var(--el-border-color) 70%, transparent);
}

.selection-summary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 10px 14px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--el-fill-color-light) 85%, white);
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.selection-summary.active {
  background: color-mix(in srgb, var(--el-color-primary) 10%, var(--el-bg-color));
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-left: auto;
}

.export-format-switch {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.export-preview {
  background: var(--el-bg-color-page);
  border-radius: 6px;
  padding: 16px;
  font-family: var(--dd-font-mono);
  font-size: 13px;
  line-height: 1.6;
  max-height: 400px;
  overflow-y: auto;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

:deep(.env-row-pinned > td) {
  background:
    linear-gradient(90deg, rgba(255, 214, 107, 0.22) 0, rgba(255, 214, 107, 0.08) 32px, transparent 220px),
    var(--el-table-tr-bg-color);
}

:deep(.env-row-pinned > td:first-child) {
  box-shadow: inset 4px 0 0 #f5a623;
}

@keyframes env-skeleton-shimmer {
  100% {
    transform: translateX(100%);
  }
}

@media (max-width: 768px) {
  .page-header {
    .header-left {
      width: 100%;
    }

    .header-right {
      width: 100%;
      flex-direction: column;
      align-items: stretch;
      justify-content: flex-start;
      flex-wrap: nowrap;
    }
  }

  .header-left {
    :deep(.el-input),
    :deep(.el-select) {
      width: 100% !important;
    }
  }

  .table-density-switch {
    width: 100%;
    justify-content: space-between;
  }

  .env-toolbar-actions {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;

    > * {
      width: 100%;
    }

    :deep(.el-button) {
      width: 100%;
      margin-left: 0;
    }
  }

  .env-toolbar-actions__dropdown {
    width: 100%;

    :deep(.el-button) {
      width: 100%;
    }
  }

  .env-card__title-row {
    flex-direction: column;
  }

  .env-card__tools {
    width: 100%;
    justify-content: space-between;
  }

  .env-card__actions > * {
    flex: 1 1 calc(50% - 4px);
  }

  .table-footer-bar {
    align-items: stretch;
  }

  .pagination-container {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
