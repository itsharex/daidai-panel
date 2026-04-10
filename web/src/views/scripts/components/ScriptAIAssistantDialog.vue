<script setup lang="ts">
import AICodeWorkspace from '@/views/ai-code/components/AICodeWorkspace.vue'
import type { AICodeMode, AICodeResponseMode, AIProviderOption } from '@/api/ai'

const showAiDialog = defineModel<boolean>('showAiDialog', { required: true })
const provider = defineModel<string>('provider', { required: true })
const modelOverride = defineModel<string>('modelOverride', { required: true })
const mode = defineModel<AICodeMode | string>('mode', { required: true })
const responseMode = defineModel<AICodeResponseMode | string>('responseMode', { required: true })
const prompt = defineModel<string>('prompt', { required: true })
const targetPath = defineModel<string>('targetPath', { required: true })
const manualLanguage = defineModel<string>('manualLanguage', { required: true })
const includeDebugLogs = defineModel<boolean>('includeDebugLogs', { required: true })
const autoDebugAfterApply = defineModel<boolean>('autoDebugAfterApply', { required: true })
const conversationMode = defineModel<'continue' | 'restart'>('conversationMode', { required: true })

defineProps<{
  isMobile: boolean
  aiEnabled: boolean
  configLoading: boolean
  generating: boolean
  selectedFile: string
  availableProviders: AIProviderOption[]
  hasDebugContext: boolean
  currentContent: string
  previewLanguage: string
  resultSummary: string
  resultWarnings: string[]
  resultContent: string
  resultPreviewContent: string
  resultProviderLabel: string
  resultModel: string
  resultResponseMode: AICodeResponseMode | string
  resultCanApply: boolean
  generationError: string
  applyButtonText: string
  hasConversation: boolean
  conversationTurnCount: number
  onGenerate: () => void | Promise<void>
  onCancel: () => void
  onApply: () => void | Promise<void>
}>()
</script>

<template>
  <el-dialog
    v-model="showAiDialog"
    class="ai-dialog-shell"
    title="AI 脚本助手"
    :width="isMobile ? '100%' : '96vw'"
    :fullscreen="isMobile"
    :close-on-click-modal="false"
    :top="isMobile ? '0' : '2vh'"
    destroy-on-close
  >
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
      variant="dialog"
      :is-mobile="isMobile"
      :ai-enabled="aiEnabled"
      :config-loading="configLoading"
      :generating="generating"
      :selected-file="selectedFile"
      :available-providers="availableProviders"
      :has-debug-context="hasDebugContext"
      :current-content="currentContent"
      :preview-language="previewLanguage"
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
      :on-generate="onGenerate"
      :on-cancel="onCancel"
      :on-apply="onApply"
    />
  </el-dialog>
</template>

<style scoped lang="scss">
:deep(.ai-dialog-shell) {
  max-width: 1520px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 18px;
}

:deep(.ai-dialog-shell .el-dialog__header) {
  padding: 16px 18px 8px;
  margin-right: 0;
}

:deep(.ai-dialog-shell .el-dialog__body) {
  flex: 1;
  min-height: 0;
  padding: 0 18px 18px;
  overflow: hidden;
}

:deep(.ai-dialog-shell.is-fullscreen) {
  border-radius: 0;
}

:deep(.ai-dialog-shell.is-fullscreen .el-dialog__header) {
  padding: 14px 14px 6px;
}

:deep(.ai-dialog-shell.is-fullscreen .el-dialog__body) {
  padding: 0 14px 14px;
  overflow: auto;
}
</style>
