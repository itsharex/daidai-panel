<script setup lang="ts">
import { ref } from 'vue'
import { systemApi } from '@/api/system'
import { ElMessage } from 'element-plus'
import { Refresh, CircleCheckFilled, CircleCloseFilled, WarningFilled } from '@element-plus/icons-vue'

const checking = ref(false)
const checked = ref(false)
const healthItems = ref<Array<{ name: string; status: string; message?: string }>>([])

type HealthItem = { name: string; status: string; message?: string }

const nameMap: Record<string, string> = {
  database: '数据库连接',
  memory: '内存使用',
  scheduler: '定时任务调度',
  network: '外部网络连接',
}

async function handleCheck() {
  checking.value = true
  try {
    const res = await systemApi.healthCheck()
    const items: HealthItem[] = (res as any).data?.items || (res as any).items || []
    healthItems.value = items.map(i => ({
      name: nameMap[i.name] || i.name,
      status: i.status,
      message: i.message,
    }))
    checked.value = true
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '健康检查失败')
  } finally {
    checking.value = false
  }
}

function isOk(status: string) {
  return status === 'ok' || status === 'normal' || status === '正常'
}

function isWarning(status: string) {
  return status === 'warning' || status === 'warn' || status === '警告'
}

function statusLabel(status: string) {
  if (isOk(status)) return '正常'
  if (isWarning(status)) return '警告'
  return '异常'
}
</script>

<template>
  <el-card shadow="never" class="health-card">
    <template #header>
      <div class="card-header">
        <span class="card-title"><el-icon><CircleCheckFilled /></el-icon> 系统健康</span>
        <el-button size="small" :loading="checking" @click="handleCheck">
          <el-icon><Refresh /></el-icon> 立即检查
        </el-button>
      </div>
    </template>

    <div v-if="!checked" class="health-placeholder">
      点击「立即检查」执行一次系统健康检测
    </div>

    <div v-else class="health-list">
      <div v-for="item in healthItems" :key="item.name" class="health-item">
        <div class="health-item__icon">
          <el-icon :size="18" :color="isOk(item.status) ? '#10b981' : (isWarning(item.status) ? '#f59e0b' : '#ef4444')">
            <CircleCheckFilled v-if="isOk(item.status)" />
            <WarningFilled v-else-if="isWarning(item.status)" />
            <CircleCloseFilled v-else />
          </el-icon>
        </div>
        <div class="health-item__body">
          <div class="health-item__name">{{ item.name }}</div>
          <div class="health-item__status" :class="{ 'is-ok': isOk(item.status), 'is-warning': isWarning(item.status), 'is-error': !isOk(item.status) && !isWarning(item.status) }">
            {{ statusLabel(item.status) }}
          </div>
        </div>
        <div v-if="item.message" class="health-item__message">{{ item.message }}</div>
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
@use './config-card-shared.scss' as *;

.health-card {
  border-radius: 12px;
  border: 1px solid #f0f0f0;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  height: 100%;
}

.health-placeholder {
  text-align: center;
  padding: 32px 16px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.health-list {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.health-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px dashed var(--el-border-color-lighter);

  &:last-child {
    border-bottom: none;
    padding-bottom: 4px;
  }

  &:first-child {
    padding-top: 0;
  }
}

.health-item__icon {
  flex-shrink: 0;
}

.health-item__body {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.health-item__name {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.health-item__status {
  font-size: 12px;
  font-weight: 600;
  padding: 1px 8px;
  border-radius: 999px;

  &.is-ok {
    color: #10b981;
    background: rgba(16, 185, 129, 0.1);
  }

  &.is-warning {
    color: #f59e0b;
    background: rgba(245, 158, 11, 0.12);
  }

  &.is-error {
    color: #ef4444;
    background: rgba(239, 68, 68, 0.1);
  }
}

.health-item__message {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}
</style>
