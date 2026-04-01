<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { taskApi } from '@/api/task'
import { openAuthorizedEventStream, type EventStreamConnection } from '@/utils/sse'
import { useResponsive } from '@/composables/useResponsive'

const props = defineProps<{
  visible: boolean
  taskId: number | null
  taskName: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const logLines = ref<string[]>([])
const logTail = ref('')
const done = ref(false)
const error = ref<string | null>(null)
const loading = ref(false)
const logContainerRef = ref<HTMLElement>()
const autoScroll = ref(false)
const { dialogFullscreen } = useResponsive()
let eventSource: EventStreamConnection | null = null
let logBuffer: string[] = []
let logFlushTimer: ReturnType<typeof setTimeout> | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let hiddenAt: number | null = null

const hasLogs = computed(() => logLines.value.length > 0 || logTail.value.length > 0)
const renderedLogText = computed(() => {
  const lines = [...logLines.value]
  if (logTail.value !== '' || lines.length === 0) {
    lines.push(logTail.value)
  }
  return lines.join('\n')
})

watch(() => props.visible, (visible) => {
  if (visible && props.taskId) {
    void startStream()
  } else {
    cleanup()
  }
})

watch(() => props.taskId, (taskId, previousTaskId) => {
  if (props.visible && taskId && taskId !== previousTaskId) {
    void startStream()
  }
})

watch(autoScroll, (enabled) => {
  if (enabled) {
    scheduleScrollToBottom()
  }
})

async function startStream() {
  cleanup()
  resetLogOutput()
  done.value = false
  error.value = null
  loading.value = true
  autoScroll.value = false
  scheduleScrollToTop()

  if (!props.taskId) {
    loading.value = false
    return
  }

  const url = `/api/v1/logs/${props.taskId}/stream`
  eventSource = openAuthorizedEventStream(url, {
    onOpen() {
      loading.value = false
    },
    onMessage(data) {
      loading.value = false
      if (!data) {
        return
      }
      logBuffer.push(data)
      scheduleBufferFlush()
    },
    onEvent(event) {
      if (event.event !== 'done') {
        return
      }
      flushBufferedLogs()
      done.value = true
      cleanup()
      if (event.data === 'reconnect') {
        reconnectTimer = setTimeout(() => {
          reconnectTimer = null
          void startStream()
        }, 500)
        return
      }
      if (!hasLogs.value) {
        void fetchLatestLog()
      }
    },
    onError() {
      flushBufferedLogs()
      loading.value = false
      done.value = true
      cleanup()
      if (!hasLogs.value) {
        void fetchLatestLog()
      }
    }
  })
}

async function fetchLatestLog(retryCount = 0) {
  try {
    const res = await taskApi.latestLog(props.taskId!) as any
    if (!res) {
      error.value = '暂无日志记录'
      return
    }
    if (res.content) {
      resetLogOutput()
      appendLogChunk(String(res.content))
      scheduleScrollToTop()
    } else {
      error.value = '日志已过期，文件已被清理'
    }
  } catch (err: any) {
    if (err?.response?.status === 404) {
      if (retryCount < 5 && props.visible) {
        reconnectTimer = setTimeout(() => {
          reconnectTimer = null
          void fetchLatestLog(retryCount + 1)
        }, 500)
        return
      }
      error.value = '暂无日志记录'
    } else {
      error.value = '获取日志失败'
    }
  }
}

function resetLogOutput() {
  logLines.value = []
  logTail.value = ''
}

function pushLogLine() {
  logLines.value.push(logTail.value)
  logTail.value = ''
}

function appendLogChunk(chunk: string, commitBoundary = false) {
  if (!chunk && !commitBoundary) return

  let endedWithLineBreak = false
  for (let i = 0; i < chunk.length; i++) {
    const char = chunk[i]
    if (char === '\r') {
      if (chunk[i + 1] === '\n') {
        pushLogLine()
        endedWithLineBreak = true
        i++
        continue
      }
      logTail.value = ''
      endedWithLineBreak = false
      continue
    }

    if (char === '\n') {
      pushLogLine()
      endedWithLineBreak = true
      continue
    }

    logTail.value += char
    endedWithLineBreak = false
  }

  if (commitBoundary && !endedWithLineBreak) {
    pushLogLine()
  }
}

function scheduleBufferFlush() {
  if (logFlushTimer !== null) {
    return
  }
  logFlushTimer = setTimeout(() => {
    logFlushTimer = null
    flushBufferedLogs()
  }, 16)
}

function flushBufferedLogs() {
  if (logFlushTimer !== null) {
    clearTimeout(logFlushTimer)
    logFlushTimer = null
  }
  if (logBuffer.length === 0) {
    return
  }

  for (const chunk of logBuffer) {
    appendLogChunk(chunk, true)
  }
  logBuffer = []

  if (autoScroll.value) {
    scheduleScrollToBottom()
  }
}

function scheduleScrollToBottom() {
  void nextTick(() => {
    scrollToBottom()
  })
}

function scheduleScrollToTop() {
  void nextTick(() => {
    scrollToTop()
  })
}

function scrollToBottom() {
  if (logContainerRef.value) {
    logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
  }
}

function scrollToTop() {
  if (logContainerRef.value) {
    logContainerRef.value.scrollTop = 0
  }
}

function cleanup() {
  if (logFlushTimer !== null) {
    clearTimeout(logFlushTimer)
    logFlushTimer = null
  }
  logBuffer = []
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  if (reconnectTimer !== null) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

function handleVisibilityChange() {
  flushBufferedLogs()
  if (document.hidden) {
    hiddenAt = Date.now()
    return
  }

  if (!props.visible || !props.taskId) {
    hiddenAt = null
    return
  }

  const wasBackgrounded = hiddenAt !== null && Date.now() - hiddenAt > 1500
  hiddenAt = null

  if (done.value) {
    if (!hasLogs.value) {
      void fetchLatestLog()
    }
    return
  }

  if (wasBackgrounded) {
    void startStream()
  }
}

onMounted(() => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  cleanup()
})

function handleClose() {
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="`任务日志 - ${taskName}`"
    width="85%"
    :fullscreen="dialogFullscreen"
    top="5vh"
    @close="handleClose"
  >
    <div class="log-viewer">
      <div class="log-toolbar">
        <div class="toolbar-copy">
          <div class="toolbar-title">实时输出</div>
          <div class="toolbar-subtitle">{{ autoScroll ? '新日志会自动滚动到底部' : '已暂停自动滚动，可手动查看历史输出' }}</div>
        </div>
        <div class="toolbar-actions">
          <el-checkbox v-model="autoScroll" size="small">自动滚动</el-checkbox>
          <transition name="status-switch" mode="out-in">
            <div v-if="!done" key="running" class="status-chip status-chip-running">
              <span class="status-orb" aria-hidden="true">
                <span class="status-orb-core"></span>
              </span>
              <span class="status-copy">
                <span class="status-label">运行中</span>
                <span class="status-meta">实时采集中</span>
              </span>
            </div>
            <div v-else key="done" class="status-chip status-chip-done">
              <span class="status-icon" aria-hidden="true">
                <el-icon :size="14"><CircleCheckFilled /></el-icon>
              </span>
              <span class="status-copy">
                <span class="status-label">已完成</span>
                <span class="status-meta">日志已收齐</span>
              </span>
            </div>
          </transition>
        </div>
      </div>

      <div ref="logContainerRef" class="log-container dd-log-surface" v-loading="loading">
        <div v-if="error" class="log-error">{{ error }}</div>
        <div v-else-if="!hasLogs && !loading" class="log-empty">暂无日志输出</div>
        <pre v-else class="log-content">{{ renderedLogText }}</pre>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped lang="scss">
.log-viewer {
  display: flex;
  flex-direction: column;
  height: 75vh;
  min-height: 500px;
}

.log-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 14px 16px;
  background:
    linear-gradient(135deg, rgba(23, 37, 84, 0.08), rgba(15, 118, 110, 0.06)),
    var(--el-fill-color-lighter);
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 14px;
  margin-bottom: 12px;
}

.log-container {
  flex: 1;
  overflow-y: auto;
  padding: 14px;
  font-family: var(--dd-font-mono);
  font-size: 13px;
  line-height: 1.6;
}

.log-content {
  margin: 0;
  color: #d4d4d4;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-error, .log-empty {
  color: #8c8c8c;
  text-align: center;
  padding: 40px 20px;
}

.toolbar-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toolbar-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.toolbar-subtitle {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  flex-wrap: wrap;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 7px 14px;
  border-radius: 999px;
  border: 1px solid transparent;
  min-height: 40px;
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.08);
}

.status-chip-running {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.18), rgba(251, 191, 36, 0.08));
  border-color: rgba(245, 158, 11, 0.24);
}

.status-chip-done {
  background: linear-gradient(135deg, rgba(22, 163, 74, 0.16), rgba(74, 222, 128, 0.08));
  border-color: rgba(34, 197, 94, 0.24);
}

.status-copy {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.1;
}

.status-label {
  font-size: 13px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.status-meta {
  font-size: 11px;
  color: var(--el-text-color-secondary);
}

.status-orb,
.status-icon {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.status-orb::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: rgba(245, 158, 11, 0.22);
  animation: runningRipple 1.8s ease-out infinite;
}

.status-orb-core {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f59e0b;
  box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.16);
  animation: runningCore 1.4s ease-in-out infinite;
}

.status-icon {
  border-radius: 50%;
  background: rgba(34, 197, 94, 0.14);
  color: #16a34a;
  animation: doneGlow 2.4s ease-in-out infinite;
}

.status-switch-enter-active,
.status-switch-leave-active {
  transition: all 0.22s ease;
}

.status-switch-enter-from,
.status-switch-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.96);
}

@keyframes runningRipple {
  0% {
    transform: scale(0.78);
    opacity: 0.7;
  }
  100% {
    transform: scale(1.35);
    opacity: 0;
  }
}

@keyframes runningCore {
  0%,
  100% {
    transform: scale(0.92);
  }
  50% {
    transform: scale(1.08);
  }
}

@keyframes doneGlow {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.1);
  }
  50% {
    box-shadow: 0 0 0 6px rgba(34, 197, 94, 0.04);
  }
}

@media (max-width: 768px) {
  .log-viewer {
    height: calc(100dvh - 120px);
    min-height: 0;
  }

  .toolbar-actions {
    width: 100%;
    justify-content: space-between;
    margin-left: 0;
  }

  .status-chip {
    min-width: 138px;
  }
}
</style>
