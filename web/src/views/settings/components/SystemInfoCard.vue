<script setup lang="ts">
import { InfoFilled } from '@element-plus/icons-vue'

defineProps<{
  systemInfo: any
  formatBytes: (bytes: number) => string
  getUsageClass: (percent: number) => string
}>()
</script>

<template>
  <el-card shadow="never" class="mt-card" v-if="systemInfo">
    <template #header>
      <div class="card-header">
        <span class="card-title"><el-icon><InfoFilled /></el-icon> 系统信息</span>
      </div>
    </template>
    <div class="system-info-grid">
      <div class="si-item">
        <div class="si-label">主机名</div>
        <div class="si-value">{{ systemInfo.hostname || '-' }}</div>
      </div>
      <div class="si-item">
        <div class="si-label">操作系统</div>
        <div class="si-value">{{ systemInfo.os || '-' }} {{ systemInfo.arch || '' }}</div>
      </div>
      <div class="si-item">
        <div class="si-label">Go</div>
        <div class="si-value">{{ systemInfo.go_version || '-' }}</div>
      </div>
      <div class="si-item">
        <div class="si-label">数据目录</div>
        <div class="si-value">{{ systemInfo.data_dir || '-' }}</div>
      </div>
      <div class="si-item">
        <div class="si-label">CPU 使用率</div>
        <div class="si-value" :class="getUsageClass(systemInfo.cpu_usage)">
          {{ systemInfo.cpu_usage || 0 }}%&nbsp;&nbsp;({{ systemInfo.num_cpu || 0 }} 核)
        </div>
      </div>
      <div class="si-item">
        <div class="si-label">内存使用</div>
        <div class="si-value" :class="getUsageClass(systemInfo.memory_usage)">
          {{ systemInfo.memory_usage || 0 }}%&nbsp;&nbsp;({{ formatBytes(systemInfo.memory_used) }} / {{ formatBytes(systemInfo.memory_total) }})
        </div>
      </div>
      <div class="si-item">
        <div class="si-label">磁盘使用</div>
        <div class="si-value" :class="getUsageClass(systemInfo.disk_usage)">
          {{ systemInfo.disk_usage || 0 }}%&nbsp;&nbsp;({{ formatBytes(systemInfo.disk_used) }} / {{ formatBytes(systemInfo.disk_total) }})
        </div>
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
@use './config-card-shared.scss' as *;

.mt-card {
  margin-top: 16px;
}

.system-info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.si-item {
  padding: 4px 0;
}

.si-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}

.si-value {
  font-size: 14px;
  font-weight: 600;
  word-break: break-word;
}

.usage-success {
  color: var(--el-color-success);
}

.usage-warning {
  color: var(--el-color-warning);
}

.usage-danger {
  color: var(--el-color-danger);
}

@media (max-width: 768px) {
  .system-info-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }
}
</style>
