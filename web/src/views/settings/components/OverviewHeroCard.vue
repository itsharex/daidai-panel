<script setup lang="ts">
import { Refresh } from '@element-plus/icons-vue'
import type { PanelUpdateStatus } from '@/api/system'
import UpdateProgressDialog from './UpdateProgressDialog.vue'

defineProps<{
  isAdmin: boolean
  currentVersion: string
  updateInfo: any
  updateStatus: PanelUpdateStatus | null
  checkingUpdate: boolean
  updatingPanel: boolean
  autoUpdateEnabled: boolean
  savingAutoUpdate: boolean
  releaseNotesVisible: boolean
  updateProgressVisible: boolean
  updateProgressStatus: 'idle' | 'running' | 'restarting' | 'failed' | 'timeout'
  updateProgressError: string
  onCheckUpdate: () => void | Promise<void>
  onStartUpdate: () => void | Promise<void>
  onRestartPanel: () => void | Promise<void>
  onToggleAutoUpdate: (value: boolean) => void | Promise<void>
  onOpenReleaseNotes: () => void | Promise<void>
  onCloseReleaseNotes: () => void | Promise<void>
  onOpenGitHub: () => void
  onCloseUpdateProgress: () => void | Promise<void>
}>()
</script>

<template>
  <el-card shadow="never" class="overview-card">
    <div class="overview-center">
      <div class="logo">
        <img src="/favicon.svg" alt="呆呆面板" class="logo-img" />
      </div>
      <h2 class="panel-name">呆呆面板</h2>
      <p class="panel-desc">轻量级定时任务管理面板</p>
      <div class="version-stats">
        <div class="version-item">
          <span class="vs-label">系统版本</span>
          <span class="vs-value">{{ currentVersion }}</span>
        </div>
        <div class="version-item">
          <span class="vs-label">技术栈</span>
          <span class="vs-value">Gin + Vue3</span>
        </div>
      </div>
      <div class="overview-buttons">
        <el-button v-if="isAdmin" type="primary" :loading="checkingUpdate" @click="onCheckUpdate">
          <el-icon><Refresh /></el-icon>检查系统更新
        </el-button>
        <el-button v-if="isAdmin" type="warning" @click="onRestartPanel">
          <el-icon><Refresh /></el-icon>重启面板
        </el-button>
        <el-button @click="onOpenGitHub">
          <svg viewBox="0 0 16 16" width="16" height="16" style="margin-right: 4px; vertical-align: middle; fill: currentColor">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
          </svg>
          GitHub
        </el-button>
      </div>
      <div v-if="isAdmin" class="auto-update-panel">
        <div class="auto-update-copy">
          <strong>静默更新</strong>
          <span>每 24 小时自动检查一次新版本；检测到更新后会按当前镜像渠道自动尝试更新，并通过通知渠道反馈结果。</span>
        </div>
        <el-switch
          :model-value="autoUpdateEnabled"
          :loading="savingAutoUpdate"
          inline-prompt
          active-text="开"
          inactive-text="关"
          @change="onToggleAutoUpdate"
        />
      </div>
      <div v-if="updateInfo" class="update-alert-wrap">
        <el-alert
          :type="updateInfo.has_update ? (updateInfo.auto_update_supported ? 'success' : 'warning') : 'info'"
          :title="updateInfo.has_update ? `发现新版本 v${updateInfo.latest}` : '当前已是最新版本'"
          :closable="false"
        >
          <div v-if="updateInfo.has_update">
            <p>发布时间: {{ new Date(updateInfo.published_at).toLocaleString() }}</p>
            <p v-if="updateInfo.update_target?.mirror_host" class="update-meta">
              系统更新镜像源：{{ updateInfo.update_target.mirror_host }}
            </p>
            <p v-if="updateInfo.update_target?.channel" class="update-meta">
              更新渠道：{{ updateInfo.update_target.channel === 'debian' ? 'Debian' : 'Latest (Alpine)' }}
            </p>
            <p
              v-if="updateInfo.update_target?.pull_image_name && updateInfo.update_target.pull_image_name !== updateInfo.update_target.image_name"
              class="update-meta"
            >
              拉取目标：{{ updateInfo.update_target.pull_image_name }}
            </p>
            <p v-if="!updateInfo.auto_update_supported" class="update-disabled-reason">
              当前部署暂不支持面板内一键更新：{{ updateInfo.update_disabled_reason || '请改用手动更新' }}
            </p>
            <div class="update-actions">
              <el-button
                v-if="isAdmin && updateInfo.auto_update_supported"
                type="primary"
                size="small"
                :loading="updatingPanel"
                @click="onStartUpdate"
              >
                立即更新
              </el-button>
              <el-button size="small" @click="onOpenReleaseNotes">查看更新日志</el-button>
            </div>
          </div>
        </el-alert>
      </div>
      <div v-if="updateStatus && updateStatus.status && updateStatus.status !== 'idle'" class="update-alert-wrap">
        <el-alert
          :type="updateStatus.status === 'failed' ? 'error' : (updateStatus.status === 'restarting' ? 'success' : 'warning')"
          :title="updateStatus.status === 'failed' ? '更新失败' : (updateStatus.status === 'restarting' ? '正在切换到新版本' : '更新进行中')"
          :closable="false"
        >
          <p>{{ updateStatus.message }}</p>
          <p v-if="updateStatus.container_name || updateStatus.image_name" class="update-meta">
            {{ updateStatus.container_name || '-' }} / {{ updateStatus.image_name || '-' }}
          </p>
          <p v-if="updateStatus.mirror_host" class="update-meta">
            系统更新镜像源：{{ updateStatus.mirror_host }}
          </p>
        </el-alert>
      </div>
    </div>
  </el-card>

  <UpdateProgressDialog
    :visible="updateProgressVisible"
    :current-version="currentVersion"
    :latest-version="updateInfo?.latest"
    :release-url="updateInfo?.release_url"
    :status="updateProgressStatus"
    :update-status="updateStatus"
    :error-message="updateProgressError"
    :on-close="onCloseUpdateProgress"
  />

  <el-dialog
    :model-value="releaseNotesVisible"
    title="发现新版本"
    width="720px"
    append-to-body
    @close="onCloseReleaseNotes"
  >
    <div v-if="updateInfo" class="release-notes-shell">
      <div class="release-notes-meta">
        <strong>版本：v{{ updateInfo.latest }}</strong>
        <span v-if="updateInfo.release_name">{{ updateInfo.release_name }}</span>
        <span v-if="updateInfo.published_at">发布时间：{{ new Date(updateInfo.published_at).toLocaleString() }}</span>
      </div>
      <pre class="release-notes-content">{{ updateInfo.release_notes || '当前版本未提供更新日志。' }}</pre>
    </div>
    <template #footer>
      <div class="release-notes-footer">
        <el-button @click="onCloseReleaseNotes">关闭</el-button>
        <el-button v-if="isAdmin && updateInfo?.auto_update_supported" type="primary" :loading="updatingPanel" @click="onStartUpdate">
          立即更新
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped lang="scss">
.overview-card {
  :deep(.el-card__body) {
    padding: 0;
  }
}

.overview-center {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 20px;
}

.logo {
  width: 72px;
  height: 72px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  margin-bottom: 16px;
  overflow: hidden;
}

.logo-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 18px;
}

.panel-name {
  font-size: 22px;
  font-weight: 700;
  margin: 0 0 4px;
}

.panel-desc {
  color: var(--el-text-color-secondary);
  font-size: 14px;
  margin: 0 0 28px;
}

.version-stats {
  display: flex;
  gap: 80px;
  margin-bottom: 28px;
}

.version-item {
  text-align: center;
}

.vs-label {
  display: block;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}

.vs-value {
  font-size: 22px;
  font-weight: 700;
}

.overview-buttons {
  display: flex;
  gap: 12px;
}

.auto-update-panel {
  width: 100%;
  max-width: 560px;
  margin-top: 18px;
  padding: 16px 18px;
  border-radius: 18px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background:
    linear-gradient(180deg, rgba(14, 165, 233, 0.06), rgba(255, 255, 255, 0.9)),
    var(--el-bg-color);
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.06);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.auto-update-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;

  strong {
    font-size: 14px;
    font-weight: 700;
  }

  span {
    color: var(--el-text-color-secondary);
    font-size: 12px;
    line-height: 1.6;
  }
}

.update-alert-wrap {
  margin-top: 20px;
  width: 100%;
  max-width: 500px;
}

.update-actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.update-disabled-reason,
.update-meta {
  color: var(--el-text-color-secondary);
}

.release-notes-shell {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.release-notes-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  color: var(--el-text-color-secondary);
}

.release-notes-content {
  margin: 0;
  max-height: 46vh;
  overflow: auto;
  padding: 16px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--el-fill-color-light) 82%, white);
  border: 1px solid var(--el-border-color-light);
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.65;
  font-family: var(--dd-font-mono);
  font-size: 13px;
}

.release-notes-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 768px) {
  .version-stats {
    width: 100%;
    gap: 20px;
    justify-content: space-between;
  }

  .overview-buttons {
    width: 100%;
    flex-direction: column;
  }

  .auto-update-panel {
    flex-direction: column;
    align-items: stretch;
  }

  .update-actions {
    flex-wrap: wrap;
  }
}
</style>
