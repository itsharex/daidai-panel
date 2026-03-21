import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { systemApi, type BackupSelection } from '@/api/system'
import { securityApi } from '@/api/security'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { ElLoading, ElMessage, ElMessageBox } from 'element-plus'
import { createQrCodeDataUrl } from '@/utils/qrcode'

export function useSettingsSecurity() {
  const router = useRouter()
  const authStore = useAuthStore()

  const securityTab = ref('password-2fa')

  const backups = ref<any[]>([])
  const backupsLoading = ref(false)
  const showBackupDialog = ref(false)
  const backupPassword = ref('')
  const backupSelection = ref<BackupSelection>({
    configs: true,
    tasks: true,
    subscriptions: true,
    env_vars: true,
    logs: true,
    scripts: true,
    dependencies: true,
  })
  const showRestoreDialog = ref(false)
  const restoreFilename = ref('')
  const restorePassword = ref('')
  const restoreCountdown = ref(0)
  let restoreTimer: ReturnType<typeof setInterval> | null = null

  const oldPassword = ref('')
  const newPassword = ref('')
  const confirmPassword = ref('')

  const twoFAEnabled = ref(false)
  const twoFASecret = ref('')
  const twoFAUri = ref('')
  const twoFAQrUrl = ref('')
  const twoFACode = ref('')
  const showSetup2FA = ref(false)

  const loginLogs = ref<any[]>([])
  const loginLogsLoading = ref(false)
  const loginLogsTotal = ref(0)
  const loginLogsPage = ref(1)

  const sessions = ref<any[]>([])
  const sessionsLoading = ref(false)

  const ipWhitelist = ref<any[]>([])
  const ipWhitelistLoading = ref(false)
  const showAddIPDialog = ref(false)
  const newIP = ref('')
  const newIPRemarks = ref('')

  async function loadBackups() {
    backupsLoading.value = true
    try {
      const res = await systemApi.backupList()
      backups.value = res.data || []
    } catch {
      ElMessage.error('加载备份列表失败')
    } finally {
      backupsLoading.value = false
    }
  }

  async function handleCreateBackup() {
    showBackupDialog.value = true
    backupPassword.value = ''
    backupSelection.value = {
      configs: true,
      tasks: true,
      subscriptions: true,
      env_vars: true,
      logs: true,
      scripts: true,
      dependencies: true,
    }
  }

  async function handleUploadBackup(e: Event) {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    if (!file) return
    try {
      await systemApi.uploadBackup(file)
      ElMessage.success('备份文件导入成功')
      void loadBackups()
    } catch {
      ElMessage.error('导入备份失败')
    }
    input.value = ''
  }

  async function confirmCreateBackup() {
    try {
      const hasSelection = Object.values(backupSelection.value).some(Boolean)
      if (!hasSelection) {
        ElMessage.warning('请至少选择一个备份项')
        return
      }
      await systemApi.backup(backupPassword.value, backupSelection.value)
      ElMessage.success('备份创建成功')
      showBackupDialog.value = false
      backupPassword.value = ''
      void loadBackups()
    } catch {
      ElMessage.error('备份失败')
    }
  }

  async function handleDownloadBackup(filename: string) {
    try {
      const blob = await systemApi.downloadBackup(filename)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
    } catch {
      ElMessage.error('下载备份失败')
    }
  }

  async function handleRestoreBackup(filename: string) {
    restoreFilename.value = filename
    restorePassword.value = ''
    showRestoreDialog.value = true
  }

  async function confirmRestore() {
    try {
      await systemApi.restore(restoreFilename.value, restorePassword.value)
      showRestoreDialog.value = false
      restoreCountdown.value = 10
      ElMessageBox.alert(
        '',
        '恢复成功',
        {
          confirmButtonText: '立即重启',
          type: 'success',
          showClose: false,
          closeOnClickModal: false,
          closeOnPressEscape: false,
          message: `数据恢复成功，面板将在 ${restoreCountdown.value} 秒后自动重启...`,
          callback: () => {
            if (restoreTimer) {
              clearInterval(restoreTimer)
              restoreTimer = null
            }
            void doRestart()
          }
        }
      )
      restoreTimer = setInterval(() => {
        restoreCountdown.value--
        const msgBox = document.querySelector('.el-message-box__message p')
        if (msgBox) {
          msgBox.textContent = `数据恢复成功，面板将在 ${restoreCountdown.value} 秒后自动重启...`
        }
        if (restoreCountdown.value <= 0) {
          if (restoreTimer) {
            clearInterval(restoreTimer)
            restoreTimer = null
          }
          ElMessageBox.close()
          void doRestart()
        }
      }, 1000)
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.error || '恢复失败')
    }
  }

  async function doRestart() {
    try {
      await systemApi.restart()
    } catch {
      // ignore
    }
    waitForRestart()
  }

  function waitForRestart() {
    const loading = ElLoading.service({
      lock: true,
      text: '面板正在重启，请稍候...',
      background: 'rgba(0, 0, 0, 0.7)'
    })
    let attempts = 0
    setTimeout(() => {
      const poll = setInterval(async () => {
        attempts++
        try {
          const res = await fetch('/', { method: 'HEAD' })
          if (res.ok) {
            clearInterval(poll)
            loading.close()
            window.location.reload()
          }
        } catch {
          // ignore
        }
        if (attempts >= 60) {
          clearInterval(poll)
          loading.close()
          ElMessage.warning('重启超时，请手动刷新页面')
        }
      }, 2000)
    }, 3000)
  }

  async function handleDeleteBackup(filename: string) {
    try {
      await ElMessageBox.confirm('确定要删除该备份吗？', '确认', { type: 'warning' })
      await systemApi.deleteBackup(filename)
      ElMessage.success('删除成功')
      void loadBackups()
    } catch {
      // cancelled
    }
  }

  async function load2FAStatus() {
    try {
      const res = await securityApi.get2FAStatus()
      twoFAEnabled.value = res.data.enabled
    } catch {
      // ignore
    }
  }

  async function handleChangePassword() {
    if (!oldPassword.value || !newPassword.value) {
      ElMessage.warning('请填写密码')
      return
    }
    if (newPassword.value !== confirmPassword.value) {
      ElMessage.warning('两次输入的密码不一致')
      return
    }
    if (newPassword.value.length < 6) {
      ElMessage.warning('密码至少 6 位')
      return
    }
    try {
      await authApi.changePassword(oldPassword.value, newPassword.value)
      ElMessage.success('密码修改成功，即将跳转到登录页')
      oldPassword.value = ''
      newPassword.value = ''
      confirmPassword.value = ''
      setTimeout(() => {
        authStore.logout()
      }, 1500)
    } catch {
      ElMessage.error('密码修改失败')
    }
  }

  async function handleSetup2FA() {
    try {
      const res = await securityApi.setup2FA()
      twoFASecret.value = res.data.secret
      twoFAUri.value = res.data.uri
      twoFAQrUrl.value = await createQrCodeDataUrl(res.data.uri, 200)
      twoFACode.value = ''
      showSetup2FA.value = true
    } catch {
      ElMessage.error('初始化 2FA 失败')
    }
  }

  async function handleVerify2FA() {
    if (!twoFACode.value) {
      ElMessage.warning('请输入验证码')
      return
    }
    try {
      await securityApi.verify2FA(twoFACode.value)
      ElMessage.success('2FA 已启用')
      twoFAEnabled.value = true
      showSetup2FA.value = false
    } catch {
      ElMessage.error('验证码错误')
    }
  }

  async function handleDisable2FA() {
    try {
      await ElMessageBox.confirm('确定要禁用两步验证吗？', '确认', { type: 'warning' })
      await securityApi.disable2FA()
      ElMessage.success('2FA 已禁用')
      twoFAEnabled.value = false
    } catch {
      // cancelled
    }
  }

  async function loadLoginLogs() {
    loginLogsLoading.value = true
    try {
      const res = await securityApi.loginLogs({ page: loginLogsPage.value, page_size: 15 })
      loginLogs.value = res.data || []
      loginLogsTotal.value = res.total || 0
    } catch {
      ElMessage.error('加载登录日志失败')
    } finally {
      loginLogsLoading.value = false
    }
  }

  async function loadSessions() {
    sessionsLoading.value = true
    try {
      const res = await securityApi.sessions()
      sessions.value = res.data || []
    } catch {
      ElMessage.error('加载会话列表失败')
    } finally {
      sessionsLoading.value = false
    }
  }

  async function handleRevokeSession(id: number) {
    try {
      await securityApi.revokeSession(id)
      ElMessage.success('会话已撤销，即将重新登录')
      authStore.clearAuth()
      setTimeout(() => void router.push('/login'), 500)
    } catch {
      ElMessage.error('操作失败')
    }
  }

  async function handleRevokeAllSessions() {
    try {
      await ElMessageBox.confirm('确定要撤销所有其他会话吗？', '确认', { type: 'warning' })
      await securityApi.revokeAllSessions()
      ElMessage.success('已撤销所有其他会话')
      void loadSessions()
    } catch {
      // cancelled
    }
  }

  async function loadIPWhitelist() {
    ipWhitelistLoading.value = true
    try {
      const res = await securityApi.ipWhitelist()
      ipWhitelist.value = res.data || []
    } catch {
      ElMessage.error('加载 IP 白名单失败')
    } finally {
      ipWhitelistLoading.value = false
    }
  }

  async function handleAddIP() {
    if (!newIP.value.trim()) {
      ElMessage.warning('IP 或网段不能为空')
      return
    }
    try {
      await securityApi.addIPWhitelist({ ip: newIP.value.trim(), remarks: newIPRemarks.value })
      ElMessage.success('添加成功')
      showAddIPDialog.value = false
      newIP.value = ''
      newIPRemarks.value = ''
      void loadIPWhitelist()
    } catch {
      ElMessage.error('添加失败')
    }
  }

  async function handleRemoveIP(id: number) {
    try {
      await ElMessageBox.confirm('确定要移除该 IP 吗？', '确认', { type: 'warning' })
      await securityApi.removeIPWhitelist(id)
      ElMessage.success('删除成功')
      void loadIPWhitelist()
    } catch {
      // cancelled
    }
  }

  async function handleClearLoginLogs() {
    try {
      await ElMessageBox.confirm('确定要清除所有登录日志吗？此操作不可恢复。', '确认', { type: 'warning' })
      const res = await securityApi.clearLoginLogs() as any
      ElMessage.success(res.message || '清除成功')
      void loadLoginLogs()
    } catch (e: any) {
      if (e !== 'cancel' && e?.toString() !== 'cancel') {
        ElMessage.error('清除失败')
      }
    }
  }

  function handleSecurityTabChange(tab: string) {
    if (tab === 'login-logs') void loadLoginLogs()
    else if (tab === 'sessions') void loadSessions()
    else if (tab === 'ip-whitelist') void loadIPWhitelist()
  }

  return {
    securityTab,
    backups,
    backupsLoading,
    showBackupDialog,
    backupPassword,
    backupSelection,
    showRestoreDialog,
    restoreFilename,
    restorePassword,
    oldPassword,
    newPassword,
    confirmPassword,
    twoFAEnabled,
    twoFASecret,
    twoFAUri,
    twoFAQrUrl,
    twoFACode,
    showSetup2FA,
    loginLogs,
    loginLogsLoading,
    loginLogsTotal,
    loginLogsPage,
    sessions,
    sessionsLoading,
    ipWhitelist,
    ipWhitelistLoading,
    showAddIPDialog,
    newIP,
    newIPRemarks,
    loadBackups,
    handleCreateBackup,
    handleUploadBackup,
    confirmCreateBackup,
    handleDownloadBackup,
    handleRestoreBackup,
    confirmRestore,
    handleDeleteBackup,
    load2FAStatus,
    handleChangePassword,
    handleSetup2FA,
    handleVerify2FA,
    handleDisable2FA,
    loadLoginLogs,
    loadSessions,
    handleRevokeSession,
    handleRevokeAllSessions,
    loadIPWhitelist,
    handleAddIP,
    handleRemoveIP,
    handleClearLoginLogs,
    handleSecurityTabChange
  }
}
