<script setup lang="ts">
import { ArrowDown, ArrowUp, CopyDocument, Cpu, MagicStick, Promotion, RefreshRight, SwitchButton, Warning } from '@element-plus/icons-vue'
import { computed, defineAsyncComponent, nextTick, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { AICodeMode, AICodeResponseMode, AIProviderOption } from '@/api/ai'
import { copyText } from '@/utils/clipboard'

const MonacoDiffEditor = defineAsyncComponent(() => import('@/components/MonacoDiffEditor.vue'))
const MonacoEditor = defineAsyncComponent(() => import('@/components/MonacoEditor.vue'))

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

const props = withDefaults(defineProps<{
  isMobile: boolean
  variant?: 'page' | 'dialog'
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
}>(), {
  variant: 'page'
})

const isFullResult = computed(() => props.resultResponseMode === 'full')
const isPatchResult = computed(() => props.resultResponseMode === 'patch')
const canApply = computed(() => {
  const applicableContent = isPatchResult.value ? props.resultPreviewContent : props.resultContent
  return props.resultCanApply && Boolean(applicableContent.trim())
})
const showStreamingDiffViewer = computed(() => {
  return props.generating && isFullResult.value && Boolean(props.selectedFile.trim()) && Boolean(props.resultContent.trim())
})
const showStreamingPreview = computed(() => {
  return props.generating && Boolean(props.resultContent.trim()) && !showStreamingDiffViewer.value
})
const hasResultDisplay = computed(() => {
  return Boolean(
    props.resultContent.trim() ||
    props.resultSummary.trim() ||
    props.resultWarnings.length ||
    props.generating
  )
})
const showAdvancedConfig = ref(false)
const showSummary = ref(true)
const showContextDetails = ref(false)
const showRawPatchResult = ref(false)
const streamingPreviewRef = ref<HTMLElement>()
const showPatchDiffPreview = computed(() => isPatchResult.value && Boolean(props.resultPreviewContent.trim()))
const diffModifiedContent = computed(() => showPatchDiffPreview.value ? props.resultPreviewContent : props.resultContent)
const streamingDiffModifiedContent = computed(() => mergeStreamingDiffContent(props.currentContent, props.resultContent))
const showDiffViewer = computed(() => {
  if (isFullResult.value) {
    return Boolean(props.resultContent.trim())
  }
  return showPatchDiffPreview.value
})
const hasModelOverride = computed(() => Boolean(modelOverride.value.trim()))
const fullResponseLabel = computed(() => props.selectedFile ? '修改后预览' : '生成结果')
const fullResponseEmptyDescription = computed(() => {
  if (props.selectedFile) {
    return '填写指令后点击“生成建议”，这里会显示 AI 返回的修改摘要和前后差异。'
  }
  return '填写指令后点击“生成建议”，这里会显示 AI 返回的生成结果。'
})
const fullResponseDebugHint = computed(() => {
  if (props.selectedFile) {
    return '仅修改后预览模式支持保存后立即启动调试。'
  }
  return '仅生成结果模式支持保存后立即启动调试。'
})
const configMetaText = computed(() => {
  if (props.selectedFile) {
    return '补丁模式会基于当前脚本返回 unified diff；修改后预览会直接显示前后差异。'
  }
  return '未选择脚本时可返回生成结果或解释说明。'
})
const displayLanguageLabel = computed(() => {
  const currentValue = String(props.selectedFile ? props.previewLanguage : manualLanguage.value || '').trim().toLowerCase()
  const matched = languageOptions.find(item => item.value === currentValue)
  if (matched) {
    return matched.label
  }
  if (!currentValue) {
    return 'Python'
  }
  return currentValue.charAt(0).toUpperCase() + currentValue.slice(1)
})
const debugContextLabel = computed(() => {
  if (!props.hasDebugContext) {
    return '无调试日志'
  }
  return includeDebugLogs.value ? '已附带调试' : '未附带调试'
})
const resultEmptyDescription = computed(() => {
  if (responseMode.value === 'patch') {
    return '填写指令后点击“生成建议”，这里会显示 AI 返回的补丁文本。'
  }
  if (responseMode.value === 'explain') {
    return '填写指令后点击“生成建议”，这里会显示 AI 返回的问题分析与建议。'
  }
  return fullResponseEmptyDescription.value
})

const promptPresets = computed(() => {
  switch (mode.value) {
    case 'modify':
      return ['优化性能', '添加错误处理', '重构为函数式', '添加日志输出']
    case 'fix':
      return ['修复报错并解释原因', '修复后添加防护逻辑']
    case 'generate':
      return ['生成 Python 定时任务脚本', '生成 Shell 监控脚本', '生成数据备份脚本']
    default:
      return []
  }
})

function appendPreset(text: string) {
  const current = prompt.value.trim()
  prompt.value = current ? `${current}\n${text}` : text
}

const languageOptions = [
  { label: 'Python', value: 'python' },
  { label: 'JavaScript', value: 'javascript' },
  { label: 'TypeScript', value: 'typescript' },
  { label: 'Shell', value: 'shell' },
  { label: 'Go', value: 'go' },
  { label: 'JSON', value: 'json' }
]

watch(hasModelOverride, (value) => {
  if (value) {
    showAdvancedConfig.value = true
  }
}, { immediate: true })

watch(mode, (value) => {
  if ((value === 'modify' || value === 'fix') && props.selectedFile) {
    responseMode.value = 'patch'
  } else if (value === 'generate') {
    responseMode.value = 'full'
  }
})

watch(() => props.resultContent, (value, previousValue) => {
  if (value && value !== previousValue) {
    showSummary.value = true
  }
  if (props.generating && value !== previousValue) {
    void nextTick(() => {
      const container = streamingPreviewRef.value
      if (!container) {
        return
      }
      container.scrollTop = container.scrollHeight
    })
  }
})

watch(() => props.resultResponseMode, (value) => {
  if (value !== 'patch') {
    showRawPatchResult.value = false
  }
}, { immediate: true })

function mergeStreamingDiffContent(originalContent: string, partialContent: string) {
  const normalizedOriginal = normalizeStreamingLineEndings(originalContent)
  const normalizedPartial = normalizeStreamingLineEndings(partialContent)

  if (!normalizedOriginal.trim() || !normalizedPartial.trim()) {
    return partialContent
  }

  const originalLines = normalizedOriginal.split('\n')
  const partialLines = normalizedPartial.split('\n')
  const partialEndsWithNewline = normalizedPartial.endsWith('\n')

  let completedLineCount = partialLines.length
  if (!partialEndsWithNewline) {
    completedLineCount = Math.max(0, partialLines.length - 1)
  }

  const mergedLines: string[] = []
  if (completedLineCount > 0) {
    mergedLines.push(...partialLines.slice(0, completedLineCount))
  }

  let originalIndex = completedLineCount
  if (!partialEndsWithNewline) {
    const partialLastLine = partialLines[partialLines.length - 1] || ''
    const originalLine = originalLines[originalIndex] || ''
    if (partialLastLine && originalLine.startsWith(partialLastLine)) {
      mergedLines.push(partialLastLine + originalLine.slice(partialLastLine.length))
    } else {
      mergedLines.push(partialLastLine)
    }
    originalIndex += 1
  }

  if (originalIndex < originalLines.length) {
    mergedLines.push(...originalLines.slice(originalIndex))
  }

  return mergedLines.join('\n')
}

async function handleCopyResult() {
  const text = props.resultPreviewContent || props.resultContent
  if (!text) return
  try {
    await copyText(text)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败')
  }
}

function normalizeStreamingLineEndings(value: string) {
  return String(value || '').replace(/\r\n/g, '\n')
}
</script>

<template>
  <div class="ai-workspace" :class="[variant, { mobile: isMobile }]">
    <el-alert
      v-if="!aiEnabled"
      type="warning"
      :closable="false"
      show-icon
      title="AI 助手尚未启用，请先到系统设置里开启并配置模型。"
    />

    <el-alert
      v-else-if="!availableProviders.length"
      type="warning"
      :closable="false"
      show-icon
      title="当前没有可用的 AI 提供商，请先在系统设置中填写至少一组 API Key 和模型。"
    />

    <div class="overview-strip">
      <div class="overview-item overview-item--wide overview-item--script">
        <div class="script-overview-copy">
          <span class="overview-label">当前脚本</span>
          <strong>{{ selectedFile || '未选择脚本，将按生成新脚本处理' }}</strong>
        </div>
        <div class="script-overview-options">
          <div class="option-stack">
            <span class="config-label">操作模式</span>
            <el-radio-group v-model="mode" class="mode-group mode-group--compact">
              <el-radio-button label="generate">生成新脚本</el-radio-button>
              <el-radio-button label="modify">修改当前脚本</el-radio-button>
              <el-radio-button label="fix">修复脚本报错</el-radio-button>
            </el-radio-group>
          </div>

          <div class="option-stack">
            <span class="config-label">返回方式</span>
            <el-radio-group v-model="responseMode" class="mode-group mode-group--compact">
              <el-radio-button label="full">{{ fullResponseLabel }}</el-radio-button>
              <el-radio-button label="patch" :disabled="!selectedFile">只返回补丁</el-radio-button>
              <el-radio-button label="explain">只解释问题</el-radio-button>
            </el-radio-group>
          </div>
        </div>
      </div>

      <div class="overview-item overview-item--provider">
        <div class="provider-head">
          <span class="overview-label">AI 提供商</span>
          <div class="provider-actions">
            <el-tag v-if="hasModelOverride" size="small" type="warning" effect="plain">临时模型</el-tag>
            <el-button text class="advanced-toggle" @click="showAdvancedConfig = !showAdvancedConfig">
              <el-icon><component :is="showAdvancedConfig ? ArrowUp : ArrowDown" /></el-icon>
              <span>{{ showAdvancedConfig ? '收起高级配置' : '高级配置' }}</span>
            </el-button>
          </div>
        </div>
        <el-select v-model="provider" style="width: 100%" :loading="configLoading" placeholder="选择一个已配置的提供商">
          <el-option
            v-for="item in availableProviders"
            :key="item.id"
            :label="`${item.label}${item.model ? ` (${item.model})` : ''}`"
            :value="String(item.id)"
          />
        </el-select>
        <el-collapse-transition>
          <div v-show="showAdvancedConfig" class="advanced-panel advanced-panel--inline">
            <div class="advanced-grid">
              <div class="form-field config-field">
                <label>临时覆盖模型</label>
                <el-input v-model="modelOverride" placeholder="留空使用系统设置中的默认模型" />
              </div>

              <div class="form-field config-field">
                <label>目标脚本路径</label>
                <el-input v-model="targetPath" placeholder="如 demo/test.py 或 scripts/fix.sh" />
                <span class="form-hint">留空时会优先使用当前选中的脚本路径</span>
              </div>

              <div class="form-field config-field">
                <label>应用后自动调试</label>
                <div class="advanced-switch-row">
                  <div class="advanced-switch-copy">
                    <span class="toggle-desc">
                      {{ responseMode === 'full' ? fullResponseDebugHint : '当前模式不会直接写入脚本，因此无法自动调试。' }}
                    </span>
                  </div>
                  <el-switch
                    v-model="autoDebugAfterApply"
                    :disabled="responseMode !== 'full'"
                    inline-prompt
                    active-text="开启"
                    inactive-text="关闭"
                  />
                </div>
              </div>
            </div>
          </div>
        </el-collapse-transition>
      </div>
    </div>

    <div class="config-meta">
      {{ configMetaText }}
    </div>

    <div class="workspace-grid">
      <div class="workspace-panel request-panel">
        <div class="panel-title">
          <div class="panel-title-main">
            <el-icon><Promotion /></el-icon>
            <span class="panel-title-text">指令与上下文</span>
          </div>
          <div class="panel-title-actions">
            <el-tag size="small" effect="plain" type="info">
              {{ hasConversation ? `已累计 ${conversationTurnCount} 轮` : '当前是新对话' }}
            </el-tag>
            <el-radio-group v-model="conversationMode" class="conversation-switch" size="small">
              <el-radio-button label="continue">继续对话</el-radio-button>
              <el-radio-button label="restart">新对话</el-radio-button>
            </el-radio-group>
          </div>
        </div>

        <div class="panel-body request-body">
          <div class="conversation-note">
            <el-icon><Cpu /></el-icon>
            <span v-if="conversationMode === 'restart'">
              下一次生成会从当前脚本重新开始，不带上之前的 AI 上下文。
            </span>
            <span v-else-if="hasConversation">
              下一次生成会基于上一轮 AI 结果继续改进，适合连续细调同一份脚本。
            </span>
            <span v-else>
              首次生成后会自动进入继续对话，后续可以在当前结果上持续迭代。
            </span>
          </div>

          <div class="form-field">
            <label>AI 指令</label>
            <div class="field-tip">描述目标、约束、保留逻辑和运行环境，结果会更稳定。</div>
            <div v-if="promptPresets.length" class="prompt-presets">
              <el-tag
                v-for="preset in promptPresets"
                :key="preset"
                size="small"
                effect="plain"
                class="preset-tag"
                @click="appendPreset(preset)"
              >{{ preset }}</el-tag>
            </div>
            <el-input
              v-model="prompt"
              type="textarea"
              :rows="isMobile ? 10 : 11"
              resize="vertical"
              class="prompt-input"
              placeholder="描述你希望 AI 如何修改、生成或修复脚本。"
            />
          </div>

          <div class="context-toolbar">
            <div class="context-tags">
              <el-tag size="small" effect="plain" type="info">
                {{ selectedFile ? `语言 ${displayLanguageLabel}` : `生成 ${displayLanguageLabel}` }}
              </el-tag>
              <el-tag size="small" effect="plain" :type="includeDebugLogs && hasDebugContext ? 'success' : 'info'">
                {{ debugContextLabel }}
              </el-tag>
            </div>
            <el-button text class="context-toggle" @click="showContextDetails = !showContextDetails">
              <el-icon><component :is="showContextDetails ? ArrowUp : ArrowDown" /></el-icon>
              <span>{{ showContextDetails ? '收起补充上下文' : '补充上下文' }}</span>
            </el-button>
          </div>

          <el-collapse-transition>
            <div v-show="showContextDetails" class="context-detail-panel">
              <div class="request-context-stack">
                <div class="form-field context-inline-field">
                  <label>{{ selectedFile ? '当前脚本语言' : '生成语言' }}</label>
                  <div class="context-inline-card">
                    <template v-if="selectedFile">
                      <div class="language-static">
                        <el-tag type="info" effect="plain">{{ displayLanguageLabel }}</el-tag>
                        <span class="form-hint">已选中文件时会沿用当前脚本语言</span>
                      </div>
                    </template>
                    <template v-else>
                      <el-select v-model="manualLanguage" style="width: 100%" class="language-select">
                        <el-option v-for="item in languageOptions" :key="item.value" :label="item.label" :value="item.value" />
                      </el-select>
                    </template>
                  </div>
                </div>

                <div class="form-field context-inline-field">
                  <label>调试上下文</label>
                  <div class="context-inline-card context-inline-card--switch">
                    <div class="advanced-switch-copy">
                      <span class="toggle-desc">
                        {{ hasDebugContext ? '把最近一次调试日志、退出码和错误带给 AI。' : '当前没有可附带的调试日志。' }}
                      </span>
                    </div>
                    <el-switch
                      v-model="includeDebugLogs"
                      :disabled="!hasDebugContext"
                      inline-prompt
                      active-text="带上"
                      inactive-text="不带"
                    />
                  </div>
                </div>
              </div>

              <div class="context-note">
                <el-icon><Warning /></el-icon>
                <span>AI 会读取当前脚本、你选择附带的调试日志，以及继续对话时的上一轮 AI 结果，用来生成新的建议。</span>
              </div>
            </div>
          </el-collapse-transition>
        </div>
      </div>

      <div class="workspace-panel result-panel">
        <div class="panel-title">
          <div class="panel-title-main">
            <el-icon><MagicStick /></el-icon>
            <span class="panel-title-text">结果预览</span>
            <el-tag v-if="resultProviderLabel" size="small" effect="plain">{{ resultProviderLabel }}</el-tag>
            <el-tag v-if="resultModel" size="small" effect="plain">{{ resultModel }}</el-tag>
          </div>
          <div class="panel-title-actions">
            <el-button
              v-if="resultContent && !generating"
              text
              size="small"
              @click="handleCopyResult"
            >
              <el-icon><CopyDocument /></el-icon>
              <span>复制结果</span>
            </el-button>
            <el-button
              v-if="resultContent"
              text
              class="summary-toggle"
              @click="showSummary = !showSummary"
            >
              <el-icon><component :is="showSummary ? ArrowUp : ArrowDown" /></el-icon>
              <span>{{ showSummary ? '收起摘要' : '展开摘要' }}</span>
            </el-button>
          </div>
        </div>

        <div v-if="generationError" class="result-error-row">
          <el-alert
            type="error"
            :closable="false"
            show-icon
            :title="generationError"
            class="result-error"
          />
          <el-button size="small" @click="onGenerate">
            <el-icon><RefreshRight /></el-icon>重试
          </el-button>
        </div>

        <div v-if="hasResultDisplay" class="panel-body result-body">
          <el-collapse-transition>
            <div v-show="showSummary && (resultSummary || resultWarnings.length || generating)" class="summary-card">
              <div class="summary-title">AI 摘要</div>
              <p class="summary-text">{{ resultSummary || (generating ? 'AI 正在生成中，结果会实时刷新到预览区。' : 'AI 已返回脚本建议。') }}</p>
              <div v-if="resultWarnings.length" class="warning-list">
                <div v-for="(warning, index) in resultWarnings" :key="`${warning}-${index}`" class="warning-item">
                  <el-icon><Warning /></el-icon>
                  <span>{{ warning }}</span>
                </div>
              </div>
            </div>
          </el-collapse-transition>

          <el-alert
            v-if="generating"
            type="info"
            :closable="false"
            show-icon
            title="正在接收模型输出，预览区会边生成边更新。"
            class="inline-alert"
          />

          <div v-if="showStreamingDiffViewer" class="text-result-shell">
            <el-alert
              type="info"
              :closable="false"
              show-icon
              title="正在实时比对当前脚本的修改内容，预览区会优先展示已发生变化的位置。"
              class="inline-alert"
            />

            <div class="result-viewer">
              <MonacoDiffEditor
                :original-value="currentContent"
                :modified-value="streamingDiffModifiedContent"
                :language="previewLanguage"
                :render-side-by-side="!isMobile"
                :hide-unchanged-regions="true"
                :context-line-count="3"
              />
            </div>
          </div>

          <div v-else-if="showStreamingPreview" class="text-result-shell">
            <el-alert
              type="info"
              :closable="false"
              show-icon
              :title="isFullResult ? '正在实时预览生成内容，完成后会自动切换为差异对比。' : '正在实时接收模型输出，文本会像打字一样持续更新。'"
              class="inline-alert"
            />

            <div ref="streamingPreviewRef" class="streaming-preview-shell">
              <div class="streaming-preview-head">
                <span class="streaming-status-dot"></span>
                <span>实时生成中</span>
              </div>
              <pre class="streaming-preview-text">{{ resultContent }}<span class="streaming-caret" aria-hidden="true"></span></pre>
            </div>
          </div>

          <div v-else-if="showDiffViewer" class="text-result-shell">
            <el-alert
              v-if="showPatchDiffPreview"
              type="success"
              :closable="false"
              show-icon
              title="当前为补丁模式，主预览区显示补丁应用后的前后对比，未改动区域会自动折叠。"
              class="inline-alert"
            />
            <el-alert
              v-else-if="isFullResult"
              type="info"
              :closable="false"
              show-icon
              :title="selectedFile ? '修改后预览模式仍会保存整份脚本，但预览区默认只聚焦改动片段，未改动区域会自动折叠。' : '生成结果模式会展示最终脚本；如果后续继续对话，会基于当前生成结果继续修改。'"
              class="inline-alert"
            />

            <div v-if="showPatchDiffPreview" class="patch-preview-toolbar">
              <span class="patch-preview-tip">默认展示修改前后对比，原始 unified diff 已折叠收起。</span>
              <el-button text class="patch-toggle" @click="showRawPatchResult = !showRawPatchResult">
                <el-icon><component :is="showRawPatchResult ? ArrowUp : ArrowDown" /></el-icon>
                <span>{{ showRawPatchResult ? '收起原始补丁' : '查看原始补丁' }}</span>
              </el-button>
            </div>

            <div class="result-viewer">
              <MonacoDiffEditor
                :original-value="currentContent"
                :modified-value="diffModifiedContent"
                :language="previewLanguage"
                :render-side-by-side="!isMobile"
                :hide-unchanged-regions="true"
                :context-line-count="3"
              />
            </div>

            <el-collapse-transition>
              <div v-if="showPatchDiffPreview" v-show="showRawPatchResult" class="raw-patch-shell">
                <div class="raw-patch-label">原始 unified diff</div>
                <div class="raw-patch-viewer">
                  <MonacoEditor
                    :model-value="resultContent"
                    language="diff"
                    :readonly="true"
                    class="text-output-editor"
                  />
                </div>
              </div>
            </el-collapse-transition>
          </div>

          <div v-else-if="resultContent" class="text-result-shell">
            <el-alert
              v-if="isPatchResult"
              type="warning"
              :closable="false"
              show-icon
              title="当前补丁还不能直接预演，所以先展示原始 patch 文本。建议重试，或让 AI 只改更小的范围。"
              class="inline-alert"
            />
            <el-alert
              v-else
              type="info"
              :closable="false"
              show-icon
              title="当前为解释模式，只返回分析和建议，不会直接应用到脚本。"
              class="inline-alert"
            />

            <div class="result-viewer">
              <MonacoEditor
                :model-value="resultContent"
                :language="isPatchResult ? 'diff' : 'markdown'"
                :readonly="true"
                class="text-output-editor"
              />
            </div>
          </div>

          <div v-else-if="generating" class="streaming-empty-state">
            <el-empty description="模型已开始生成，结果将在这里实时出现。" :image-size="84" />
          </div>
        </div>

        <div v-else class="panel-body empty-body">
          <el-empty :description="resultEmptyDescription" :image-size="92" />
        </div>
      </div>
    </div>

    <div class="workspace-footer">
      <div class="footer-note">
        <el-icon><Promotion /></el-icon>
        <span>AI 结果不会直接写入脚本，先预览，再由你应用。</span>
      </div>
      <div class="footer-actions">
        <el-button v-if="generating" type="danger" @click="onCancel">
          <el-icon><SwitchButton /></el-icon>取消生成
        </el-button>
        <el-button v-else type="primary" @click="onGenerate">
          <el-icon><MagicStick /></el-icon>生成建议
        </el-button>
        <el-button :disabled="!canApply" @click="onApply">
          <el-icon><SwitchButton /></el-icon>{{ applyButtonText }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.ai-workspace {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
}

.ai-workspace.dialog {
  height: min(80vh, 900px);
}

.ai-workspace.page {
  height: 100%;
}

.overview-strip {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(260px, 0.95fr);
  gap: 12px;
}

.overview-item,
.workspace-panel {
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
}

.overview-item {
  min-width: 0;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;

  strong {
    font-size: 14px;
    line-height: 1.55;
    color: var(--el-text-color-primary);
    word-break: break-all;
  }
}

.overview-item--wide {
  background: linear-gradient(180deg, color-mix(in srgb, var(--el-color-primary-light-9) 46%, white) 0%, var(--el-bg-color) 100%);
}

.overview-item--provider {
  justify-content: flex-start;
}

.overview-item--script {
  gap: 14px;
}

.script-overview-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.script-overview-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.option-stack {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.overview-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  letter-spacing: 0.03em;
}

.workspace-panel {
  min-height: 0;
  padding: 14px;
  display: flex;
  flex-direction: column;
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 14px;
}

.panel-title-main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.panel-title-text {
  white-space: nowrap;
  flex-shrink: 0;
}

.panel-title-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.panel-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.config-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.config-field {
  margin-bottom: 0;
}

.provider-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.provider-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-toggle {
  padding-inline: 0;
  font-size: 13px;
}

.advanced-panel {
  padding: 12px;
  border: 1px dashed var(--el-border-color);
  border-radius: 10px;
  background: var(--el-fill-color-extra-light);
}

.advanced-panel--inline {
  margin-top: 10px;
}

.advanced-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.advanced-panel--inline .advanced-grid {
  grid-template-columns: 1fr;
  gap: 10px;
}

.advanced-switch-row {
  min-height: 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 10px;
  background: var(--el-bg-color);
}

.advanced-switch-copy {
  min-width: 0;
}

.config-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
  padding-inline: 2px;
}

.workspace-grid {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(210px, 0.34fr) minmax(0, 1.96fr);
  gap: 14px;
}

.form-field {
  min-width: 0;

  label {
    display: block;
    margin-bottom: 8px;
    font-size: 14px;
    color: var(--el-text-color-primary);
  }
}

.form-field--full {
  grid-column: 1 / -1;
}

.form-hint,
.field-tip {
  display: block;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

.field-tip {
  margin-bottom: 8px;
}

.mode-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mode-group--compact {
  gap: 6px;
}

.request-body {
  gap: 14px;
  overflow: auto;
  padding-right: 4px;
}

.context-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--el-fill-color-extra-light);
}

.context-tags {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.context-toggle {
  padding-inline: 0;
  font-size: 13px;
}

.context-detail-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.conversation-switch {
  flex-wrap: wrap;
}

.conversation-note {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--el-color-primary-light-9) 58%, white 42%);
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.request-context-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.context-inline-field {
  min-width: 0;
}

.context-inline-card {
  min-height: 48px;
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 10px;
  background: var(--el-fill-color-extra-light);
}

.context-inline-card--switch {
  justify-content: space-between;
  gap: 12px;
}

.language-static {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.language-select {
  width: 100%;
}

.language-select :deep(.el-select__wrapper) {
  min-height: 42px;
  border-radius: 10px;
  background: var(--el-bg-color);
  box-shadow: 0 0 0 1px var(--el-border-color-light) inset;
}

.prompt-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}

.preset-tag {
  cursor: pointer;
  transition: background-color 0.15s;
}

.preset-tag:hover {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.prompt-input :deep(.el-textarea__inner) {
  min-height: clamp(260px, 34vh, 420px);
  line-height: 1.65;
}

.toggle-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.toggle-desc {
  font-size: 12px;
  line-height: 1.55;
  color: var(--el-text-color-secondary);
}

.summary-toggle {
  margin-left: auto;
  padding-inline: 0;
  font-size: 13px;
}

.context-note {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--el-fill-color-extra-light);
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.result-panel {
  min-width: 0;
}

.result-error-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.result-error-row .result-error {
  flex: 1;
  margin-bottom: 0;
}

.result-body {
  overflow: auto;
  padding-right: 4px;
}

.empty-body {
  align-items: center;
  justify-content: center;
}

.streaming-empty-state {
  flex: 1;
  min-height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.summary-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 10px;
  background: var(--el-fill-color-extra-light);
  padding: 12px 14px;
  margin-bottom: 10px;
  flex-shrink: 0;
}

.summary-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.summary-text {
  margin: 0;
  font-size: 14px;
  line-height: 1.65;
  color: var(--el-text-color-primary);
}

.warning-list {
  margin-top: 12px;
}

.warning-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--el-color-warning-dark-2);
  font-size: 13px;

  & + & {
    margin-top: 6px;
  }
}

.text-result-shell {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.patch-preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.patch-preview-tip {
  min-width: 0;
  line-height: 1.5;
}

.patch-toggle {
  flex-shrink: 0;
}

.raw-patch-shell {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 10px;
  background: var(--el-fill-color-extra-light);
}

.raw-patch-label {
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
}

.raw-patch-viewer {
  min-height: 220px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  overflow: hidden;
}

.streaming-preview-shell {
  flex: 1;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 10px;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.14), transparent 28%),
    var(--dd-editor-bg-color, #111827);
  color: var(--dd-editor-fg-color, #e5e7eb);
  overflow: auto;
  position: relative;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.streaming-preview-head {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: rgba(191, 219, 254, 0.92);
  flex-shrink: 0;
}

.streaming-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #34d399;
  box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.45);
  animation: streamingPulse 1.3s ease-out infinite;
}

.streaming-preview-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: Monaco, Consolas, 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.72;
  color: inherit;
}

.streaming-caret {
  display: inline-block;
  width: 9px;
  height: 1.1em;
  margin-left: 2px;
  vertical-align: -0.18em;
  border-radius: 2px;
  background: #60a5fa;
  box-shadow: 0 0 12px rgba(96, 165, 250, 0.4);
  animation: streamingCaretBlink 0.92s steps(1, end) infinite;
}

.inline-alert {
  margin-bottom: 12px;
}

.result-viewer {
  flex: 1;
  min-height: 320px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 10px;
  overflow: hidden;
}

.text-output-editor {
  height: 100%;
  min-height: 0;
}

.result-viewer :deep(.monaco-editor-wrapper),
.result-viewer :deep(.monaco-diff-wrapper) {
  min-height: 0;
}

.result-viewer :deep(.monaco-editor-container),
.result-viewer :deep(.monaco-diff-container) {
  min-height: 0;
}

.workspace-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
}

.footer-note {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ai-workspace.mobile {
  .overview-strip,
  .workspace-grid,
  .script-overview-options,
  .advanced-grid {
    grid-template-columns: 1fr;
  }

  .workspace-panel,
  .workspace-footer {
    padding: 14px;
  }

  .request-body {
    overflow: visible;
    padding-right: 0;
  }

  .workspace-footer,
  .footer-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .provider-head,
  .panel-title {
    flex-direction: column;
    align-items: flex-start;
  }

  .panel-title-actions {
    width: 100%;
    justify-content: space-between;
  }

  .context-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .provider-actions {
    width: 100%;
    justify-content: space-between;
  }

  .summary-toggle {
    margin-left: 0;
  }

  .streaming-preview-shell {
    min-height: 240px;
    padding: 12px 13px;
  }

  .patch-preview-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
}

.ai-workspace.dialog {
  gap: 12px;

  .overview-strip {
    grid-template-columns: minmax(0, 1.8fr) minmax(240px, 0.9fr);
    gap: 10px;
  }

  .overview-item {
    padding: 10px 12px;
  }

  .overview-item--script {
    gap: 12px;
  }

  .workspace-panel {
    padding: 12px;
  }

  .panel-title {
    margin-bottom: 10px;
  }

  .advanced-panel {
    padding: 10px 12px;
  }

  .advanced-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .config-meta {
    font-size: 11px;
  }

  .workspace-grid {
    grid-template-columns: minmax(200px, 0.3fr) minmax(0, 2.04fr);
    gap: 12px;
  }

  .request-body {
    gap: 12px;
  }

  .provider-actions {
    gap: 6px;
  }

  .request-context-stack {
    gap: 10px;
  }

  .prompt-input :deep(.el-textarea__inner) {
    min-height: clamp(220px, 24vh, 320px);
  }

  .result-viewer {
    min-height: 280px;
  }

  .summary-card {
    padding: 10px 12px;
    margin-bottom: 8px;
  }

  .workspace-footer {
    padding: 12px 14px;
  }
}

.ai-workspace.dialog.mobile {
  .advanced-grid {
    grid-template-columns: 1fr;
  }
}

@keyframes streamingCaretBlink {
  0%,
  46% {
    opacity: 1;
  }

  47%,
  100% {
    opacity: 0.08;
  }
}

@keyframes streamingPulse {
  0% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.45);
  }

  70% {
    transform: scale(1.12);
    box-shadow: 0 0 0 10px rgba(52, 211, 153, 0);
  }

  100% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(52, 211, 153, 0);
  }
}
</style>
