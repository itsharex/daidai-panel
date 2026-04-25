import { computed, ref, watch, type ComputedRef, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { aiApi, type AICodeConversationTurn, type AICodeGenerateResponse, type AICodeMode, type AICodeResponseMode, type AIProviderOption } from '@/api/ai'
import { scriptApi } from '@/api/script'
import { openAuthorizedEventStream, type EventStreamConnection } from '@/utils/sse'

interface UseScriptAIOptions {
  selectedFile: Ref<string>
  fileContent: Ref<string>
  isBinary: Ref<boolean>
  isEditing: Ref<boolean>
  hasChanges: ComputedRef<boolean>
  editorLanguage: ComputedRef<string>
  loadTree: () => Promise<void>
  loadFileContent: (path: string) => Promise<boolean>
  debugLogs: Ref<string[]>
  debugExitCode: Ref<number | null>
  debugError: Ref<string>
  openDebugAndStart: (options?: { useEditorContent?: boolean }) => Promise<void>
  applyMode?: 'editor' | 'save'
}

type AICodeConversationMode = 'continue' | 'restart'

export function useScriptAI({
  selectedFile,
  fileContent,
  isBinary,
  isEditing,
  hasChanges,
  editorLanguage,
  loadTree,
  loadFileContent,
  debugLogs,
  debugExitCode,
  debugError,
  openDebugAndStart,
  applyMode = 'editor'
}: UseScriptAIOptions) {
  const showAIDialog = ref(false)
  const aiEnabled = ref(false)
  const configLoading = ref(false)
  const generating = ref(false)

  const providerOptions = ref<AIProviderOption[]>([])
  const provider = ref('')
  const modelOverride = ref('')
  const mode = ref<AICodeMode>('modify')
  const responseMode = ref<AICodeResponseMode>('full')
  const prompt = ref('')
  const targetPath = ref('')
  const manualLanguage = ref('python')
  const includeDebugLogs = ref(true)
  const autoDebugAfterApply = ref(false)
  const conversationMode = ref<AICodeConversationMode>('continue')
  const conversationTurns = ref<AICodeConversationTurn[]>([])
  const conversationBaseContent = ref('')
  const previewBaseContent = ref('')

  const resultSummary = ref('')
  const resultContent = ref('')
  const resultPreviewContent = ref('')
  const resultWarnings = ref<string[]>([])
  const resultProviderLabel = ref('')
  const resultModel = ref('')
  const resultResponseMode = ref<AICodeResponseMode>('full')
  const resultCanApply = ref(false)
  const generationError = ref('')
  const streamRawOutput = ref('')
  let generationStream: EventStreamConnection | null = null

  const configuredProviders = computed(() => providerOptions.value.filter(item => item.configured))
  const hasDebugContext = computed(() => debugLogs.value.length > 0 || debugExitCode.value !== null || !!debugError.value)
  const hasConversation = computed(() => conversationTurns.value.length > 0)
  const conversationTurnCount = computed(() => conversationTurns.value.length)
  const resolvedLanguage = computed(() => {
    if (selectedFile.value) {
      return normalizeEditorLanguage(editorLanguage.value)
    }
    return normalizeEditorLanguage(manualLanguage.value)
  })
  const applyButtonText = computed(() => {
    const normalizedTarget = targetPath.value.trim() || selectedFile.value.trim()
    if (selectedFile.value && normalizedTarget === selectedFile.value) {
      return applyMode === 'save' ? '保存到当前脚本' : '应用到编辑器'
    }
    return '保存到脚本目录'
  })

  const resetResultState = () => {
    resultSummary.value = ''
    resultContent.value = ''
    resultPreviewContent.value = ''
    resultWarnings.value = []
    resultProviderLabel.value = ''
    resultModel.value = ''
    resultResponseMode.value = 'full'
    resultCanApply.value = false
    previewBaseContent.value = ''
    streamRawOutput.value = ''
  }

  const closeGenerationStream = () => {
    if (!generationStream) {
      return
    }
    generationStream.close()
    generationStream = null
  }

  const resetConversationState = () => {
    conversationMode.value = 'continue'
    conversationTurns.value = []
    conversationBaseContent.value = ''
    generationError.value = ''
    resetResultState()
  }

  watch(selectedFile, (value, previousValue) => {
    if (value === previousValue) {
      return
    }
    closeGenerationStream()
    targetPath.value = normalizeScriptPath(value)
    resetConversationState()
  })

  watch(showAIDialog, (value) => {
    if (!value) {
      closeGenerationStream()
    }
  })

  async function loadAIConfig() {
    configLoading.value = true
    try {
      const res = await aiApi.config()
      aiEnabled.value = !!res.data?.enabled
      providerOptions.value = Array.isArray(res.data?.providers) ? res.data.providers : []

      const defaultProvider = String(res.data?.default_provider || '')
      if (!provider.value || !providerOptions.value.some(item => item.id === provider.value && item.configured)) {
        provider.value = pickProvider(defaultProvider, providerOptions.value)
      }
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '加载 AI 配置失败')
    } finally {
      configLoading.value = false
    }
  }

  function openAIDialogFor(modeValue: AICodeMode = selectedFile.value ? 'modify' : 'generate') {
    if (selectedFile.value && isBinary.value) {
      ElMessage.warning('当前是二进制文件，暂不支持 AI 修改')
      return
    }

    showAIDialog.value = true
    mode.value = selectedFile.value ? modeValue : 'generate'
    if (selectedFile.value && (mode.value === 'modify' || mode.value === 'fix')) {
      responseMode.value = 'patch'
    } else {
      responseMode.value = 'full'
    }
    if (selectedFile.value) {
      targetPath.value = normalizeScriptPath(selectedFile.value)
      manualLanguage.value = normalizeEditorLanguage(editorLanguage.value)
    } else if (!manualLanguage.value.trim()) {
      manualLanguage.value = 'python'
    }
    includeDebugLogs.value = hasDebugContext.value
    autoDebugAfterApply.value = !!selectedFile.value && mode.value === 'fix'
    generationError.value = ''

    if (!prompt.value.trim()) {
      prompt.value = defaultPrompt(mode.value)
    }

    void loadAIConfig()
  }

  async function handleGenerate() {
    if (!aiEnabled.value) {
      ElMessage.warning('AI 脚本助手尚未启用，请先在系统设置中配置')
      return
    }
    if (!configuredProviders.value.length) {
      ElMessage.warning('当前没有已配置的 AI 提供商，请先到系统设置中填写模型配置')
      return
    }
    if (!provider.value) {
      ElMessage.warning('请先选择一个已配置的 AI 提供商')
      return
    }
    if (!prompt.value.trim()) {
      ElMessage.warning('请输入本次 AI 需求')
      return
    }
    if (responseMode.value === 'patch' && !selectedFile.value) {
      ElMessage.warning('只返回补丁需要先选择当前脚本')
      return
    }
    if (selectedFile.value && isBinary.value && mode.value !== 'generate') {
      ElMessage.warning('当前是二进制文件，暂不支持 AI 修改或修复')
      return
    }

    const shouldRestartConversation = conversationMode.value === 'restart'
    const baseContentForRequest = resolveCurrentContentForRequest(
      shouldRestartConversation,
      selectedFile.value,
      isBinary.value,
      fileContent.value,
      conversationBaseContent.value
    )
    const historyForRequest = shouldRestartConversation ? [] : trimConversationHistory(conversationTurns.value)

    generating.value = true
    generationError.value = ''
    closeGenerationStream()
    resetResultState()
    resultResponseMode.value = responseMode.value
    previewBaseContent.value = baseContentForRequest
    const requestPayload = {
      provider: provider.value,
      model: modelOverride.value.trim() || undefined,
      mode: mode.value,
      response_mode: responseMode.value,
      prompt: prompt.value.trim(),
      language: resolvedLanguage.value,
      target_path: normalizeScriptPath(targetPath.value) || normalizeScriptPath(selectedFile.value),
      current_content: baseContentForRequest,
      debug_logs: includeDebugLogs.value ? debugLogs.value : [],
      debug_exit_code: includeDebugLogs.value ? debugExitCode.value : null,
      debug_error: includeDebugLogs.value ? debugError.value : '',
      conversation_history: historyForRequest
    }
    const providerOption = providerOptions.value.find(item => String(item.id) === provider.value)
    resultProviderLabel.value = providerOption?.label || ''
    resultModel.value = modelOverride.value.trim() || providerOption?.model || ''
    resultCanApply.value = false
    try {
      await new Promise<void>((resolve, reject) => {
        let settled = false
        generationStream = openAuthorizedEventStream(
          '/api/v1/ai-code/generate-stream',
          {
            onEvent: (event) => {
              if (settled) {
                return
              }
              if (event.event === 'delta') {
                const payload = parseAIStreamEventData<{ text?: string }>(event.data)
                const chunkText = String(payload?.text || '')
                if (!chunkText) {
                  return
                }
                streamRawOutput.value += chunkText
                applyStreamingPreview(streamRawOutput.value, resultResponseMode.value, resultSummary, resultContent)
                return
              }

              if (event.event === 'done') {
                settled = true
                closeGenerationStream()
                const payload = parseAIStreamEventData<{ data?: AICodeGenerateResponse }>(event.data)
                applyCompletedResult(
                  payload?.data as AICodeGenerateResponse | undefined,
                  shouldRestartConversation,
                  baseContentForRequest
                )
                resolve()
                return
              }

              if (event.event === 'error') {
                settled = true
                closeGenerationStream()
                const payload = parseAIStreamEventData<{ error?: string }>(event.data)
                reject(new Error(String(payload?.error || 'AI 生成失败')))
              }
            },
            onError: (error) => {
              if (settled) {
                return
              }
              settled = true
              closeGenerationStream()
              reject(error)
            }
          },
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json'
            },
            body: JSON.stringify(requestPayload)
          }
        )
      })
    } catch (err: any) {
      generationError.value = err?.response?.data?.error || err?.message || 'AI 生成失败'
      ElMessage.error(generationError.value)
    } finally {
      closeGenerationStream()
      generating.value = false
    }
  }

  function applyCompletedResult(
    data: AICodeGenerateResponse | undefined,
    shouldRestartConversation: boolean,
    baseContentForRequest: string
  ) {
    const nextResponseMode = (data?.response_mode || resultResponseMode.value || 'full') as AICodeResponseMode
    const nextContent = data?.content || ''
    const nextPreviewContent = data?.preview_content || (nextResponseMode === 'full' ? nextContent : '')
    const contentForApply = resolveApplicableResultContent(nextResponseMode, nextContent, nextPreviewContent)
    const contentForConversation = resolveConversationContent(nextResponseMode, nextContent, nextPreviewContent)

    resultSummary.value = data?.summary || ''
    resultContent.value = nextContent
    resultPreviewContent.value = nextPreviewContent
    resultWarnings.value = Array.isArray(data?.warnings) ? data!.warnings! : []
    resultProviderLabel.value = data?.provider_label || resultProviderLabel.value
    resultModel.value = data?.model || resultModel.value
    resultResponseMode.value = nextResponseMode
    resultCanApply.value = !!data?.can_apply && Boolean(contentForApply.trim())
    conversationTurns.value = trimConversationHistory([
      ...(shouldRestartConversation ? [] : conversationTurns.value),
      {
        mode: mode.value,
        response_mode: responseMode.value,
        prompt: prompt.value.trim(),
        summary: resultSummary.value,
        content: contentForConversation
      }
    ])
    if ((resultResponseMode.value === 'full' || resultResponseMode.value === 'patch') && contentForConversation.trim()) {
      conversationBaseContent.value = contentForConversation
    } else if (shouldRestartConversation) {
      conversationBaseContent.value = baseContentForRequest
    }
    conversationMode.value = 'continue'
  }

  async function handleApply() {
    if (!resultCanApply.value) {
      ElMessage.warning('当前结果模式不支持直接应用，请切换到可保存的预览模式')
      return
    }

    const content = resolveApplicableResultContent(resultResponseMode.value, resultContent.value, resultPreviewContent.value)
    if (!content.trim()) {
      ElMessage.warning('当前没有可应用的 AI 结果')
      return
    }

    const currentPath = normalizeScriptPath(selectedFile.value)
    const desiredPath = normalizeScriptPath(targetPath.value) || currentPath
    if (!desiredPath) {
      ElMessage.warning('请填写目标脚本路径')
      return
    }

    if (currentPath && desiredPath === currentPath) {
      if (applyMode === 'save') {
        try {
          await ElMessageBox.confirm(
            `确认将 AI 结果覆盖保存到 ${desiredPath} 吗？`,
            '保存 AI 脚本',
            {
              type: 'info',
              confirmButtonText: '确认保存',
              cancelButtonText: '取消'
            }
          )
        } catch {
          return
        }

        try {
          await scriptApi.saveContent(desiredPath, content, buildAIVersionMessage(mode.value))
          await loadTree()
          selectedFile.value = desiredPath
          await loadFileContent(desiredPath)
          isEditing.value = false
          showAIDialog.value = false
          ElMessage.success(autoDebugAfterApply.value ? 'AI 结果已保存，正在启动调试' : 'AI 结果已保存到当前脚本')
          if (autoDebugAfterApply.value) {
            await openDebugAndStart()
          }
        } catch (err: any) {
          ElMessage.error(err?.response?.data?.error || err?.message || '保存 AI 结果失败')
        }
        return
      }

      if (hasChanges.value && fileContent.value !== content) {
        try {
          await ElMessageBox.confirm(
            '当前编辑器里有未保存的修改，应用 AI 结果会直接替换当前内容，是否继续？',
            '覆盖当前编辑器',
            {
              type: 'warning',
              confirmButtonText: '继续应用',
              cancelButtonText: '取消'
            }
          )
        } catch {
          return
        }
      }

      fileContent.value = content
      isEditing.value = true
      showAIDialog.value = false
      ElMessage.success(autoDebugAfterApply.value ? 'AI 结果已应用到当前编辑器，正在启动调试' : 'AI 结果已应用到当前编辑器，请确认后保存')
      if (autoDebugAfterApply.value) {
        await openDebugAndStart({ useEditorContent: true })
      }
      return
    }

    if (currentPath && hasChanges.value) {
      try {
        await ElMessageBox.confirm(
          `当前脚本 ${currentPath} 有未保存修改。继续后会切换到 ${desiredPath}，可能丢失当前未保存内容，是否继续？`,
          '切换脚本前确认',
          {
            type: 'warning',
            confirmButtonText: '继续',
            cancelButtonText: '取消'
          }
        )
      } catch {
        return
      }
    }

    const overwriteHint = desiredPath === currentPath ? '覆盖当前脚本' : `保存到 ${desiredPath}`
    try {
      await ElMessageBox.confirm(
        `确认将 AI 结果${overwriteHint}吗？`,
        '保存 AI 脚本',
        {
          type: 'info',
          confirmButtonText: '确认保存',
          cancelButtonText: '取消'
        }
      )
    } catch {
      return
    }

    try {
      await scriptApi.saveContent(desiredPath, content, buildAIVersionMessage(mode.value))
      await loadTree()
      selectedFile.value = desiredPath
      await loadFileContent(desiredPath)
      isEditing.value = false
      showAIDialog.value = false
      ElMessage.success(autoDebugAfterApply.value ? 'AI 结果已保存，正在启动调试' : 'AI 结果已保存到脚本目录')
      if (autoDebugAfterApply.value) {
        await openDebugAndStart()
      }
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '保存 AI 结果失败')
    }
  }

  function handleCancelGenerate() {
    closeGenerationStream()
    generating.value = false
    ElMessage.info('已取消生成')
  }

  return {
    showAIDialog,
    aiEnabled,
    configLoading,
    generating,
    providerOptions,
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
    openAIDialogFor,
    handleGenerate,
    handleCancelGenerate,
    handleApply
  }
}

function pickProvider(defaultProvider: string, options: AIProviderOption[]) {
  const configured = options.filter(item => item.configured)
  if (!configured.length) {
    return defaultProvider || ''
  }
  if (defaultProvider && configured.some(item => item.id === defaultProvider)) {
    return defaultProvider
  }
  return String(configured[0]?.id || '')
}

function defaultPrompt(mode: AICodeMode) {
  switch (mode) {
    case 'fix':
      return '请根据当前脚本内容和最近调试日志，修复脚本报错，并尽量保持原有逻辑不变。'
    case 'generate':
      return '请生成一个可直接运行的新脚本，并在必要时补齐注释与错误处理。'
    default:
      return '请按我的需求修改当前脚本，并只做必要改动。'
  }
}

function buildAIVersionMessage(mode: AICodeMode | string) {
  switch (mode) {
    case 'fix':
      return 'AI 修复脚本'
    case 'generate':
      return 'AI 生成脚本'
    default:
      return 'AI 修改脚本'
  }
}

function normalizeEditorLanguage(language: string) {
  const normalized = String(language || '').trim().toLowerCase()
  if (!normalized || normalized === 'plaintext') {
    return 'python'
  }
  if (normalized === 'shell') {
    return 'shell'
  }
  return normalized
}

function normalizeScriptPath(path: string) {
  return String(path || '').trim().replace(/\\/g, '/')
}

function resolveApplicableResultContent(
  responseMode: AICodeResponseMode | string,
  content: string,
  previewContent: string
) {
  if (responseMode === 'patch') {
    return previewContent || ''
  }
  return content || ''
}

function resolveConversationContent(
  responseMode: AICodeResponseMode | string,
  content: string,
  previewContent: string
) {
  if (responseMode === 'patch' || responseMode === 'full') {
    const applicableContent = resolveApplicableResultContent(responseMode, content, previewContent)
    if (applicableContent.trim()) {
      return applicableContent
    }
  }
  return content || ''
}

function resolveCurrentContentForRequest(
  shouldRestartConversation: boolean,
  selectedFile: string,
  isBinary: boolean,
  fileContent: string,
  conversationBaseContent: string
) {
  if (!shouldRestartConversation && stringsHasContent(conversationBaseContent)) {
    return conversationBaseContent
  }
  if (selectedFile && !isBinary) {
    return fileContent
  }
  return ''
}

function trimConversationHistory(turns: AICodeConversationTurn[]) {
  const normalizedTurns = turns
    .map((item) => ({
      ...item,
      prompt: String(item.prompt || '').trim(),
      summary: String(item.summary || '').trim(),
      content: String(item.content || '').trim()
    }))
    .filter(item => item.prompt)

  if (normalizedTurns.length <= 4) {
    return normalizedTurns
  }
  return normalizedTurns.slice(normalizedTurns.length - 4)
}

function stringsHasContent(value: string) {
  return String(value || '').trim().length > 0
}

function parseAIStreamEventData<T>(raw: string): T | null {
  try {
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

function applyStreamingPreview(
  rawOutput: string,
  responseMode: AICodeResponseMode | string,
  summaryRef: Ref<string>,
  contentRef: Ref<string>
) {
  const preview = extractPartialAICodeOutput(rawOutput)
  if (preview.hasSummary) {
    summaryRef.value = normalizeStreamingPreviewText(preview.summary)
  }
  if (preview.hasContent) {
    contentRef.value = normalizeStreamingPreviewText(preview.content)
    return
  }

  if ((responseMode === 'patch' || responseMode === 'explain') && !summaryRef.value.trim()) {
    contentRef.value = normalizeStreamingPreviewText(rawOutput)
  }
}

function extractPartialAICodeOutput(rawOutput: string) {
  const summary = extractPartialJSONStringField(rawOutput, 'summary')
  const content = extractPartialJSONStringField(rawOutput, 'content')
  return {
    hasSummary: summary.found,
    summary: summary.value,
    hasContent: content.found,
    content: content.value
  }
}

function extractPartialJSONStringField(rawOutput: string, fieldName: string) {
  const keyIndex = rawOutput.indexOf(`"${fieldName}"`)
  if (keyIndex === -1) {
    return { found: false, value: '' }
  }

  const colonIndex = rawOutput.indexOf(':', keyIndex + fieldName.length + 2)
  if (colonIndex === -1) {
    return { found: false, value: '' }
  }

  let index = colonIndex + 1
  while (index < rawOutput.length && /\s/.test(rawOutput.charAt(index))) {
    index++
  }

  if (rawOutput.charAt(index) !== '"') {
    return { found: false, value: '' }
  }

  index++
  let value = ''
  let escaping = false
  while (index < rawOutput.length) {
    const char = rawOutput.charAt(index)
    if (escaping) {
      const decoded = decodeJSONStringEscape(rawOutput, index)
      value += decoded.value
      index += decoded.advance
      escaping = false
      continue
    }
    if (char === '\\') {
      escaping = true
      index++
      continue
    }
    if (char === '"') {
      return { found: true, value }
    }
    value += char
    index++
  }

  return { found: true, value }
}

function decodeJSONStringEscape(rawOutput: string, index: number) {
  const char = rawOutput.charAt(index)
  switch (char) {
    case 'n':
      return { value: '\n', advance: 1 }
    case 'r':
      return { value: '\r', advance: 1 }
    case 't':
      return { value: '\t', advance: 1 }
    case '"':
      return { value: '"', advance: 1 }
    case '\\':
      return { value: '\\', advance: 1 }
    case '/':
      return { value: '/', advance: 1 }
    case 'b':
      return { value: '\b', advance: 1 }
    case 'f':
      return { value: '\f', advance: 1 }
    case 'u': {
      const hex = rawOutput.slice(index + 1, index + 5)
      if (/^[0-9a-fA-F]{4}$/.test(hex)) {
        return { value: String.fromCharCode(Number.parseInt(hex, 16)), advance: 5 }
      }
      return { value: '', advance: 1 }
    }
    default:
      return { value: char || '', advance: 1 }
  }
}

function normalizeStreamingPreviewText(value: string) {
  let current = value
  for (let i = 0; i < 2; i++) {
    const next = decodeLooseEscapes(current)
    if (next === current) {
      break
    }
    current = next
  }
  return current
}

function decodeLooseEscapes(value: string) {
  return value
    .replace(/\\u([0-9a-fA-F]{4})/g, (_, hex: string) => String.fromCharCode(Number.parseInt(hex, 16)))
    .replace(/\\r/g, '\r')
    .replace(/\\n/g, '\n')
    .replace(/\\t/g, '\t')
    .replace(/\\"/g, '"')
    .replace(/\\\\/g, '\\')
}
