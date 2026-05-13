<script setup lang="ts">
import { Clock, Delete, Download, Upload } from '@element-plus/icons-vue'
import { ref } from 'vue'
import type { BackupSelection } from '@/api/system'
import { useResponsive } from '@/composables/useResponsive'
import RestoreProgressDialog from './RestoreProgressDialog.vue'

const showBackupDialog = defineModel<boolean>('showBackupDialog', { required: true })
const backupName = defineModel<string>('backupName', { required: true })
const backupPassword = defineModel<string>('backupPassword', { required: true })
const backupSelection = defineModel<BackupSelection>('backupSelection', { required: true })
const backupScheduleSelection = defineModel<BackupSelection>('backupScheduleSelection', { required: true })
const showRestoreDialog = defineModel<boolean>('showRestoreDialog', { required: true })
const restorePassword = defineModel<string>('restorePassword', { required: true })

defineProps<{
  settingsForm: {
    backup_schedule_enabled: boolean
    backup_schedule_frequency: 'daily' | 'weekly' | 'monthly' | string
    backup_schedule_time: string
    backup_schedule_weekday: string
    backup_schedule_monthday: number
    backup_schedule_name: string
    backup_schedule_password: string
  }
  configsSaving: boolean
  backups: Array<{ name: string; size: number; created_at: string }>
  backupsLoading: boolean
  restoreFilename: string
  restoreProgressVisible: boolean
  restoreProgressStatus: string
  restoreProgressStage: string
  restoreProgressMessage: string
  restoreProgressPercent: number
  restoreProgressSource: string
  restoreProgressSelection: Partial<BackupSelection>
  restoreProgressStartedAt?: string
  restoreRestartCountdown: number
  restoreProgressError: string
  onCreateBackup: () => void | Promise<void>
  onUploadBackup: (event: Event) => void | Promise<void>
  onSaveSchedule: () => void | Promise<void>
  onConfirmCreateBackup: () => void | Promise<void>
  onDownloadBackup: (filename: string) => void | Promise<void>
  onRestoreBackup: (filename: string) => void | Promise<void>
  onConfirmRestore: () => void | Promise<void>
  onCloseRestoreProgress: () => void | Promise<void>
  onRestartRestoreNow: () => void | Promise<void>
  onDeleteBackup: (filename: string) => void | Promise<void>
}>()

const backupFileInput = ref<HTMLInputElement | null>(null)
const { isMobile, dialogFullscreen } = useResponsive()

const backupSelectionOptions: Array<{ key: keyof BackupSelection; title: string; description: string }> = [
  {
    key: 'configs',
    title: '配置项',
    description: '系统设置、Open API、通知渠道与安全配置；恢复时不会覆盖当前面板账号密码',
  },
  {
    key: 'tasks',
    title: '定时任务',
    description: '任务定义、标签、执行参数与依赖关系',
  },
  {
    key: 'subscriptions',
    title: '订阅管理',
    description: '订阅配置与 SSH 密钥',
  },
  {
    key: 'env_vars',
    title: '环境变量',
    description: '面板环境变量与分组信息',
  },
  {
    key: 'logs',
    title: '日志文件',
    description: '任务日志记录、日志目录与面板运行日志',
  },
  {
    key: 'scripts',
    title: '脚本文件',
    description: '脚本目录内的源码、资源和可执行文件',
  },
  {
    key: 'dependencies',
    title: '依赖记录',
    description: '记录已安装依赖，恢复时按记录重新安装',
  },
]

function triggerUploadBackup() {
  backupFileInput.value?.click()
}

function updateBackupSelection(key: keyof BackupSelection, value: boolean) {
  backupSelection.value = {
    ...backupSelection.value,
    [key]: value
  }
}

function updateBackupScheduleSelection(key: keyof BackupSelection, value: boolean) {
  backupScheduleSelection.value = {
    ...backupScheduleSelection.value,
    [key]: value
  }
}
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="card-header">
        <span class="card-title"><el-icon><Clock /></el-icon> 数据备份与恢复</span>
        <div class="card-header-buttons">
          <el-button @click="triggerUploadBackup">
            <el-icon><Download /></el-icon>导入备份
          </el-button>
          <el-button type="primary" @click="onCreateBackup">
            <el-icon><Upload /></el-icon>创建备份
          </el-button>
          <input ref="backupFileInput" type="file" accept=".json,.enc,.tgz,.tar.gz" style="display: none" @change="onUploadBackup" />
        </div>
      </div>
    </template>

    <div v-if="isMobile" class="dd-mobile-list">
      <div
        v-for="row in backups"
        :key="row.name"
        class="dd-mobile-card"
      >
        <div class="dd-mobile-card__header">
          <div class="dd-mobile-card__title-wrap">
            <span class="dd-mobile-card__title">{{ row.name }}</span>
            <span class="dd-mobile-card__subtitle">{{ new Date(row.created_at).toLocaleString() }}</span>
          </div>
        </div>
        <div class="dd-mobile-card__body">
          <div class="dd-mobile-card__grid">
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">大小</span>
              <span class="dd-mobile-card__value">{{ (row.size / 1024).toFixed(2) }} KB</span>
            </div>
          </div>
          <div class="dd-mobile-card__actions backup-actions">
            <el-button size="small" type="primary" plain @click="onDownloadBackup(row.name)">下载</el-button>
            <el-button size="small" type="success" plain @click="onRestoreBackup(row.name)">恢复</el-button>
            <el-button size="small" type="danger" plain @click="onDeleteBackup(row.name)">删除</el-button>
          </div>
        </div>
      </div>
      <el-empty v-if="!backupsLoading && backups.length === 0" description="暂无备份" />
    </div>

    <el-table v-else :data="backups" v-loading="backupsLoading" empty-text="暂无备份">
      <el-table-column prop="name" label="文件名" min-width="200" />
      <el-table-column label="大小" width="120">
        <template #default="{ row }">{{ (row.size / 1024).toFixed(2) }} KB</template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="170">
        <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right" align="center">
        <template #default="{ row }">
          <div class="backup-actions">
            <el-button size="small" type="primary" plain @click="onDownloadBackup(row.name)">
              <el-icon><Download /></el-icon>下载
            </el-button>
            <el-button size="small" type="success" plain @click="onRestoreBackup(row.name)">
              <el-icon><Upload /></el-icon>恢复
            </el-button>
            <el-button size="small" type="danger" plain @click="onDeleteBackup(row.name)">
              <el-icon><Delete /></el-icon>删除
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <el-alert type="info" :closable="false" show-icon style="margin-top: 16px">
      支持导入呆呆面板备份（`.tgz` / `.enc` / 旧版 `.json`）以及青龙面板导出的 `.tgz` 备份包
    </el-alert>

    <el-divider />

    <section class="schedule-section">
      <div class="schedule-header">
        <div>
          <h4>定时备份</h4>
          <p>按每天、每周或每月自动创建面板备份，便于长期保留和灾备恢复。</p>
        </div>
        <el-button type="primary" :loading="configsSaving" @click="onSaveSchedule">
          保存定时配置
        </el-button>
      </div>

      <div class="schedule-grid">
        <div class="schedule-field schedule-field--switch">
          <label>启用定时备份</label>
          <el-switch v-model="settingsForm.backup_schedule_enabled" inline-prompt active-text="开" inactive-text="关" />
        </div>
        <div class="schedule-field">
          <label>备份频率</label>
          <el-select v-model="settingsForm.backup_schedule_frequency" style="width: 100%">
            <el-option label="每天" value="daily" />
            <el-option label="每周" value="weekly" />
            <el-option label="每月" value="monthly" />
          </el-select>
        </div>
        <div class="schedule-field">
          <label>执行时间</label>
          <el-time-select
            v-model="settingsForm.backup_schedule_time"
            style="width: 100%"
            start="00:00"
            step="00:30"
            end="23:30"
            placeholder="选择执行时间"
          />
        </div>
        <div v-if="settingsForm.backup_schedule_frequency === 'weekly'" class="schedule-field">
          <label>每周执行日</label>
          <el-select v-model="settingsForm.backup_schedule_weekday" style="width: 100%">
            <el-option label="周日" value="0" />
            <el-option label="周一" value="1" />
            <el-option label="周二" value="2" />
            <el-option label="周三" value="3" />
            <el-option label="周四" value="4" />
            <el-option label="周五" value="5" />
            <el-option label="周六" value="6" />
          </el-select>
        </div>
        <div v-if="settingsForm.backup_schedule_frequency === 'monthly'" class="schedule-field">
          <label>每月执行日</label>
          <el-input-number v-model="settingsForm.backup_schedule_monthday" :min="1" :max="28" style="width: 100%" />
        </div>
        <div class="schedule-field">
          <label>文件名前缀</label>
          <el-input v-model="settingsForm.backup_schedule_name" placeholder="可选，例如：daily-auto-backup" />
        </div>
        <div class="schedule-field">
          <label>加密密码</label>
          <el-input v-model="settingsForm.backup_schedule_password" type="password" show-password placeholder="可选，留空则不加密" />
        </div>
      </div>

      <div class="schedule-selection">
        <label class="schedule-selection-title">定时备份内容</label>
        <div class="backup-selection-grid">
          <label
            v-for="option in backupSelectionOptions"
            :key="`schedule-${option.key}`"
            class="backup-selection-card"
            :class="{ 'is-active': backupScheduleSelection[option.key] }"
          >
            <el-checkbox
              :model-value="backupScheduleSelection[option.key]"
              @update:model-value="updateBackupScheduleSelection(option.key, Boolean($event))"
            >
              {{ option.title }}
            </el-checkbox>
            <span class="backup-selection-hint">{{ option.description }}</span>
          </label>
        </div>
      </div>
    </section>
  </el-card>

  <el-dialog v-model="showBackupDialog" title="创建备份" width="520px" :fullscreen="dialogFullscreen">
    <el-form :label-width="dialogFullscreen ? 'auto' : '100px'" :label-position="dialogFullscreen ? 'top' : 'right'">
      <el-form-item label="备份内容">
        <div class="backup-selection-grid">
          <label
            v-for="option in backupSelectionOptions"
            :key="option.key"
            class="backup-selection-card"
            :class="{ 'is-active': backupSelection[option.key] }"
          >
            <el-checkbox
              :model-value="backupSelection[option.key]"
              @update:model-value="updateBackupSelection(option.key, Boolean($event))"
            >
              {{ option.title }}
            </el-checkbox>
            <span class="backup-selection-hint">{{ option.description }}</span>
          </label>
        </div>
      </el-form-item>
      <el-form-item label="备份密码">
        <el-input v-model="backupPassword" type="password" placeholder="可选，留空则不加密" show-password />
      </el-form-item>
      <el-form-item label="备份文件名">
        <el-input v-model="backupName" placeholder="可选，例如：周五全量备份" />
      </el-form-item>
      <el-alert type="info" :closable="false" show-icon>
        创建的备份默认导出为 `.tgz`，设置密码后会加密为 `.enc`；如果填写名称，系统会自动补全正确扩展名。
      </el-alert>
    </el-form>
    <template #footer>
      <el-button @click="showBackupDialog = false">取消</el-button>
      <el-button type="primary" @click="onConfirmCreateBackup">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="showRestoreDialog" title="恢复备份" width="400px" :fullscreen="dialogFullscreen">
    <el-form :label-width="dialogFullscreen ? 'auto' : '100px'" :label-position="dialogFullscreen ? 'top' : 'right'">
      <el-form-item label="备份文件">
        <el-input :model-value="restoreFilename" disabled />
      </el-form-item>
      <el-form-item v-if="restoreFilename.endsWith('.enc')" label="备份密码">
        <el-input v-model="restorePassword" type="password" placeholder="请输入备份密码" show-password />
      </el-form-item>
      <el-alert type="warning" :closable="false" show-icon>
        恢复将覆盖当前数据，请谨慎操作！
      </el-alert>
    </el-form>
    <template #footer>
      <el-button @click="showRestoreDialog = false">取消</el-button>
      <el-button type="danger" @click="onConfirmRestore">确认恢复</el-button>
    </template>
  </el-dialog>

  <RestoreProgressDialog
    :visible="restoreProgressVisible"
    :fullscreen="dialogFullscreen"
    :filename="restoreFilename"
    :status="restoreProgressStatus"
    :stage="restoreProgressStage"
    :message="restoreProgressMessage"
    :percent="restoreProgressPercent"
    :source="restoreProgressSource"
    :selection="restoreProgressSelection"
    :started-at="restoreProgressStartedAt"
    :countdown="restoreRestartCountdown"
    :error-message="restoreProgressError"
    :on-close="onCloseRestoreProgress"
    :on-restart-now="onRestartRestoreNow"
  />
</template>

<style scoped lang="scss">
@use './config-card-shared.scss' as *;

.card-header-buttons,
.backup-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.backup-actions {
  justify-content: center;
}

.backup-selection-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.backup-selection-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 14px;
  background:
    linear-gradient(180deg, rgba(59, 130, 246, 0.03), rgba(15, 23, 42, 0)),
    var(--el-fill-color-extra-light);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
  cursor: pointer;

  &:hover {
    border-color: rgba(59, 130, 246, 0.35);
    box-shadow: 0 10px 22px rgba(15, 23, 42, 0.08);
    transform: translateY(-1px);
  }

  &.is-active {
    border-color: rgba(59, 130, 246, 0.48);
    background:
      linear-gradient(180deg, rgba(59, 130, 246, 0.08), rgba(59, 130, 246, 0.02)),
      var(--el-bg-color);
    box-shadow: 0 10px 24px rgba(59, 130, 246, 0.12);
  }

  :deep(.el-checkbox) {
    align-items: flex-start;
    line-height: 1.4;
  }

  :deep(.el-checkbox__label) {
    font-weight: 600;
    color: var(--el-text-color-primary);
    padding-left: 10px;
  }
}

.backup-selection-hint {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
  margin-left: 26px;
}

.schedule-section {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.schedule-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;

  h4 {
    margin: 0 0 6px;
    font-size: 16px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  p {
    margin: 0;
    font-size: 13px;
    line-height: 1.7;
    color: var(--el-text-color-secondary);
  }
}

.schedule-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px 18px;
}

.schedule-field {
  display: flex;
  flex-direction: column;
  gap: 8px;

  label {
    font-size: 13px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }
}

.schedule-field--switch {
  justify-content: flex-end;
}

.schedule-selection {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.schedule-selection-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

@media (max-width: 768px) {
  .card-header-buttons {
    width: 100%;
  }

  .backup-selection-grid {
    grid-template-columns: 1fr;
  }

  .schedule-grid {
    grid-template-columns: 1fr;
  }
}
</style>
