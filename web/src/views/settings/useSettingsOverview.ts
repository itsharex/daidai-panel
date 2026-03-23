import { onBeforeUnmount, ref } from 'vue'
import { systemApi, type PanelUpdateStatus } from '@/api/system'
import { ElMessage, ElMessageBox } from 'element-plus'

type UpdateVisualStatus = 'idle' | 'running' | 'restarting' | 'failed' | 'timeout'

export function useSettingsOverview() {
  const systemInfo = ref<any>({})
  const systemStats = ref<any>(null)
  const currentVersion = ref('')
  const updateInfo = ref<any>(null)
  const updateStatus = ref<PanelUpdateStatus | null>(null)
  const checkingUpdate = ref(false)
  const updatingPanel = ref(false)
  const updateProgressVisible = ref(false)
  const updateProgressStatus = ref<UpdateVisualStatus>('idle')
  const updateProgressError = ref('')
  let updateStatusPollTimer: ReturnType<typeof setTimeout> | null = null
  let updateAvailabilityDelayTimer: ReturnType<typeof setTimeout> | null = null
  let updateAvailabilityTimer: ReturnType<typeof setTimeout> | null = null

  function formatBytes(bytes: number): string {
    if (!bytes) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
  }

  function getUsageClass(percent: number): string {
    if (!percent) return ''
    if (percent < 60) return 'usage-success'
    if (percent < 80) return 'usage-warning'
    return 'usage-danger'
  }

  async function loadSystemInfo() {
    try {
      const res = await systemApi.info()
      systemInfo.value = res.data || {}
    } catch {
      // ignore
    }
  }

  async function loadSystemStats() {
    try {
      const res = await systemApi.stats()
      systemStats.value = res.data || {}
    } catch {
      // ignore
    }
  }

  async function loadVersion() {
    try {
      const res = await systemApi.version()
      currentVersion.value = res.data.version || ''
    } catch {
      // ignore
    }
  }

  async function handleCheckUpdate() {
    checkingUpdate.value = true
    try {
      const res = await systemApi.checkUpdate()
      updateInfo.value = res.data
      if (res.data.has_update) {
        if (res.data.auto_update_supported) {
          ElMessage.success(`发现新版本 v${res.data.latest}，可在下方直接点击“立即更新”`)
        } else {
          ElMessage.warning(res.data.update_disabled_reason || '当前部署暂不支持面板内一键更新')
        }
      } else {
        ElMessage.success(`当前版本 v${res.data.current} 已经是最新版了`)
      }
    } catch (err: any) {
      const msg = err?.response?.data?.error || '检查更新失败，请稍后重试'
      ElMessage.error(msg)
    } finally {
      checkingUpdate.value = false
    }
  }

  async function handleUpdatePanel() {
    if (updatingPanel.value) {
      ElMessage.warning('更新任务已经在进行中，请稍候')
      return
    }

    if (updateInfo.value?.auto_update_supported === false) {
      ElMessage.warning(updateInfo.value?.update_disabled_reason || '当前部署暂不支持面板内一键更新')
      return
    }

    try {
      await ElMessageBox.confirm(
        '确认开始更新面板吗？系统会先拉取最新镜像，再自动重建容器。更新期间服务会短暂中断。',
        '立即更新',
        {
          confirmButtonText: '开始更新',
          cancelButtonText: '取消',
          type: 'warning'
        }
      )
    } catch (err: any) {
      if (err === 'cancel' || err?.toString?.() === 'cancel') {
        return
      }
      ElMessage.error(err?.message || '无法确认更新操作')
      return
    }

    updatingPanel.value = true
    openUpdateProgress({
      status: 'running',
      phase: 'preparing',
      message: '正在提交更新任务',
      started_at: new Date().toISOString(),
    })

    try {
      const res = await systemApi.updatePanel()
      applyUpdateSnapshot(res.data || updateStatus.value)
      startUpdateStatusPolling()
    } catch (err: any) {
      failUpdateProgress(err?.response?.data?.error || err?.message || '更新失败，请手动更新')
    }
  }

  function openUpdateProgress(snapshot?: PanelUpdateStatus | null) {
    updateProgressVisible.value = true
    updateProgressStatus.value = 'running'
    updateProgressError.value = ''
    updateStatus.value = snapshot || {
      status: 'running',
      phase: 'preparing',
      message: '正在准备更新任务...',
      started_at: new Date().toISOString(),
    }
  }

  function applyUpdateSnapshot(snapshot?: PanelUpdateStatus | null) {
    updateStatus.value = snapshot || {}
    updateProgressVisible.value = true

    if (updateStatus.value?.status === 'failed') {
      updateProgressStatus.value = 'failed'
      updateProgressError.value = updateStatus.value?.error || updateStatus.value?.message || '更新失败'
      updatingPanel.value = false
      return
    }

    if (updateStatus.value?.status === 'restarting') {
      updateProgressStatus.value = 'restarting'
      updateProgressError.value = ''
      return
    }

    updateProgressStatus.value = 'running'
    updateProgressError.value = ''
  }

  function failUpdateProgress(message: string) {
    stopUpdateStatusPolling()
    stopUpdateAvailabilityChecks()
    updateProgressVisible.value = true
    updateProgressStatus.value = 'failed'
    updateProgressError.value = message
    updateStatus.value = {
      ...(updateStatus.value || {}),
      status: 'failed',
      phase: updateStatus.value?.phase || 'failed',
      message,
      error: message,
    }
    updatingPanel.value = false
    ElMessage.error(message)
  }

  function startUpdateStatusPolling() {
    stopUpdateStatusPolling()
    const startedAt = Date.now()

    const poll = async () => {
      try {
        const res = await systemApi.updateStatus()
        applyUpdateSnapshot(res.data || {})

        if (updateStatus.value?.status === 'failed') {
          return
        }

        if (updateStatus.value?.status === 'restarting') {
          stopUpdateStatusPolling()
          waitForAvailability()
          return
        }

        if (Date.now() - startedAt >= 12 * 60 * 1000) {
          failUpdateProgress('更新超时，请手动刷新页面检查')
          return
        }

        updateStatusPollTimer = setTimeout(() => {
          void poll()
        }, 2000)
      } catch (err: any) {
        if (shouldTreatAsRestart(err)) {
          stopUpdateStatusPolling()
          updateProgressStatus.value = 'restarting'
          waitForAvailability()
          return
        }
        failUpdateProgress(err?.response?.data?.error || err?.message || '更新状态获取失败')
      }
    }

    void poll()
  }

  async function handleRestartPanel() {
    try {
      await ElMessageBox.confirm('确定要重启面板吗？重启期间服务将短暂中断。', '重启面板', {
        confirmButtonText: '确认重启',
        cancelButtonText: '取消',
        type: 'warning'
      })
      await systemApi.restart()
      waitForRestart()
    } catch {
      // cancelled
    }
  }

  function waitForRestart() {
    let attempts = 0
    setTimeout(() => {
      const poll = setInterval(async () => {
        attempts++
        try {
          const res = await fetch('/', { method: 'HEAD' })
          if (res.ok) {
            clearInterval(poll)
            window.location.reload()
          }
        } catch {
          // ignore
        }
        if (attempts >= 60) {
          clearInterval(poll)
          ElMessage.warning('重启超时，请手动刷新页面')
        }
      }, 2000)
    }, 3000)
  }

  function waitForAvailability() {
    stopUpdateAvailabilityChecks()
    updateProgressVisible.value = true
    updateProgressStatus.value = 'restarting'

    let attempts = 0
    updateAvailabilityDelayTimer = setTimeout(() => {
      updateAvailabilityDelayTimer = null
      const probe = async () => {
        attempts++
        try {
          const res = await fetch('/', { method: 'HEAD', cache: 'no-store' })
          if (res.ok) {
            stopUpdateAvailabilityChecks()
            window.location.reload()
            return
          }
        } catch {
          // ignore
        }

        if (attempts >= 80) {
          stopUpdateAvailabilityChecks()
          updateProgressStatus.value = 'timeout'
          updateProgressError.value = '等待新版本启动超时，请稍后手动刷新页面检查'
          updatingPanel.value = false
          ElMessage.warning('等待新版本启动超时，请手动刷新页面检查')
          return
        }

        updateAvailabilityTimer = setTimeout(() => {
          void probe()
        }, 3000)
      }

      void probe()
    }, 2000)
  }

  function stopUpdateStatusPolling() {
    if (updateStatusPollTimer) {
      clearTimeout(updateStatusPollTimer)
      updateStatusPollTimer = null
    }
  }

  function stopUpdateAvailabilityChecks() {
    if (updateAvailabilityDelayTimer) {
      clearTimeout(updateAvailabilityDelayTimer)
      updateAvailabilityDelayTimer = null
    }
    if (updateAvailabilityTimer) {
      clearTimeout(updateAvailabilityTimer)
      updateAvailabilityTimer = null
    }
  }

  function closeUpdateProgress() {
    if (updateProgressStatus.value === 'running' || updateProgressStatus.value === 'restarting') {
      return
    }
    stopUpdateStatusPolling()
    stopUpdateAvailabilityChecks()
    updateProgressVisible.value = false
    updateProgressStatus.value = 'idle'
    updateProgressError.value = ''
  }

  function shouldTreatAsRestart(err: any) {
    if (!updateStatus.value?.status) {
      return false
    }

    if (err?.response) {
      return false
    }

    return updateStatus.value.status === 'running' || updateStatus.value.status === 'restarting'
  }

  function openGitHub() {
    const url = updateInfo.value?.has_update && updateInfo.value?.release_url
      ? updateInfo.value.release_url
      : 'https://github.com/linzixuanzz/daidai-panel/releases'
    window.open(url, '_blank')
  }

  onBeforeUnmount(() => {
    stopUpdateStatusPolling()
    stopUpdateAvailabilityChecks()
  })

  return {
    systemInfo,
    systemStats,
    currentVersion,
    updateInfo,
    updateStatus,
    checkingUpdate,
    updatingPanel,
    updateProgressVisible,
    updateProgressStatus,
    updateProgressError,
    formatBytes,
    getUsageClass,
    loadSystemInfo,
    loadSystemStats,
    loadVersion,
    handleCheckUpdate,
    handleUpdatePanel,
    handleRestartPanel,
    openGitHub,
    closeUpdateProgress
  }
}
