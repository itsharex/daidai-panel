<script setup lang="ts">
import { Clock, Delete, Download, Upload } from '@element-plus/icons-vue'
import { ref } from 'vue'
import type { BackupSelection } from '@/api/system'

const showBackupDialog = defineModel<boolean>('showBackupDialog', { required: true })
const backupPassword = defineModel<string>('backupPassword', { required: true })
const backupSelection = defineModel<BackupSelection>('backupSelection', { required: true })
const showRestoreDialog = defineModel<boolean>('showRestoreDialog', { required: true })
const restorePassword = defineModel<string>('restorePassword', { required: true })

defineProps<{
  backups: Array<{ name: string; size: number; created_at: string }>
  backupsLoading: boolean
  restoreFilename: string
  onCreateBackup: () => void | Promise<void>
  onUploadBackup: (event: Event) => void | Promise<void>
  onConfirmCreateBackup: () => void | Promise<void>
  onDownloadBackup: (filename: string) => void | Promise<void>
  onRestoreBackup: (filename: string) => void | Promise<void>
  onConfirmRestore: () => void | Promise<void>
  onDeleteBackup: (filename: string) => void | Promise<void>
}>()

const backupFileInput = ref<HTMLInputElement | null>(null)

function triggerUploadBackup() {
  backupFileInput.value?.click()
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

    <el-table :data="backups" v-loading="backupsLoading" empty-text="暂无备份">
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
  </el-card>

  <el-dialog v-model="showBackupDialog" title="创建备份" width="520px">
    <el-form label-width="100px">
      <el-form-item label="备份内容">
        <div class="backup-selection-grid">
          <el-checkbox v-model="backupSelection.configs">配置项
            <span class="backup-selection-hint">系统设置、Open API、通知渠道与安全配置；恢复时不会覆盖当前面板账号密码</span>
          </el-checkbox>
          <el-checkbox v-model="backupSelection.tasks">定时任务
            <span class="backup-selection-hint">任务定义、标签、执行参数与依赖关系</span>
          </el-checkbox>
          <el-checkbox v-model="backupSelection.subscriptions">订阅管理
            <span class="backup-selection-hint">订阅配置与 SSH 密钥</span>
          </el-checkbox>
          <el-checkbox v-model="backupSelection.env_vars">环境变量
            <span class="backup-selection-hint">面板环境变量与分组信息</span>
          </el-checkbox>
          <el-checkbox v-model="backupSelection.logs">日志文件
            <span class="backup-selection-hint">任务日志记录、日志目录与面板运行日志</span>
          </el-checkbox>
          <el-checkbox v-model="backupSelection.scripts">脚本文件
            <span class="backup-selection-hint">脚本目录内的源码、资源和可执行文件</span>
          </el-checkbox>
          <el-checkbox v-model="backupSelection.dependencies">依赖记录
            <span class="backup-selection-hint">记录已安装依赖，恢复时按记录重新安装</span>
          </el-checkbox>
        </div>
      </el-form-item>
      <el-form-item label="备份密码">
        <el-input v-model="backupPassword" type="password" placeholder="可选，留空则不加密" show-password />
      </el-form-item>
      <el-alert type="info" :closable="false" show-icon>
        创建的备份默认导出为 `.tgz`，设置密码后会加密为 `.enc`
      </el-alert>
    </el-form>
    <template #footer>
      <el-button @click="showBackupDialog = false">取消</el-button>
      <el-button type="primary" @click="onConfirmCreateBackup">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="showRestoreDialog" title="恢复备份" width="400px">
    <el-form label-width="100px">
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
}

.backup-selection-hint {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
  margin-left: 24px;
}
</style>
