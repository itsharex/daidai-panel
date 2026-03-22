<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import BackupManagementCard from './components/BackupManagementCard.vue'
import CaptchaConfigCard from './components/CaptchaConfigCard.vue'
import IPWhitelistCard from './components/IPWhitelistCard.vue'
import LoginLogsCard from './components/LoginLogsCard.vue'
import OverviewHeroCard from './components/OverviewHeroCard.vue'
import OverviewStatsCard from './components/OverviewStatsCard.vue'
import ProxyConfigCard from './components/ProxyConfigCard.vue'
import SecurityPassword2FACard from './components/SecurityPassword2FACard.vue'
import SessionManagementCard from './components/SessionManagementCard.vue'
import SystemConfigCard from './components/SystemConfigCard.vue'
import SystemInfoCard from './components/SystemInfoCard.vue'
import TaskExecutionCard from './components/TaskExecutionCard.vue'
import { useSettingsConfig } from './useSettingsConfig'
import { useSettingsOverview } from './useSettingsOverview'
import { useSettingsSecurity } from './useSettingsSecurity'
import { Connection, Document, Lock, Monitor, Refresh } from '@element-plus/icons-vue'

const authStore = useAuthStore()
const roleLevel: Record<string, number> = { viewer: 1, operator: 2, admin: 3 }
const isAdmin = computed(() => (roleLevel[authStore.user?.role || ''] || 0) >= (roleLevel.admin || 0))

const activeTab = ref('overview')

const overview = useSettingsOverview()
const config = useSettingsConfig()
const security = useSettingsSecurity()

const {
  systemInfo,
  systemStats,
  currentVersion,
  updateInfo,
  updateStatus,
  checkingUpdate,
  updatingPanel,
  formatBytes,
  getUsageClass,
  loadSystemInfo,
  loadSystemStats,
  loadVersion,
  handleCheckUpdate,
  handleUpdatePanel,
  handleRestartPanel,
  openGitHub
} = overview

const {
  captchaFeatureImplemented,
  configsLoading,
  configsSaving,
  configForm,
  loadSystemConfigs,
  handleSaveSystemConfig,
  handleIconUpload,
  handleLogBackgroundUpload,
  handleSaveTaskConfig,
  handleSaveProxy,
  handleSaveCaptcha
} = config

const {
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
} = security

function handleRefresh() {
  handleTabChange(activeTab.value)
}

function handleTabChange(tab: string) {
  if (tab === 'overview') {
    void loadVersion()
    void loadSystemStats()
    void loadSystemInfo()
  } else if (tab === 'config' || tab === 'task-exec' || tab === 'proxy' || tab === 'captcha') {
    void loadSystemConfigs()
  } else if (tab === 'backup') {
    void loadBackups()
  } else if (tab === 'security') {
    void load2FAStatus()
  }
}

onMounted(() => {
  void loadVersion()
  void loadSystemStats()
  void loadSystemInfo()
  if (!isAdmin.value) {
    securityTab.value = 'password-2fa'
  }
})
</script>

<template>
  <div class="settings-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">系统设置</h2>
        <span class="page-subtitle">管理员后台：管理面板配置、安全策略和数据备份</span>
      </div>
      <el-button @click="handleRefresh">
        <el-icon><Refresh /></el-icon>刷新
      </el-button>
    </div>

    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="概览" name="overview">
        <OverviewHeroCard
          :is-admin="isAdmin"
          :current-version="currentVersion"
          :update-info="updateInfo"
          :update-status="updateStatus"
          :checking-update="checkingUpdate"
          :updating-panel="updatingPanel"
          :on-check-update="handleCheckUpdate"
          :on-start-update="handleUpdatePanel"
          :on-restart-panel="handleRestartPanel"
          :on-open-git-hub="openGitHub"
        />

        <OverviewStatsCard :system-stats="systemStats" />

        <SystemInfoCard
          :system-info="systemInfo"
          :format-bytes="formatBytes"
          :get-usage-class="getUsageClass"
        />
      </el-tab-pane>

      <el-tab-pane v-if="isAdmin" label="系统配置" name="config">
        <SystemConfigCard
          :configs-loading="configsLoading"
          :configs-saving="configsSaving"
          :form="configForm"
          :on-save="handleSaveSystemConfig"
          :on-icon-upload="handleIconUpload"
          :on-log-background-upload="handleLogBackgroundUpload"
        />
      </el-tab-pane>

      <el-tab-pane v-if="isAdmin" label="任务执行" name="task-exec">
        <TaskExecutionCard
          :configs-loading="configsLoading"
          :configs-saving="configsSaving"
          :form="configForm"
          :on-save="handleSaveTaskConfig"
        />
      </el-tab-pane>

      <el-tab-pane v-if="isAdmin" label="网络代理" name="proxy">
        <ProxyConfigCard
          :configs-saving="configsSaving"
          :form="configForm"
          :on-save="handleSaveProxy"
        />
      </el-tab-pane>

      <el-tab-pane v-if="isAdmin" label="验证码设置" name="captcha">
        <CaptchaConfigCard
          :configs-saving="configsSaving"
          :form="configForm"
          :captcha-feature-implemented="captchaFeatureImplemented"
          :on-save="handleSaveCaptcha"
        />
      </el-tab-pane>

      <el-tab-pane v-if="isAdmin" label="数据备份" name="backup">
        <BackupManagementCard
          v-model:show-backup-dialog="showBackupDialog"
          v-model:backup-password="backupPassword"
          v-model:backup-selection="backupSelection"
          v-model:show-restore-dialog="showRestoreDialog"
          v-model:restore-password="restorePassword"
          :backups="backups"
          :backups-loading="backupsLoading"
          :restore-filename="restoreFilename"
          :on-create-backup="handleCreateBackup"
          :on-upload-backup="handleUploadBackup"
          :on-confirm-create-backup="confirmCreateBackup"
          :on-download-backup="handleDownloadBackup"
          :on-restore-backup="handleRestoreBackup"
          :on-confirm-restore="confirmRestore"
          :on-delete-backup="handleDeleteBackup"
        />
      </el-tab-pane>

      <el-tab-pane label="安全" name="security">
        <el-tabs v-model="securityTab" @tab-change="handleSecurityTabChange">
          <el-tab-pane name="password-2fa">
            <template #label>
              <span class="sub-tab-label"><el-icon :size="14"><Lock /></el-icon>密码与2FA</span>
            </template>
            <SecurityPassword2FACard
              v-model:old-password="oldPassword"
              v-model:new-password="newPassword"
              v-model:confirm-password="confirmPassword"
              v-model:show-setup2-f-a="showSetup2FA"
              v-model:two-f-a-code="twoFACode"
              :two-f-a-enabled="twoFAEnabled"
              :two-f-a-secret="twoFASecret"
              :two-f-a-qr-url="twoFAQrUrl"
              :on-change-password="handleChangePassword"
              :on-setup2-f-a="handleSetup2FA"
              :on-disable2-f-a="handleDisable2FA"
              :on-verify2-f-a="handleVerify2FA"
            />
          </el-tab-pane>

          <el-tab-pane v-if="isAdmin" name="login-logs">
            <template #label>
              <span class="sub-tab-label"><el-icon :size="14"><Document /></el-icon>登录日志</span>
            </template>
            <LoginLogsCard
              v-model:login-logs-page="loginLogsPage"
              :login-logs="loginLogs"
              :login-logs-loading="loginLogsLoading"
              :login-logs-total="loginLogsTotal"
              :on-load-login-logs="loadLoginLogs"
              :on-clear-login-logs="handleClearLoginLogs"
            />
          </el-tab-pane>

          <el-tab-pane v-if="isAdmin" name="sessions">
            <template #label>
              <span class="sub-tab-label"><el-icon :size="14"><Monitor /></el-icon>会话管理</span>
            </template>
            <SessionManagementCard
              :sessions="sessions"
              :sessions-loading="sessionsLoading"
              :on-load-sessions="loadSessions"
              :on-revoke-all-sessions="handleRevokeAllSessions"
              :on-revoke-session="handleRevokeSession"
            />
          </el-tab-pane>

          <el-tab-pane v-if="isAdmin" name="ip-whitelist">
            <template #label>
              <span class="sub-tab-label"><el-icon :size="14"><Connection /></el-icon>IP白名单</span>
            </template>
            <IPWhitelistCard
              v-model:show-add-i-p-dialog="showAddIPDialog"
              v-model:new-i-p="newIP"
              v-model:new-i-p-remarks="newIPRemarks"
              :ip-whitelist="ipWhitelist"
              :ip-whitelist-loading="ipWhitelistLoading"
              :on-load-i-p-whitelist="loadIPWhitelist"
              :on-add-i-p="handleAddIP"
              :on-remove-i-p="handleRemoveIP"
            />
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped lang="scss">
.settings-page {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  color: var(--el-text-color-primary);
}

.page-subtitle {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  display: block;
  margin-top: 2px;
}

.sub-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  :deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
  }

  :deep(.el-tabs__nav-scroll) {
    min-width: max-content;
  }

  :deep(.el-tabs__item) {
    white-space: nowrap;
  }
}
</style>
