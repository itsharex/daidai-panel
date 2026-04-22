<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { subscriptionApi } from '@/api/subscription'
import { sshKeyApi } from '@/api/notification'
import { ElMessage, ElMessageBox } from 'element-plus'
import { openAuthorizedEventStream, type EventStreamConnection } from '@/utils/sse'
import { useResponsive } from '@/composables/useResponsive'

const subList = ref<any[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const selectedIds = ref<number[]>([])
const selectedIdSet = computed(() => new Set(selectedIds.value))
const { isMobile, dialogFullscreen } = useResponsive()

const showEditDialog = ref(false)
const showLogDialog = ref(false)
const isCreate = ref(true)
const qlCommand = ref('')

const GITHUB_MIRROR_STORAGE_KEY = 'subscription.github_mirror'
const DEFAULT_GITHUB_MIRROR = 'https://gh-proxy.com/'
const githubMirror = ref(localStorage.getItem(GITHUB_MIRROR_STORAGE_KEY) || DEFAULT_GITHUB_MIRROR)

function normalizeMirror(u: string): string {
  const t = u.trim()
  if (!t) return ''
  return t.endsWith('/') ? t : t + '/'
}

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
  hook_script: '',
  auto_add_task: false,
  auto_del_task: false,
  save_dir: '',
  sub_path: '',
  ssh_key_id: null as number | null,
  alias: '',
  force_overwrite: true
})

const sshKeys = ref<any[]>([])
const logList = ref<any[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logSubId = ref(0)
const logLoading = ref(false)

const showLogDetail = ref(false)
const logDetailContent = ref('')

const showPullLog = ref(false)
const pullLogLines = ref<string[]>([])
const pullRunning = ref(false)
const pullingSubId = ref<number | null>(null)
let pullEventSource: EventStreamConnection | null = null
const pullLogRef = ref<HTMLElement>()
let pullBuffer: string[] = []
let pullFlushRaf = 0

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
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '加载订阅列表失败')
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
  if (pullFlushRaf) {
    cancelAnimationFrame(pullFlushRaf)
    pullFlushRaf = 0
  }
})

function handleSearch() {
  page.value = 1
  loadData()
}

function openCreate() {
  isCreate.value = true
  qlCommand.value = ''
  editForm.value = {
    id: 0, name: '', type: 'git-repo', url: '', branch: '', schedule: '',
    whitelist: '', blacklist: '', depend_on: '', hook_script: '', auto_add_task: false,
    auto_del_task: false, save_dir: '', sub_path: '', ssh_key_id: null, alias: '',
    force_overwrite: true
  }
  showEditDialog.value = true
}

function addGithubMirror(url: string): string {
  if (!url) return url
  const mirror = normalizeMirror(githubMirror.value)
  if (!mirror) return url
  const githubPattern = /^https?:\/\/github\.com\//
  // 已经包含镜像（任何协议）就不再重复包裹
  const mirrorHost = mirror.replace(/^https?:\/\//, '').replace(/\/$/, '')
  if (mirrorHost && url.includes(mirrorHost)) return url
  if (githubPattern.test(url)) {
    return url.replace(/^https?:\/\/github\.com\//, mirror + 'https://github.com/')
  }
  return url
}

async function handleConfigMirror() {
  try {
    const { value } = await ElMessageBox.prompt(
      '请输入 GitHub 镜像加速地址（留空则使用默认值）',
      'GitHub 镜像配置',
      {
        inputValue: githubMirror.value,
        inputPlaceholder: DEFAULT_GITHUB_MIRROR,
        inputPattern: /^(https?:\/\/.+)?$/,
        inputErrorMessage: '请输入有效的 URL（需以 http:// 或 https:// 开头）',
        confirmButtonText: '保存',
        cancelButtonText: '取消',
      }
    )
    const mirror = (value || '').trim() || DEFAULT_GITHUB_MIRROR
    githubMirror.value = normalizeMirror(mirror)
    localStorage.setItem(GITHUB_MIRROR_STORAGE_KEY, githubMirror.value)
    ElMessage.success('镜像地址已保存')
  } catch { /* cancelled */ }
}

function deriveSubscriptionSaveDir(url: string): string {
  const trimmed = url.trim().replace(/\/+$/, '').replace(/\.git$/i, '')
  if (!trimmed) return ''
  const parts = trimmed.split('/').filter(Boolean)
  if (parts.length >= 2) {
    const owner = parts[parts.length - 2]
    const repo = parts[parts.length - 1]
    if (owner && repo) {
      return `${owner}_${repo}`
    }
  }
  return parts[parts.length - 1] || ''
}

function normalizeRecognizedHookScript(raw: string): string {
  return raw.replace(/(?:\$\{?QL_DIR\}?|%QL_DIR%)[/\\]data[/\\](?:repo|scripts)[/\\][^/\\"'\s;]+/g, '$SUB_DIR').trim()
}

function parseQLCommand() {
  const cmd = qlCommand.value.trim()
  if (!cmd) return

  const lines = cmd
    .split(/\r?\n/)
    .map(line => line.trim())
    .filter(Boolean)
  const qlLine = lines.find(line => /^ql\s+(repo|raw)\b/.test(line)) || cmd
  const hookScript = normalizeRecognizedHookScript(
    lines.filter(line => line !== qlLine && !/^ql\s+(repo|raw)\b/.test(line)).join(' ; ')
  )

  const repoMatch = qlLine.match(/ql\s+repo\s+"?([^\s"]+)"?\s*"?([^"]*)"?\s*"?([^"]*)"?\s*"?([^"]*)"?\s*"?([^"]*)"?/)
  if (repoMatch) {
    const [, url = '', whitelist, blacklist, dependOn, branch] = repoMatch
    const repoName = url.replace(/\.git$/, '').split('/').pop() || 'repo'
    editForm.value.type = 'git-repo'
    editForm.value.url = addGithubMirror(url)
    editForm.value.name = repoName
    editForm.value.save_dir = deriveSubscriptionSaveDir(url)
    editForm.value.whitelist = whitelist || ''
    editForm.value.blacklist = blacklist || ''
    editForm.value.branch = branch || ''
    editForm.value.depend_on = dependOn || ''
    if (hookScript) editForm.value.hook_script = hookScript
    editForm.value.auto_add_task = true
    ElMessage.success('已识别 ql repo 命令')
    qlCommand.value = ''
    return
  }

  const rawMatch = qlLine.match(/ql\s+raw\s+"?([^\s"]+)"?/)
  if (rawMatch) {
    const url = rawMatch[1] || ''
    const fileName = url.split('/').pop() || 'file'
    editForm.value.type = 'single-file'
    editForm.value.url = addGithubMirror(url)
    editForm.value.name = fileName.replace(/\.[^/.]+$/, '')
    editForm.value.save_dir = deriveSubscriptionSaveDir(url) || 'downloads'
    if (hookScript) editForm.value.hook_script = hookScript
    editForm.value.auto_add_task = true
    ElMessage.success('已识别 ql raw 命令')
    qlCommand.value = ''
    return
  }

  if (cmd.includes('github.com') || cmd.includes('.git') || cmd.startsWith('http')) {
    editForm.value.url = addGithubMirror(cmd)
    const repoName = cmd.replace(/\.git$/, '').split('/').pop() || ''
    if (repoName) editForm.value.name = repoName
    editForm.value.save_dir = deriveSubscriptionSaveDir(cmd)
    editForm.value.type = cmd.endsWith('.js') || cmd.endsWith('.py') || cmd.endsWith('.ts') || cmd.endsWith('.sh') ? 'single-file' : 'git-repo'
    ElMessage.success('已识别链接')
    qlCommand.value = ''
    return
  }

  ElMessage.warning('无法识别命令格式，支持 ql repo/raw 命令或直接粘贴链接')
}

function openEdit(row: any) {
  isCreate.value = false
  editForm.value = {
    id: row.id, name: row.name, type: row.type, url: row.url,
    branch: row.branch || '', schedule: row.schedule || '',
    whitelist: row.whitelist || '', blacklist: row.blacklist || '',
    depend_on: row.depend_on || '', hook_script: row.hook_script || '', auto_add_task: row.auto_add_task,
    auto_del_task: row.auto_del_task, save_dir: row.save_dir || '', sub_path: row.sub_path || '',
    ssh_key_id: row.ssh_key_id, alias: row.alias || '',
    force_overwrite: row.force_overwrite !== false
  }
  showEditDialog.value = true
}

async function handleSave() {
  if (!editForm.value.name.trim() || !editForm.value.url.trim()) {
    ElMessage.warning('名称和 URL 不能为空')
    return
  }
  const mirror = normalizeMirror(githubMirror.value)
  const mirrorHost = mirror.replace(/^https?:\/\//, '').replace(/\/$/, '')
  const githubDirect = /^https?:\/\/github\.com\//.test(editForm.value.url) && mirrorHost && !editForm.value.url.includes(mirrorHost)
  if (githubDirect) {
    try {
      await ElMessageBox.confirm(
        '检测到 GitHub 直连地址，是否自动添加镜像加速？\n加速地址: ' + mirror,
        '镜像加速',
        { confirmButtonText: '添加加速', cancelButtonText: '保持原样', type: 'info' }
      )
      editForm.value.url = addGithubMirror(editForm.value.url)
    } catch { /* keep original */ }
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
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || (isCreate.value ? '创建失败' : '更新失败'))
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
    const enabling = !row.enabled
    await ElMessageBox.confirm(
      enabling
        ? `确认启用订阅「${row.name}」吗？`
        : `确认禁用订阅「${row.name}」吗？禁用后将停止后续自动拉取。`,
      enabling ? '启用确认' : '禁用确认',
      { type: enabling ? 'info' : 'warning' }
    )
    if (row.enabled) {
      await subscriptionApi.disable(row.id)
    } else {
      await subscriptionApi.enable(row.id)
    }
    ElMessage.success(row.enabled ? '已禁用' : '已启用')
    loadData()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error(err?.response?.data?.error || '操作失败')
  }
}

async function handlePullWithMode(row: any, mode: string) {
  if (pullingSubId.value === row.id && pullRunning.value) {
    showPullLog.value = true
    return
  }

  const modeLabels: Record<string, string> = { default: '按订阅设置', force: '覆盖', keep: '保留本地修改' }
  try {
    await ElMessageBox.confirm(
      `确认以「${modeLabels[mode] || '默认'}」模式拉取订阅「${row.name}」吗？`,
      '拉取确认',
      { type: 'warning', confirmButtonText: '立即拉取', cancelButtonText: '取消' }
    )
  } catch {
    return
  }

  try {
    const params: Record<string, string> = {}
    if (mode === 'force') params.force_overwrite = 'true'
    else if (mode === 'keep') params.force_overwrite = 'false'
    await subscriptionApi.pull(row.id, params)
    pullLogLines.value = []
    pullRunning.value = true
    pullingSubId.value = row.id
    showPullLog.value = true
    connectPullStream(row.id)
  } catch (err: any) {
    if (err?.response?.data?.error?.includes('拉取中')) {
      pullRunning.value = true
      pullingSubId.value = row.id
      showPullLog.value = true
      connectPullStream(row.id)
      return
    }
    ElMessage.error(err?.response?.data?.error || '拉取失败')
  }
}

async function handlePull(row: any) {
  await handlePullWithMode(row, 'default')
}

async function handleStopPull() {
  if (!pullingSubId.value) {
    return
  }

  try {
    await ElMessageBox.confirm('确认停止当前拉库任务吗？', '停止拉库', {
      type: 'warning',
      confirmButtonText: '停止',
      cancelButtonText: '取消'
    })
    await subscriptionApi.stopPull(pullingSubId.value)
    ElMessage.success('已发送停止请求')
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error(err?.response?.data?.error || '停止失败')
  }
}

function connectPullStream(id: number) {
  closePullStream()
  const base = import.meta.env.VITE_API_BASE || '/api/v1'
  const url = `${base}/subscriptions/${id}/pull-stream`
  pullEventSource = openAuthorizedEventStream(url, {
    onMessage(data) {
      pullBuffer.push(data)
      if (!pullFlushRaf) {
        pullFlushRaf = requestAnimationFrame(() => {
          pullLogLines.value.push(...pullBuffer)
          pullBuffer = []
          pullFlushRaf = 0
          if (pullLogRef.value) pullLogRef.value.scrollTop = pullLogRef.value.scrollHeight
        })
      }
    },
    onEvent(event) {
      if (event.event === 'done') {
        pullRunning.value = false
        pullingSubId.value = null
        closePullStream()
        loadData()
      }
    },
    onError() {
      pullRunning.value = false
      pullingSubId.value = null
      closePullStream()
    }
  })
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
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '加载日志失败')
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

function viewLogDetail(log: any) {
  logDetailContent.value = log.content || '(无日志内容)'
  showLogDetail.value = true
}
</script>

<template>
  <div class="subscriptions-page">
    <div class="page-header">
      <div>
        <h2>订阅管理</h2>
        <span class="page-subtitle">管理 Git 仓库和单文件自动拉取订阅</span>
      </div>
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
        <el-button @click="handleConfigMirror" title="配置 GitHub 镜像加速地址">
          <el-icon><Setting /></el-icon>镜像设置
        </el-button>
      </div>
    </div>

    <div v-if="isMobile" class="dd-mobile-list">
      <div
        v-for="row in subList"
        :key="row.id"
        class="dd-mobile-card"
      >
        <div class="dd-mobile-card__header">
          <div class="dd-mobile-card__title-wrap">
            <div class="subscription-card__title-row">
              <div class="dd-mobile-card__selection">
                <el-checkbox :model-value="isSelected(row.id)" @change="toggleSelected(row.id, $event)" />
                <span class="dd-mobile-card__title">{{ row.name }}</span>
              </div>
              <el-tag size="small" :type="row.type === 'git-repo' ? '' : 'warning'">
                {{ row.type === 'git-repo' ? 'Git 仓库' : '单文件' }}
              </el-tag>
            </div>
            <div class="dd-mobile-card__subtitle">{{ row.url }}</div>
          </div>
        </div>

        <div class="dd-mobile-card__body">
          <div class="dd-mobile-card__grid">
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">分支</span>
              <span class="dd-mobile-card__value">{{ row.branch || '-' }}</span>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">状态</span>
              <div class="dd-mobile-card__value">
                <el-tag size="small" :type="getStatusTag(row.status)">{{ getStatusText(row.status) }}</el-tag>
              </div>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">定时拉取</span>
              <span class="dd-mobile-card__value">{{ row.schedule || '手动拉取' }}</span>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">启用</span>
              <div class="dd-mobile-card__value">
                <el-switch :model-value="row.enabled" size="small" @change="handleToggle(row)" />
              </div>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">最后拉取</span>
              <span class="dd-mobile-card__value">{{ row.last_pull_at ? new Date(row.last_pull_at).toLocaleString() : '-' }}</span>
            </div>
          </div>

          <div class="dd-mobile-card__actions subscription-card__actions">
            <el-dropdown trigger="click" @command="(cmd: string) => handlePullWithMode(row, cmd)">
              <el-button size="small" type="success">拉取</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="default">按订阅设置拉取</el-dropdown-item>
                  <el-dropdown-item command="force">覆盖拉取</el-dropdown-item>
                  <el-dropdown-item command="keep">保留本地修改拉取</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" @click="openLogs(row.id)">日志</el-button>
            <el-button size="small" type="primary" plain @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" plain @click="handleDelete(row.id)">删除</el-button>
          </div>
        </div>
      </div>

      <el-empty v-if="!loading && subList.length === 0" description="暂无订阅" />
    </div>

    <el-table
      v-else
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
      <el-table-column prop="schedule" label="定时拉取" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.schedule || '手动拉取' }}
        </template>
      </el-table-column>
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
            <el-dropdown trigger="click" @command="(cmd: string) => handlePullWithMode(row, cmd)">
              <el-button size="small" type="success" plain circle>
                <el-icon><Download /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="default">按订阅设置拉取</el-dropdown-item>
                  <el-dropdown-item command="force">覆盖拉取</el-dropdown-item>
                  <el-dropdown-item command="keep">保留本地修改拉取</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
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

    <el-dialog v-model="showEditDialog" :title="isCreate ? '新建订阅' : '编辑订阅'" width="600px" :fullscreen="dialogFullscreen">
      <el-form :model="editForm" :label-width="dialogFullscreen ? 'auto' : '100px'" :label-position="dialogFullscreen ? 'top' : 'right'">
        <el-form-item v-if="isCreate" label="一键识别">
          <div style="display: flex; gap: 8px; width: 100%">
            <el-input v-model="qlCommand" placeholder="粘贴 ql repo/raw 命令或仓库链接" clearable @keyup.enter="parseQLCommand" />
            <el-button type="primary" @click="parseQLCommand">识别</el-button>
          </div>
        </el-form-item>
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
        <el-form-item v-if="editForm.type === 'git-repo'" label="指定子目录">
          <el-input v-model="editForm.sub_path" placeholder="仅拉取仓库中的指定子目录 (逗号分隔多个)" />
          <div style="color: var(--el-text-color-secondary); font-size: 12px; margin-top: 4px; line-height: 1.4">
            留空拉取全部内容，填写后仅检出指定子目录（如 scripts/daily, utils）
          </div>
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
        <el-form-item label="依赖说明">
          <el-input v-model="editForm.depend_on" placeholder="用于记录订阅依赖、过滤说明或迁移信息" />
        </el-form-item>
        <el-form-item v-if="editForm.type === 'git-repo'" label="覆盖本地修改">
          <el-switch v-model="editForm.force_overwrite" />
          <span style="margin-left: 8px; color: var(--el-text-color-secondary); font-size: 12px">
            {{ editForm.force_overwrite ? '拉取时覆盖本地修改并清理多余文件' : '拉取时保留本地修改的文件' }}
          </span>
        </el-form-item>
        <el-form-item label="拉取后钩子">
          <el-input
            v-model="editForm.hook_script"
            type="textarea"
            :rows="4"
            placeholder="拉取成功后执行的 Shell 命令。支持使用 $SUB_DIR、$SCRIPTS_DIR、$QL_DIR 等变量。"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSave">{{ isCreate ? '创建' : '保存' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showLogDialog" title="拉取日志" width="700px" :fullscreen="dialogFullscreen">
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
          <template #default="{ row }">{{ typeof row.duration === 'number' ? row.duration.toFixed(1) + 's' : '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right" align="center">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="viewLogDetail(row)">查看</el-button>
          </template>
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

    <el-dialog v-model="showLogDetail" title="日志详情" width="700px" :fullscreen="dialogFullscreen">
      <pre class="pull-log-content" style="min-height: 100px">{{ logDetailContent || '(无日志内容)' }}</pre>
    </el-dialog>

    <el-dialog v-model="showPullLog" title="拉取日志" width="700px" :fullscreen="dialogFullscreen" :close-on-click-modal="false" @close="closePullStream">
      <div ref="pullLogRef" class="pull-log-content">
        <div v-for="(line, i) in pullLogLines" :key="i" class="pull-log-line">{{ line }}</div>
        <div v-if="pullRunning" class="pull-log-line pull-running">
      <span class="pull-spinner"></span> 拉取中...
    </div>
        <el-empty v-if="!pullRunning && pullLogLines.length === 0" description="暂无输出" :image-size="60" />
      </div>
      <template #footer>
        <el-tag v-if="pullRunning" type="warning" effect="plain" size="small" style="margin-right: auto">运行中</el-tag>
        <el-tag v-else-if="pullLogLines.length > 0" type="success" effect="plain" size="small" style="margin-right: auto">已完成</el-tag>
        <el-button v-if="pullRunning" type="danger" @click="handleStopPull">停止</el-button>
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
  flex-wrap: wrap;
  gap: 12px;

  h2 { margin: 0; font-size: 20px; font-weight: 700; color: var(--el-text-color-primary); }

  .page-subtitle {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    display: block;
    margin-top: 2px;
  }

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

.subscription-card__title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.subscription-card__actions > * {
  flex: 1 1 calc(50% - 4px);
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
  display: flex;
  align-items: center;
  gap: 8px;
}

.pull-spinner {
  display: inline-block;
  width: 10px;
  height: 10px;
  border: 2px solid rgba(230, 162, 60, 0.3);
  border-top-color: #e6a23c;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  flex-shrink: 0;
}

@media (max-width: 768px) {
  .page-header {
    align-items: flex-start;

    .header-left,
    .header-right {
      width: 100%;
      flex-wrap: wrap;
    }

    .header-left {
      :deep(.el-input) {
        width: 100% !important;
      }
    }
  }

  .subscription-card__title-row {
    flex-direction: column;
  }
}
</style>
