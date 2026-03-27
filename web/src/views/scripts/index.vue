<script setup lang="ts">
import ScriptExecutionDialogs from './components/ScriptExecutionDialogs.vue'
import ScriptManageDialogs from './components/ScriptManageDialogs.vue'
import ScriptsEditorPane from './components/ScriptsEditorPane.vue'
import ScriptsSidebar from './components/ScriptsSidebar.vue'
import { useScriptExecution } from './useScriptExecution'
import { useScriptWorkspace } from './useScriptWorkspace'

const workspace = useScriptWorkspace()
const execution = useScriptExecution({
  selectedFile: workspace.selectedFile,
  fileContent: workspace.fileContent
})

const {
  isMobile,
  mobileShowEditor,
  fileTree,
  selectedFile,
  fileContent,
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
  handleDebugRun,
  handleDebugStart,
  handleDebugStop,
  openCodeRunner,
  handleRunCode,
  handleStopRunner
} = execution

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
      :on-add-to-task="handleAddToTask"
      :on-save="handleSave"
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
      :on-debug-start="handleDebugStart"
      :on-debug-stop="handleDebugStop"
      :on-run-code="handleRunCode"
      :on-stop-runner="handleStopRunner"
    />
  </div>
</template>

<style scoped lang="scss">
.scripts-page {
  display: flex;
  height: calc(100dvh - 120px);
  gap: 0;
  font-size: 14px;
}

.scripts-page.mobile {
  flex-direction: column;
  height: calc(100dvh - 100px);

  :deep(.scripts-sidebar) {
    width: 100%;
    min-width: unset;
    flex: 1;
    border-right: none;
    border-bottom: 1px solid var(--el-border-color-light);
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
