<script setup lang="ts">
import { ElMessageBox } from 'element-plus'
import ScriptAIAssistantDialog from './components/ScriptAIAssistantDialog.vue'
import ScriptExecutionDialogs from './components/ScriptExecutionDialogs.vue'
import ScriptManageDialogs from './components/ScriptManageDialogs.vue'
import ScriptsEditorPane from './components/ScriptsEditorPane.vue'
import ScriptsSidebar from './components/ScriptsSidebar.vue'
import { useScriptAI } from './useScriptAI'
import { useScriptExecution } from './useScriptExecution'
import { useScriptWorkspace } from './useScriptWorkspace'

const workspace = useScriptWorkspace()
const execution = useScriptExecution({
  selectedFile: workspace.selectedFile,
  fileContent: workspace.fileContent
})
const ai = useScriptAI({
  selectedFile: workspace.selectedFile,
  fileContent: workspace.fileContent,
  isBinary: workspace.isBinary,
  isEditing: workspace.isEditing,
  hasChanges: workspace.hasChanges,
  editorLanguage: workspace.editorLanguage,
  loadTree: workspace.loadTree,
  loadFileContent: workspace.loadFileContent,
  debugLogs: execution.debugLogs,
  debugExitCode: execution.debugExitCode,
  debugError: execution.debugError,
  openDebugAndStart: execution.openDebugAndStart
})

const {
  isMobile,
  mobileShowEditor,
  fileTree,
  selectedFile,
  fileContent,
  originalContent,
  isBinary,
  loading,
  saving,
  treeLoading,
  isEditing,
  showCreateFileDialog,
  showCreateDirDialog,
  showRenameDialog,
  showVersionDialog,
  showVersionDiffDialog,
  showUploadDialog,
  uploadDir,
  newFileName,
  newFileParent,
  newDirName,
  newDirParent,
  renameTarget,
  versions,
  versionsLoading,
  versionDiffLoading,
  versionDiffOriginalTitle,
  versionDiffModifiedTitle,
  versionDiffOriginalContent,
  versionDiffModifiedContent,
  formatting,
  editorLanguage,
  hasChanges,
  allFolders,
  loadTree,
  handleNodeClick,
  handleSave,
  handleCreateFile,
  handleCreateDir,
  handleDelete,
  handleMoveToRoot,
  allowDrag,
  allowDrop,
  handleNodeDrop,
  handleRename,
  openRename,
  openUploadDialog,
  handleUploadFileChange,
  handleUploadSubmit,
  handleAddToTask,
  loadVersions,
  handleRollback,
  handleClearVersions,
  handleCompareVersion,
  handleFormat,
  handleDownload,
  handleMobileBack
} = workspace

const {
  showDebugDialog,
  debugCode,
  debugFileName,
  debugLogs,
  debugRunning,
  debugError,
  debugExitCode,
  debugCodeChanged,
  showCodeRunner,
  runnerCode,
  runnerLanguage,
  runnerLogs,
  runnerRunning,
  runnerExitCode,
  runnerError,
  handleDebugRun,
  handleDebugStart,
  handleDebugStop,
  openCodeRunner,
  handleRunCode,
  handleStopRunner
} = execution

const {
  showAIDialog,
  aiEnabled,
  configLoading,
  generating,
  configuredProviders,
  provider,
  modelOverride,
  mode,
  responseMode,
  prompt,
  targetPath,
  manualLanguage,
  includeDebugLogs,
  autoDebugAfterApply,
  conversationMode,
  hasConversation,
  conversationTurnCount,
  previewBaseContent,
  resultSummary,
  resultContent,
  resultPreviewContent,
  resultWarnings,
  resultProviderLabel,
  resultModel,
  resultResponseMode,
  resultCanApply,
  generationError,
  hasDebugContext,
  resolvedLanguage,
  applyButtonText,
  openAIDialogFor,
  handleGenerate,
  handleCancelGenerate,
  handleApply
} = ai

async function handleDebugSave() {
  if (!selectedFile.value || isBinary.value) {
    return
  }
  fileContent.value = debugCode.value
  isEditing.value = true
  await handleSave()
  debugCodeChanged.value = debugCode.value !== originalContent.value
}

function openCreateFileDialog() {
  showCreateFileDialog.value = true
}

function openCreateDirDialog() {
  showCreateDirDialog.value = true
}

function openSelectedFileRenameDialog() {
  openRename(selectedFile.value)
}

function handleDeleteSelectedFile() {
  return handleDelete(selectedFile.value)
}

function openSelectedFileAIDialog() {
  openAIDialogFor(selectedFile.value ? 'modify' : 'generate')
}

async function handleCancelEdit() {
  if (hasChanges.value) {
    try {
      await ElMessageBox.confirm('当前有未保存的改动，确认放弃修改并退出编辑？', '退出编辑', {
        confirmButtonText: '放弃改动',
        cancelButtonText: '继续编辑',
        type: 'warning'
      })
    } catch {
      return
    }
    fileContent.value = originalContent.value
  }
  isEditing.value = false
}
</script>

<template>
  <div class="scripts-page" :class="{ mobile: isMobile, 'mobile-show-editor': isMobile && mobileShowEditor }">
    <ScriptsSidebar
      :is-mobile="isMobile"
      :mobile-show-editor="mobileShowEditor"
      :tree-loading="treeLoading"
      :file-tree="fileTree"
      :allow-drag="allowDrag"
      :allow-drop="allowDrop"
      :on-open-create-file="openCreateFileDialog"
      :on-open-create-dir="openCreateDirDialog"
      :on-open-upload="openUploadDialog"
      :on-open-code-runner="openCodeRunner"
      :on-refresh="loadTree"
      :on-node-click="handleNodeClick"
      :on-node-drop="handleNodeDrop"
      :on-open-rename="openRename"
      :on-delete="handleDelete"
      :on-move-to-root="handleMoveToRoot"
    />

    <ScriptsEditorPane
      v-model:file-content="fileContent"
      v-model:is-editing="isEditing"
      :is-mobile="isMobile"
      :mobile-show-editor="mobileShowEditor"
      :selected-file="selectedFile"
      :is-binary="isBinary"
      :has-changes="hasChanges"
      :saving="saving"
      :formatting="formatting"
      :loading="loading"
      :editor-language="editorLanguage"
      :on-mobile-back="handleMobileBack"
      :on-debug-run="handleDebugRun"
      :on-open-ai="openSelectedFileAIDialog"
      :on-add-to-task="handleAddToTask"
      :on-save="handleSave"
      :on-cancel-edit="handleCancelEdit"
      :on-format="handleFormat"
      :on-load-versions="loadVersions"
      :on-open-rename="openSelectedFileRenameDialog"
      :on-download="handleDownload"
      :on-delete="handleDeleteSelectedFile"
    />

    <ScriptManageDialogs
      v-model:show-create-file-dialog="showCreateFileDialog"
      v-model:show-create-dir-dialog="showCreateDirDialog"
      v-model:show-rename-dialog="showRenameDialog"
      v-model:show-version-dialog="showVersionDialog"
      v-model:show-version-diff-dialog="showVersionDiffDialog"
      v-model:show-upload-dialog="showUploadDialog"
      v-model:new-file-name="newFileName"
      v-model:new-file-parent="newFileParent"
      v-model:new-dir-name="newDirName"
      v-model:new-dir-parent="newDirParent"
      v-model:rename-target="renameTarget"
      v-model:upload-dir="uploadDir"
      v-model:version-diff-original-title="versionDiffOriginalTitle"
      v-model:version-diff-modified-title="versionDiffModifiedTitle"
      v-model:version-diff-original-content="versionDiffOriginalContent"
      v-model:version-diff-modified-content="versionDiffModifiedContent"
      :is-mobile="isMobile"
      :selected-file="selectedFile"
      :all-folders="allFolders"
      :editor-language="editorLanguage"
      :versions="versions"
      :versions-loading="versionsLoading"
      :version-diff-loading="versionDiffLoading"
      :on-create-file="handleCreateFile"
      :on-create-dir="handleCreateDir"
      :on-rename="handleRename"
      :on-compare-version="handleCompareVersion"
      :on-rollback="handleRollback"
      :on-clear-versions="handleClearVersions"
      :on-upload-file-change="handleUploadFileChange"
      :on-upload-submit="handleUploadSubmit"
    />

    <ScriptExecutionDialogs
      v-model:show-code-runner="showCodeRunner"
      v-model:runner-code="runnerCode"
      v-model:runner-language="runnerLanguage"
      v-model:show-debug-dialog="showDebugDialog"
      v-model:debug-code="debugCode"
      v-model:debug-code-changed="debugCodeChanged"
      :is-mobile="isMobile"
      :editor-language="editorLanguage"
      :debug-file-name="debugFileName"
      :debug-logs="debugLogs"
      :debug-running="debugRunning"
      :debug-error="debugError"
      :debug-exit-code="debugExitCode"
      :runner-logs="runnerLogs"
      :runner-running="runnerRunning"
      :runner-exit-code="runnerExitCode"
      :runner-error="runnerError"
      :debug-saving="saving"
      :on-debug-start="handleDebugStart"
      :on-debug-save="handleDebugSave"
      :on-debug-stop="handleDebugStop"
      :on-run-code="handleRunCode"
      :on-stop-runner="handleStopRunner"
    />

    <ScriptAIAssistantDialog
      v-model:show-ai-dialog="showAIDialog"
      v-model:provider="provider"
      v-model:model-override="modelOverride"
      v-model:mode="mode"
      v-model:response-mode="responseMode"
      v-model:prompt="prompt"
      v-model:target-path="targetPath"
      v-model:manual-language="manualLanguage"
      v-model:include-debug-logs="includeDebugLogs"
      v-model:auto-debug-after-apply="autoDebugAfterApply"
      v-model:conversation-mode="conversationMode"
      :is-mobile="isMobile"
      :ai-enabled="aiEnabled"
      :config-loading="configLoading"
      :generating="generating"
      :selected-file="selectedFile"
      :available-providers="configuredProviders"
      :has-debug-context="hasDebugContext"
      :current-content="previewBaseContent"
      :preview-language="resolvedLanguage"
      :result-summary="resultSummary"
      :result-warnings="resultWarnings"
      :result-content="resultContent"
      :result-preview-content="resultPreviewContent"
      :result-provider-label="resultProviderLabel"
      :result-model="resultModel"
      :result-response-mode="resultResponseMode"
      :result-can-apply="resultCanApply"
      :generation-error="generationError"
      :apply-button-text="applyButtonText"
      :has-conversation="hasConversation"
      :conversation-turn-count="conversationTurnCount"
      :on-generate="handleGenerate"
      :on-cancel="handleCancelGenerate"
      :on-apply="handleApply"
    />
  </div>
</template>

<style scoped lang="scss">
.scripts-page {
  --scripts-accent: #22c55e;
  --scripts-ai-accent-start: #6366f1;
  --scripts-ai-accent-end: #8b5cf6;
  --scripts-surface: var(--el-bg-color);
  --scripts-surface-muted: color-mix(in srgb, var(--el-fill-color) 70%, transparent);
  --scripts-border-soft: color-mix(in srgb, var(--el-border-color-light) 85%, transparent);

  display: flex;
  height: calc(100dvh - 120px);
  gap: 0;
  font-size: 14px;
  font-family: var(--dd-font-ui);
  background: var(--el-bg-color);
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid var(--scripts-border-soft);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

.scripts-page.mobile {
  flex-direction: column;
  height: calc(100dvh - 100px);
  border-radius: 0;
  border: none;
  box-shadow: none;

  :deep(.scripts-sidebar) {
    width: 100%;
    min-width: unset;
    flex: 1;
    border-right: none;
    border-bottom: 1px solid var(--scripts-border-soft);
  }

  :deep(.scripts-editor) {
    width: 100%;
    flex: 1;
  }
}

.scripts-page.mobile.mobile-show-editor {
  :deep(.scripts-editor) {
    height: 100%;
  }
}
</style>
