<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, ref, watch } from 'vue'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Clock,
  Close,
  Delete,
  Document,
  Download,
  Edit,
  MagicStick,
  MoreFilled,
  Plus,
  VideoPlay
} from '@element-plus/icons-vue'

const MonacoEditor = defineAsyncComponent(() => import('@/components/MonacoEditor.vue'))

const fileContent = defineModel<string>('fileContent', { required: true })
const isEditing = defineModel<boolean>('isEditing', { required: true })

const props = defineProps<{
  isMobile: boolean
  mobileShowEditor: boolean
  selectedFile: string
  isBinary: boolean
  hasChanges: boolean
  saving: boolean
  formatting: boolean
  loading: boolean
  editorLanguage: string
  onMobileBack: () => void
  onDebugRun: () => void | Promise<void>
  onOpenAi: () => void | Promise<void>
  onAddToTask: () => void
  onSave: () => void | Promise<void>
  onCancelEdit: () => void | Promise<void>
  onFormat: () => void | Promise<void>
  onLoadVersions: () => void | Promise<void>
  onOpenRename: () => void
  onDownload: () => void
  onDelete: () => void | Promise<void>
}>()

const fileName = computed(() => {
  if (!props.selectedFile) return ''
  return props.selectedFile.split('/').pop() || props.selectedFile
})

const filePath = computed(() => {
  if (!props.selectedFile) return ''
  const parts = props.selectedFile.split('/')
  parts.pop()
  return parts
})

const languageLabel = computed(() => {
  const lang = (props.editorLanguage || '').toLowerCase()
  if (!lang) return ''
  const map: Record<string, string> = {
    javascript: 'JS',
    typescript: 'TS',
    python: 'PY',
    shell: 'SH',
    bash: 'SH',
    yaml: 'YAML',
    json: 'JSON',
    markdown: 'MD',
    html: 'HTML',
    css: 'CSS',
    go: 'GO',
    plaintext: 'TXT'
  }
  return map[lang] || lang.toUpperCase().slice(0, 4)
})

const fileSizeLabel = computed(() => {
  if (props.isBinary) return ''
  if (typeof fileContent.value !== 'string') return ''
  const bytes = new Blob([fileContent.value]).size
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
})

const lineCountLabel = computed(() => {
  if (props.isBinary || !fileContent.value) return ''
  const count = fileContent.value.split('\n').length
  return `${count} 行`
})

function startEdit() {
  isEditing.value = true
}

const previewRef = ref<HTMLElement | null>(null)

// Ctrl+A / Cmd+A while the preview is focused selects only the script content,
// not the whole page. Monaco handles its own selection natively, so this only
// applies to the read-only preview mode.
function handlePreviewKeydown(event: KeyboardEvent) {
  const isSelectAll = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'a'
  if (!isSelectAll || event.altKey || event.shiftKey) return
  if (!previewRef.value) return
  event.preventDefault()
  const selection = window.getSelection()
  if (!selection) return
  selection.removeAllRanges()
  const range = document.createRange()
  range.selectNodeContents(previewRef.value)
  selection.addRange(range)
}

// When a script is opened in preview mode we give its pane focus so Ctrl+A
// immediately targets the script content without the user having to click the
// pane first. Editing mode defers focus to Monaco.
watch(
  [() => props.selectedFile, () => props.isBinary, isEditing, () => props.loading],
  ([file, binary, editing, loading]) => {
    if (!file || binary || editing || loading) return
    void nextTick(() => {
      previewRef.value?.focus({ preventScroll: true })
    })
  },
  { immediate: true }
)
</script>

<template>
  <section class="scripts-editor" :class="{ mobile: isMobile }" v-show="!isMobile || mobileShowEditor">
    <!-- Empty state -->
    <div v-if="!selectedFile" class="editor-empty">
      <div class="empty-card">
        <div class="empty-aura" aria-hidden="true"></div>
        <div class="empty-badge">
          <el-icon :size="20"><MagicStick /></el-icon>
        </div>
        <h2 class="empty-title">选一个文件开始，或让 AI 帮你写一个</h2>
        <p class="empty-subtitle">左侧树里点击任意脚本即可查看，或直接用 AI 生成一份新脚本草稿再保存到目录。</p>
        <div class="empty-actions">
          <el-button class="ai-cta" type="primary" size="large" @click="onOpenAi">
            <el-icon><MagicStick /></el-icon>让 AI 生成脚本
          </el-button>
        </div>
      </div>
    </div>

    <template v-else>
      <!-- Hero header -->
      <header class="editor-hero">
        <div class="hero-file">
          <el-button v-if="isMobile" class="mobile-back" text @click="onMobileBack" aria-label="返回文件列表">
            <el-icon :size="18"><ArrowLeft /></el-icon>
          </el-button>
          <div class="file-icon" aria-hidden="true">
            <el-icon :size="18"><Document /></el-icon>
          </div>
          <div class="file-meta">
            <nav v-if="filePath.length > 0 && !isMobile" class="breadcrumb" aria-label="路径">
              <template v-for="(seg, idx) in filePath" :key="idx">
                <span class="breadcrumb-seg">{{ seg }}</span>
                <el-icon v-if="idx < filePath.length - 1" class="breadcrumb-sep" :size="10">
                  <ArrowRight />
                </el-icon>
              </template>
            </nav>
            <div class="file-title-row">
              <h1 class="file-title" :title="selectedFile">{{ fileName }}</h1>
              <span v-if="languageLabel" class="file-pill file-pill--lang">{{ languageLabel }}</span>
              <span v-if="isBinary" class="file-pill file-pill--binary">二进制</span>
              <span v-else-if="fileSizeLabel && !isMobile" class="file-pill file-pill--muted">{{ fileSizeLabel }}</span>
              <span v-if="hasChanges" class="unsaved-pulse" role="status" aria-label="文件有未保存的改动">
                <span class="unsaved-dot"></span>
                <span class="unsaved-label">未保存</span>
              </span>
            </div>
          </div>
        </div>

        <div class="hero-actions">
          <el-button
            v-if="!isEditing"
            class="action-btn"
            :size="isMobile ? 'small' : 'default'"
            :disabled="isBinary"
            @click="startEdit"
          >
            <el-icon><Edit /></el-icon><span v-if="!isMobile">编辑</span>
          </el-button>
          <template v-else>
            <el-button
              class="action-btn action-btn--primary"
              type="primary"
              :size="isMobile ? 'small' : 'default'"
              :loading="saving"
              :disabled="!hasChanges || isBinary"
              @click="onSave"
            >
              <el-icon><Check /></el-icon><span v-if="!isMobile">保存</span>
            </el-button>
            <el-button
              class="action-btn action-btn--cancel"
              :size="isMobile ? 'small' : 'default'"
              :disabled="saving"
              @click="onCancelEdit"
            >
              <el-icon><Close /></el-icon><span v-if="!isMobile">退出编辑</span>
            </el-button>
          </template>

          <el-button
            class="action-btn action-btn--run"
            :size="isMobile ? 'small' : 'default'"
            :disabled="isBinary"
            @click="onDebugRun"
          >
            <el-icon><VideoPlay /></el-icon><span v-if="!isMobile">调试</span>
          </el-button>

          <el-button
            class="action-btn action-btn--ai"
            :size="isMobile ? 'small' : 'default'"
            :disabled="isBinary"
            @click="onOpenAi"
          >
            <el-icon><MagicStick /></el-icon><span v-if="!isMobile">AI 助手</span>
          </el-button>

          <el-dropdown trigger="click" placement="bottom-end">
            <el-button class="action-btn" :size="isMobile ? 'small' : 'default'" aria-label="更多操作">
              <el-icon><MoreFilled /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="isEditing" @click="onFormat" :disabled="isBinary">
                  <el-icon><MagicStick /></el-icon>格式化
                </el-dropdown-item>
                <el-dropdown-item @click="onLoadVersions" :disabled="isBinary">
                  <el-icon><Clock /></el-icon>版本历史
                </el-dropdown-item>
                <el-dropdown-item @click="onAddToTask" :disabled="isBinary">
                  <el-icon><Plus /></el-icon>添加任务
                </el-dropdown-item>
                <el-dropdown-item @click="onOpenRename">
                  <el-icon><Edit /></el-icon>重命名
                </el-dropdown-item>
                <el-dropdown-item @click="onDownload">
                  <el-icon><Download /></el-icon>下载
                </el-dropdown-item>
                <el-dropdown-item divided @click="onDelete">
                  <el-icon><Delete /></el-icon>
                  <span style="color: var(--el-color-danger)">删除</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <!-- Editor body -->
      <div class="editor-body" v-loading="loading">
        <div v-if="isBinary" class="binary-card">
          <div class="binary-card-title">二进制文件</div>
          <p class="binary-card-text">该文件为二进制格式，无法在线编辑。可通过右上角「更多 → 下载」取回文件。</p>
        </div>
        <pre
          v-else-if="!isEditing"
          ref="previewRef"
          class="code-preview"
          tabindex="0"
          @keydown="handlePreviewKeydown"
        >{{ fileContent }}</pre>
        <MonacoEditor
          v-else
          v-model="fileContent"
          :language="editorLanguage"
          :readonly="!isEditing"
          class="code-editor"
        />
      </div>

      <!-- Status strip -->
      <footer v-if="!isBinary && selectedFile" class="editor-statusbar">
        <div class="status-group">
          <span v-if="languageLabel" class="status-item status-item--lang">{{ languageLabel }}</span>
          <span v-if="lineCountLabel" class="status-item">{{ lineCountLabel }}</span>
          <span v-if="fileSizeLabel" class="status-item">{{ fileSizeLabel }}</span>
        </div>
        <div class="status-group">
          <span class="status-item">UTF-8</span>
          <span class="status-item">LF</span>
          <span class="status-item" :class="{ 'status-item--accent': isEditing }">
            {{ isEditing ? '编辑中' : '只读' }}
          </span>
        </div>
      </footer>
    </template>
  </section>
</template>

<style scoped lang="scss">
.scripts-editor {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: var(--scripts-surface, var(--el-bg-color));
  font-family: var(--dd-font-ui);
}

/* ---------------- Empty state ---------------- */
.editor-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
}

.empty-card {
  position: relative;
  max-width: 480px;
  width: 100%;
  padding: 36px 32px 32px;
  text-align: center;
  border-radius: 18px;
  background: var(--scripts-surface-muted, var(--el-fill-color-lighter));
  border: 1px solid var(--scripts-border-soft, var(--el-border-color-light));
  overflow: hidden;
  isolation: isolate;
}

.empty-aura {
  position: absolute;
  inset: -2px;
  z-index: -1;
  padding: 1px;
  border-radius: inherit;
  background: linear-gradient(135deg,
      color-mix(in srgb, #22c55e 50%, transparent) 0%,
      color-mix(in srgb, #6366f1 45%, transparent) 50%,
      color-mix(in srgb, #8b5cf6 40%, transparent) 100%);
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
          mask-composite: exclude;
  opacity: 0.75;
  pointer-events: none;
}

.empty-badge {
  width: 44px;
  height: 44px;
  margin: 0 auto 14px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  box-shadow: 0 6px 18px -8px rgba(99, 102, 241, 0.55);
}

.empty-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 6px;
  letter-spacing: 0.2px;
  color: var(--el-text-color-primary);
}

.empty-subtitle {
  font-size: 13px;
  line-height: 1.55;
  margin: 0 0 20px;
  color: var(--el-text-color-secondary);
}

.empty-actions {
  display: inline-flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: center;
}

.ai-cta {
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  border: none;
  min-width: 160px;

  &:hover,
  &:focus {
    background: linear-gradient(135deg, #4f46e5, #7c3aed);
    border: none;
  }
}

/* ---------------- Hero header ---------------- */
.editor-hero {
  padding: 14px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  border-bottom: 1px solid var(--scripts-border-soft, var(--el-border-color-light));
  background: linear-gradient(180deg,
      color-mix(in srgb, var(--el-bg-color) 100%, transparent) 0%,
      color-mix(in srgb, var(--el-fill-color-lighter) 60%, transparent) 100%);
  flex-shrink: 0;
  min-width: 0;
}

.hero-file {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  flex: 1;
}

.file-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 10%, transparent);
  flex-shrink: 0;
}

.file-meta {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  color: var(--el-text-color-placeholder);
  line-height: 1.2;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;

  .breadcrumb-seg {
    font-family: var(--dd-font-mono);
    letter-spacing: 0.2px;
  }

  .breadcrumb-sep {
    opacity: 0.5;
    flex-shrink: 0;
  }
}

.file-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.file-title {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 0.1px;
  color: var(--el-text-color-primary);
  margin: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-pill {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.5px;
  font-family: var(--dd-font-mono);
  line-height: 1;
}

.file-pill--lang {
  background: color-mix(in srgb, var(--scripts-accent, #22c55e) 15%, transparent);
  color: color-mix(in srgb, var(--scripts-accent, #22c55e) 80%, #fff 20%);
}

.file-pill--muted {
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
}

.file-pill--binary {
  background: color-mix(in srgb, var(--el-color-warning) 16%, transparent);
  color: var(--el-color-warning);
}

.unsaved-pulse {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 20px;
  padding: 0 9px 0 6px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--el-color-warning);
  background: color-mix(in srgb, var(--el-color-warning) 12%, transparent);

  .unsaved-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--el-color-warning);
    animation: unsaved-pulse 1.6s ease-in-out infinite;
  }

  .unsaved-label {
    font-weight: 600;
    letter-spacing: 0.3px;
  }
}

@keyframes unsaved-pulse {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.25); opacity: 0.55; }
}

@media (prefers-reduced-motion: reduce) {
  .unsaved-dot { animation: none; }
}

.hero-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.action-btn {
  border-radius: 8px;
  font-weight: 500;
}

.action-btn--primary {
  box-shadow: 0 6px 14px -8px color-mix(in srgb, var(--el-color-primary) 60%, transparent);
}

.action-btn--cancel {
  color: var(--el-text-color-regular);

  &:hover:not(.is-disabled) {
    color: var(--el-color-danger);
    border-color: color-mix(in srgb, var(--el-color-danger) 40%, var(--el-border-color));
    background: color-mix(in srgb, var(--el-color-danger) 6%, transparent);
  }
}

.action-btn--run {
  --el-button-bg-color: color-mix(in srgb, #22c55e 18%, transparent);
  --el-button-border-color: color-mix(in srgb, #22c55e 35%, var(--el-border-color));
  --el-button-hover-bg-color: color-mix(in srgb, #22c55e 28%, transparent);
  --el-button-hover-border-color: #22c55e;
  --el-button-hover-text-color: #22c55e;
  --el-button-text-color: color-mix(in srgb, #22c55e 85%, var(--el-text-color-primary));
}

.action-btn--ai {
  color: #fff;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  border: none;
  box-shadow: 0 6px 14px -10px rgba(99, 102, 241, 0.6);

  &:hover {
    background: linear-gradient(135deg, #4f46e5, #7c3aed);
    color: #fff;
  }

  &:focus-visible {
    outline: 2px solid color-mix(in srgb, #8b5cf6 70%, transparent);
    outline-offset: 2px;
  }

  &.is-disabled {
    opacity: 0.55;
    filter: saturate(0.6);
  }
}

/* ---------------- Editor body ---------------- */
.editor-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;

  .code-editor {
    flex: 1;
    height: 100%;
    min-height: 420px;
  }
}

.code-preview {
  margin: 0;
  height: 100%;
  overflow: auto;
  padding: 18px 22px;
  background: var(--dd-editor-bg-color, #0f172a);
  color: var(--dd-editor-fg-color, #e2e8f0);
  font-size: 13px;
  line-height: 1.6;
  font-family: var(--dd-font-mono);
  white-space: pre;
  outline: none;
  cursor: text;

  &::selection,
  *::selection {
    background: color-mix(in srgb, var(--scripts-accent, #22c55e) 35%, transparent);
  }

  &:focus-visible {
    box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--scripts-accent, #22c55e) 55%, transparent);
  }
}

.binary-card {
  margin: 24px;
  padding: 24px 28px;
  border: 1px dashed var(--scripts-border-soft, var(--el-border-color-light));
  border-radius: 14px;
  background: var(--scripts-surface-muted, var(--el-fill-color-lighter));

  .binary-card-title {
    font-size: 14px;
    font-weight: 600;
    font-family: var(--dd-font-mono);
    letter-spacing: 0.4px;
    color: var(--el-text-color-primary);
    margin-bottom: 6px;
  }

  .binary-card-text {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    margin: 0;
    line-height: 1.55;
  }
}

/* ---------------- Status bar ---------------- */
.editor-statusbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 18px;
  border-top: 1px solid var(--scripts-border-soft, var(--el-border-color-light));
  font-family: var(--dd-font-mono);
  font-size: 11px;
  color: var(--el-text-color-placeholder);
  background: color-mix(in srgb, var(--el-fill-color-lighter) 70%, transparent);
}

.status-group {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.status-item {
  letter-spacing: 0.4px;
}

.status-item--lang {
  color: var(--scripts-accent, #22c55e);
  font-weight: 600;
}

.status-item--accent {
  color: var(--scripts-accent, #22c55e);
}

/* ---------------- Mobile ---------------- */
.mobile-back {
  padding: 4px;
  margin-right: -2px;
}

.scripts-editor.mobile {
  .editor-hero {
    padding: 10px 12px;
    gap: 8px;
    align-items: flex-start;
    flex-wrap: wrap;

    .hero-file {
      gap: 8px;
      width: 100%;
      min-width: 0;
    }

    .file-icon {
      width: 30px;
      height: 30px;
      border-radius: 8px;
    }

    .file-title {
      font-size: 16px;
    }

    .hero-actions {
      width: 100%;
      justify-content: flex-end;
      gap: 6px;
      flex-wrap: wrap;
    }
  }

  .editor-body {
    .code-editor {
      min-height: 300px;
    }
  }

  .editor-statusbar {
    padding: 5px 12px;
    font-size: 10.5px;
  }

  .empty-card {
    padding: 28px 20px;
  }
}
</style>
