<script setup lang="ts">
import { ArrowLeft, Check, Clock, Delete, Document, Download, Edit, MagicStick, MoreFilled, Plus, VideoPlay } from '@element-plus/icons-vue'
import { defineAsyncComponent } from 'vue'

const MonacoEditor = defineAsyncComponent(() => import('@/components/MonacoEditor.vue'))

const fileContent = defineModel<string>('fileContent', { required: true })
const isEditing = defineModel<boolean>('isEditing', { required: true })

defineProps<{
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
  onAddToTask: () => void
  onSave: () => void | Promise<void>
  onFormat: () => void | Promise<void>
  onLoadVersions: () => void | Promise<void>
  onOpenRename: () => void
  onDownload: () => void
  onDelete: () => void | Promise<void>
}>()

function getFileName(path: string) {
  return path.split('/').pop() || path
}

function startEdit() {
  isEditing.value = true
}
</script>

<template>
  <div class="scripts-editor" :class="{ mobile: isMobile }" v-show="!isMobile || mobileShowEditor">
    <div v-if="!selectedFile" class="editor-placeholder">
      <el-empty description="选择一个文件查看内容" />
    </div>
    <template v-else>
      <div class="editor-header">
        <div class="editor-file-info">
          <el-button v-if="isMobile" class="mobile-back-btn" text @click="onMobileBack">
            <el-icon><ArrowLeft /></el-icon>
          </el-button>
          <el-icon><Document /></el-icon>
          <span>{{ getFileName(selectedFile) }}</span>
          <el-tag v-if="hasChanges" size="small" type="warning">未保存</el-tag>
        </div>
        <div class="editor-actions">
          <el-button v-if="!isEditing" :size="isMobile ? 'small' : 'default'" type="primary" @click="startEdit" :disabled="isBinary">
            <el-icon><Edit /></el-icon><span v-if="!isMobile">编辑</span>
          </el-button>
          <el-button v-if="isEditing" :size="isMobile ? 'small' : 'default'" type="success" @click="onDebugRun" :disabled="isBinary">
            <el-icon><VideoPlay /></el-icon><span v-if="!isMobile">调试</span>
          </el-button>
          <el-button v-if="!isMobile" size="default" type="primary" @click="onAddToTask" :disabled="isBinary">
            <el-icon><Plus /></el-icon>添加任务
          </el-button>
          <el-button v-if="isEditing" :size="isMobile ? 'small' : 'default'" type="primary" @click="onSave" :loading="saving" :disabled="!hasChanges || isBinary">
            <el-icon><Check /></el-icon><span v-if="!isMobile">保存</span>
          </el-button>
          <el-button v-if="isEditing && !isMobile" size="default" @click="onFormat" :loading="formatting" :disabled="isBinary">
            <el-icon><MagicStick /></el-icon>格式化
          </el-button>
          <el-dropdown trigger="click">
            <el-button :size="isMobile ? 'small' : 'default'">
              <el-icon><MoreFilled /></el-icon><span v-if="!isMobile">更多</span>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="onLoadVersions" :disabled="isBinary">
                  <el-icon><Clock /></el-icon>版本历史
                </el-dropdown-item>
                <el-dropdown-item v-if="isMobile" @click="onAddToTask" :disabled="isBinary">
                  <el-icon><Plus /></el-icon>添加任务
                </el-dropdown-item>
                <el-dropdown-item v-if="isMobile && isEditing" @click="onFormat" :disabled="isBinary">
                  <el-icon><MagicStick /></el-icon>格式化
                </el-dropdown-item>
                <el-dropdown-item @click="onOpenRename">
                  <el-icon><Edit /></el-icon>重命名
                </el-dropdown-item>
                <el-dropdown-item @click="onDownload" :disabled="isBinary">
                  <el-icon><Download /></el-icon>下载
                </el-dropdown-item>
                <el-dropdown-item divided @click="onDelete">
                  <el-icon><Delete /></el-icon>删除
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
      <div class="editor-content" v-loading="loading">
        <div v-if="isBinary" class="binary-notice">
          <el-result icon="info" title="二进制文件" sub-title="该文件为二进制格式，无法在线编辑" />
        </div>
        <pre v-else-if="!isEditing" class="code-preview">{{ fileContent }}</pre>
        <MonacoEditor
          v-else
          v-model="fileContent"
          :language="editorLanguage"
          :readonly="!isEditing"
          class="code-editor"
        />
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.scripts-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.editor-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.editor-header {
  padding: 12px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  flex-shrink: 0;

  .editor-file-info {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 15px;
  }

  .editor-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }
}

.editor-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;

  .code-editor {
    flex: 1;
    height: 100%;
    min-height: 500px;
  }
}

.code-preview {
  margin: 0;
  height: 100%;
  overflow: auto;
  padding: 18px 20px;
  background: #111827;
  color: #e5e7eb;
  font-size: 13px;
  line-height: 1.6;
  font-family: var(--dd-font-mono, Consolas, 'Courier New', monospace);
  white-space: pre;
}

.binary-notice {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mobile-back-btn {
  padding: 4px;
  margin-right: -4px;
}

.scripts-editor.mobile {
  .editor-header {
    padding: 8px 12px;
    gap: 6px;
    align-items: flex-start;
    flex-wrap: wrap;

    .editor-file-info {
      gap: 6px;
      font-size: 14px;
      min-width: 0;
      overflow: hidden;
      width: 100%;

      span {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .editor-actions {
      gap: 4px;
      width: 100%;
      flex-wrap: wrap;
      justify-content: flex-end;
    }
  }

  .editor-content {
    .code-editor {
      min-height: 300px;
    }
  }
}
</style>
