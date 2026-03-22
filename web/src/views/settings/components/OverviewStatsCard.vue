<script setup lang="ts">
import { InfoFilled } from '@element-plus/icons-vue'

defineProps<{
  systemStats: any
}>()
</script>

<template>
  <el-card shadow="never" class="mt-card" v-if="systemStats">
    <template #header>
      <div class="card-header">
        <span class="card-title"><el-icon><InfoFilled /></el-icon> 面板概况</span>
      </div>
    </template>
    <div class="overview-stats-row">
      <div class="os-item">
        <div class="os-label">任务总数</div>
        <div class="os-value">{{ systemStats.tasks?.total || 0 }}</div>
      </div>
      <div class="os-item">
        <div class="os-label">已启用</div>
        <div class="os-value color-success">{{ systemStats.tasks?.enabled || 0 }}</div>
      </div>
      <div class="os-item">
        <div class="os-label">运行中</div>
        <div class="os-value color-warning">{{ systemStats.tasks?.running || 0 }}</div>
      </div>
      <div class="os-item">
        <div class="os-label">执行日志</div>
        <div class="os-value">{{ systemStats.logs?.total || 0 }}</div>
      </div>
      <div class="os-item">
        <div class="os-label">成功率</div>
        <div class="os-value color-success">{{ (systemStats.logs?.success_rate || 0).toFixed(1) }}%</div>
      </div>
      <div class="os-item">
        <div class="os-label">脚本数</div>
        <div class="os-value">{{ systemStats.scripts?.total || 0 }}</div>
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
@use './config-card-shared.scss' as *;

.mt-card {
  margin-top: 16px;
}

.overview-stats-row {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  text-align: center;
}

.os-item {
  padding: 16px 0;
}

.os-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 10px;
}

.os-value {
  font-size: 26px;
  font-weight: 700;
}

.color-success {
  color: var(--el-color-success);
}

.color-warning {
  color: var(--el-color-warning);
}

@media (max-width: 768px) {
  .overview-stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 420px) {
  .overview-stats-row {
    grid-template-columns: 1fr;
  }
}
</style>
