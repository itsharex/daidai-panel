<script setup lang="ts">
import { Document, Folder, Refresh, Search, VideoPlay } from '@element-plus/icons-vue'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AICodeWorkspace from './components/AICodeWorkspace.vue'
import { scriptApi } from '@/api/script'
import ScriptExecutionDialogs from '@/views/scripts/components/ScriptExecutionDialogs.vue'
import { useScriptAI } from '@/views/scripts/useScriptAI'
import { useScriptExecution } from '@/views/scripts/useScriptExecution'
import { useScriptWorkspaceBrowser } from '@/views/scripts/useScriptWorkspaceBrowser'
import type { TreeNode } from '@/views/scripts/types'

const route = useRoute()
const router = useRouter()
const browser = useScriptWorkspaceBrowser()
const execution = useScriptExecution({
  selectedFile: browser.selectedFile,
  fileContent: browser.fileContent
})
const ai = useScriptAI({
  selectedFile: browser.selectedFile,
  fileContent: browser.fileContent,
  isBinary: browser.isBinary,
  isEditing: browser.isEditing,
  hasChanges: browser.hasChanges,
  editorLanguage: browser.editorLanguage,
  loadTree: browser.loadTree,
  loadFileContent: browser.loadFileContent,
  debugLogs: execution.debugLogs,
  debugExitCode: execution.debugExitCode,
  debugError: execution.debugError,
  openDebugAndStart: execution.openDebugAndStart,
  applyMode: 'save'
})

const treeRef = ref()
const searchKeyword = ref('')

const {
  isMobile,
  fileTree,
  selectedFile,
  fileContent,
  originalContent,
  isBinary,
  loading,
  treeLoading,
  isEditing,
  editorLanguage,
  handleResize,
  loadTree,
  loadFileContent,
  handleNodeClick
} = browser

const {
  showCodeRunner,
  showDebugDialog,
  debugCode,
  debugFileName,
  debugLogs,
  debugRunning,
  debugError,
  debugExitCode,
  debugCodeChanged,
  runnerCode,
  runnerLanguage,
  runnerLogs,
  runnerRunning,
  runnerExitCode,
  runnerError,
  handleDebugRun,
  handleDebugStart,
  handleDebugStop,
  handleRunCode,
  handleStopRunner
} = execution

const {
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
  loadAIConfig,
  handleGenerate,
  handleCancelGenerate,
  handleApply
} = ai

function filterNode(value: string, data: TreeNode) {
  if (!value) return true
  return (data.title || '').toLowerCase().includes(value.toLowerCase())
}

function getFileIconColor(node: TreeNode) {
  if (!node.isLeaf) return '#e6a23c'
  const ext = node.title.split('.').pop()?.toLowerCase()
  switch (ext) {
    case 'js':
      return '#f0db4f'
    case 'ts':
      return '#3178c6'
    case 'py':
      return '#4b8bbe'
    case 'sh':
      return '#4eaa25'
    case 'json':
      return '#e37e36'
    case 'yaml':
    case 'yml':
      return '#cb171e'
    case 'md':
      return '#083fa1'
    case 'html':
      return '#e34c26'
    case 'css':
      return '#264de4'
    default:
      return 'var(--el-text-color-secondary)'
  }
}

function getTreeIcon(node: TreeNode) {
  return node.isLeaf ? Document : Folder
}

function resetSelection() {
  selectedFile.value = ''
  fileContent.value = ''
  originalContent.value = ''
  isBinary.value = false
  isEditing.value = false
}

function openScriptsPage() {
  router.push(selectedFile.value ? { path: '/scripts', query: { file: selectedFile.value } } : '/scripts')
}

async function openFileFromRoute(fileParam?: string) {
  if (!fileParam) return
  selectedFile.value = fileParam
  const loaded = await loadFileContent(fileParam)
  if (!loaded) {
    selectedFile.value = ''
  }
  await router.replace({ path: '/ai-code' })
}

async function handleDebugSave() {
  if (!selectedFile.value || isBinary.value) {
    return
  }

  try {
    await scriptApi.saveContent(selectedFile.value, debugCode.value, '调试结果保存脚本')
    fileContent.value = debugCode.value
    originalContent.value = debugCode.value
    isEditing.value = false
    ElMessage.success('脚本已保存')
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || err?.message || '保存脚本失败')
  }

  debugCodeChanged.value = debugCode.value !== originalContent.value
}

watch(searchKeyword, (value) => {
  treeRef.value?.filter(value)
})

watch(
  () => route.query.file,
  (fileParam) => {
    if (typeof fileParam !== 'string' || !fileParam.trim()) {
      return
    }
    void openFileFromRoute(fileParam)
  },
  { immediate: true }
)

const AI_CODE_PROMPT_STORAGE_KEY = 'ai-code.prompt'
const AI_CODE_PROMPT_TOUCHED_KEY = 'ai-code.prompt.touched'
const DEFAULT_AI_CODE_PROMPT = '请生成一个可直接运行的新脚本，并在必要时补齐注释与错误处理。'

// 监听用户修改 prompt：只要用户手动编辑过，就记录 touched 标志，下次不再覆盖
watch(prompt, (val) => {
  try {
    if (val && val.trim()) {
      localStorage.setItem(AI_CODE_PROMPT_STORAGE_KEY, val)
      localStorage.setItem(AI_CODE_PROMPT_TOUCHED_KEY, '1')
    } else if (localStorage.getItem(AI_CODE_PROMPT_TOUCHED_KEY) === '1') {
      // 用户主动清空，也记录为已触碰过
      localStorage.setItem(AI_CODE_PROMPT_STORAGE_KEY, '')
    }
  } catch { /* localStorage disabled */ }
})

onMounted(() => {
  window.addEventListener('resize', handleResize)
  void loadTree()
  void loadAIConfig()
  const touched = (() => {
    try { return localStorage.getItem(AI_CODE_PROMPT_TOUCHED_KEY) === '1' } catch { return false }
  })()
  if (touched) {
    // 用户曾修改过，恢复保存值（即使是空字符串）
    try {
      const saved = localStorage.getItem(AI_CODE_PROMPT_STORAGE_KEY)
      if (saved !== null) prompt.value = saved
    } catch { /* ignore */ }
  } else if (!prompt.value.trim()) {
    prompt.value = DEFAULT_AI_CODE_PROMPT
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="ai-code-page" :class="{ mobile: isMobile }">
    <div class="page-header">
      <div class="page-copy">
        <div class="page-kicker">Ai Code</div>
        <h1>AI 脚本助手</h1>
        <p>选择现有脚本进行修改或修复，也可以直接生成新脚本并保存到脚本目录。</p>
      </div>
      <div class="page-actions">
        <el-button @click="openScriptsPage">脚本管理</el-button>
        <el-button type="primary" :disabled="!selectedFile || isBinary || loading" @click="handleDebugRun">
          <el-icon><VideoPlay /></el-icon>调试当前脚本
        </el-button>
      </div>
    </div>

    <div class="page-body">
      <aside class="browser-panel">
        <div class="browser-header">
          <div class="browser-title">
            <el-icon><Document /></el-icon>
            <span>脚本上下文</span>
          </div>
          <el-button text @click="loadTree">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <el-input
          v-model="searchKeyword"
          placeholder="搜索文件或目录..."
          clearable
          size="small"
          :prefix-icon="Search"
          class="browser-search"
        />

        <div class="browser-tree" v-loading="treeLoading">
          <el-tree
            ref="treeRef"
            :data="fileTree"
            node-key="key"
            :props="{ children: 'children', label: 'title' }"
            :highlight-current="true"
            :expand-on-click-node="true"
            :filter-node-method="filterNode"
            @node-click="handleNodeClick"
          >
            <template #default="{ data }">
              <div class="tree-entry">
                <el-icon :size="14" :style="{ color: getFileIconColor(data) }"><component :is="getTreeIcon(data)" /></el-icon>
                <span class="tree-entry-label">{{ data.title }}</span>
                <span v-if="data.isLeaf && data.title.includes('.')" class="tree-entry-ext">
                  {{ data.title.split('.').pop()?.toUpperCase() }}
                </span>
              </div>
            </template>
          </el-tree>
        </div>
      </aside>

      <section class="workspace-panel">
        <AICodeWorkspace
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
          variant="page"
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
      </section>
    </div>

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
      :debug-saving="generating"
      :on-debug-start="handleDebugStart"
      :on-debug-save="handleDebugSave"
      :on-debug-stop="handleDebugStop"
      :on-run-code="handleRunCode"
      :on-stop-runner="handleStopRunner"
    />
  </div>
</template>

<style scoped lang="scss">
.ai-code-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: calc(100dvh - 120px);
  min-height: 0;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 16px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--el-color-primary-light-9) 42%, white) 0%, var(--el-bg-color) 58%);
}

.page-copy {
  min-width: 0;

  h1 {
    margin: 6px 0 8px;
    font-size: 24px;
    line-height: 1.15;
    color: var(--el-text-color-primary);
  }

  p {
    margin: 0;
    font-size: 14px;
    line-height: 1.65;
    color: var(--el-text-color-secondary);
  }
}

.page-kicker {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--el-color-primary);
}

.page-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.page-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 16px;
}

.browser-panel,
.workspace-panel {
  min-height: 0;
  border: 1px solid var(--el-border-color-light);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.browser-panel {
  display: flex;
  flex-direction: column;
  padding: 14px;
}

.browser-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 12px;
}

.browser-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.browser-search {
  margin-bottom: 12px;
}

.browser-tree {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;

  :deep(.el-tree-node__content) {
    height: 36px;
    border-radius: 8px;
  }

  :deep(.el-tree-node__content:hover) {
    background: var(--el-fill-color-light);
  }

  :deep(.el-tree-node.is-current > .el-tree-node__content) {
    background: var(--el-color-primary-light-9);
    color: var(--el-color-primary);
  }
}

.tree-entry {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  overflow: hidden;
}

.tree-entry-label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.tree-entry-ext {
  font-size: 9px;
  font-weight: 700;
  font-family: var(--dd-font-mono);
  padding: 1px 4px;
  border-radius: 3px;
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}

.workspace-panel {
  padding: 16px;
  overflow: hidden;
}

.ai-code-page.mobile {
  height: auto;

  .page-header,
  .page-actions,
  .page-body {
    display: flex;
    flex-direction: column;
  }

  .page-actions {
    align-items: stretch;
  }

  .page-body {
    gap: 12px;
  }

  .browser-panel,
  .workspace-panel {
    padding: 12px;
  }
}
</style>
