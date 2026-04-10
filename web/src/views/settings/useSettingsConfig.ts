import { ref } from 'vue'
import { configApi } from '@/api/system'
import { aiApi, type AIProviderId } from '@/api/ai'
import { ElMessage } from 'element-plus'
import { applyPanelAppearance } from '@/utils/panelAppearance'
import type { AIProviderTestState, SettingsConfigForm } from './types'

const defaultAICodeCustomPromptTemplate = [
  '- 默认使用简体中文编写必要的日志、注释和错误提示，输出内容要方便在面板日志里直接排查。',
  '- 默认生成可直接运行的完整脚本；如果是修改现有脚本，优先最小必要改动，保留原有变量名、入口形式和已有依赖风格。',
  '- 所有网络请求都应显式设置合理的 timeout；需要重试时只做有限次重试，并输出清晰日志。',
  '- 不要吞掉异常；失败时要输出明确错误原因，必要时返回非 0 退出码或抛出异常，方便面板识别失败。',
  '- 如果脚本需要通知、环境变量管理、任务或脚本相关能力，优先沿用面板官方 helper、官方接口和当前仓库已有调用方式。',
  '- 除非用户明确要求，否则不要引入体积大、安装重或与当前脚本无关的新依赖。'
].join('\n')

export function useSettingsConfig() {
  const captchaFeatureImplemented = true
  const configsLoading = ref(false)
  const configsSaving = ref(false)
  const aiTestingProvider = ref('')
  const aiProviderTestStates = ref<Record<string, AIProviderTestState>>({})

  const configForm = ref<SettingsConfigForm>({
    max_concurrent_tasks: 5,
    command_timeout: 86400,
    log_retention_days: 7,
    max_log_content_size: 102400,
    random_delay: '',
    random_delay_extensions: '',
    auto_install_deps: true,
    auto_add_cron: true,
    auto_del_cron: true,
    default_cron_rule: '',
    repo_file_extensions: '',
    cpu_warn: 80,
    memory_warn: 80,
    disk_warn: 90,
    notify_on_resource_warn: false,
    notify_on_login: false,
    proxy_url: '',
    update_image_mirror: '',
    auto_update_enabled: false,
    trusted_proxy_cidrs: '',
    captcha_enabled: false,
    captcha_id: '',
    captcha_key: '',
    captcha_fail_mode: 'open',
    panel_title: '',
    panel_icon: '',
    editor_background_color: '',
    log_background_color: '#0f172a',
    log_background_image: '',
    ai_enabled: false,
    ai_code_custom_prompt: defaultAICodeCustomPromptTemplate,
    ai_default_provider: 'openai',
    ai_request_timeout_seconds: 120,
    ai_temperature: '0.2',
    ai_openai_base_url: 'https://api.openai.com/v1',
    ai_openai_api_key: '',
    ai_openai_model: '',
    ai_openai_api_format: 'openai_chat',
    ai_openai_is_full_url: false,
    ai_anthropic_base_url: 'https://api.anthropic.com',
    ai_anthropic_api_key: '',
    ai_anthropic_model: '',
    ai_anthropic_api_format: 'anthropic',
    ai_anthropic_auth_strategy: 'anthropic_key',
    ai_anthropic_is_full_url: false,
    ai_gemini_base_url: 'https://generativelanguage.googleapis.com',
    ai_gemini_api_key: '',
    ai_gemini_model: '',
    ai_gemini_is_full_url: false,
    ai_custom_base_url: '',
    ai_custom_api_key: '',
    ai_custom_model: '',
    ai_custom_api_format: 'openai_chat',
    ai_custom_auth_strategy: 'bearer',
    ai_custom_is_full_url: false
  })

  function readConfigString(cfgs: Record<string, any>, key: string, fallback = ''): string {
    const entry = cfgs[key]
    const raw = entry?.value ?? entry?.default_value ?? fallback
    if (raw === null || raw === undefined) return fallback
    return String(raw)
  }

  function readConfigNumber(cfgs: Record<string, any>, key: string, fallback: number): number {
    const raw = readConfigString(cfgs, key, String(fallback))
    const parsed = Number(raw)
    return Number.isFinite(parsed) ? parsed : fallback
  }

  function readConfigBool(cfgs: Record<string, any>, key: string, fallback: boolean): boolean {
    const raw = readConfigString(cfgs, key, fallback ? 'true' : 'false').trim().toLowerCase()
    if (['true', '1', 'yes', 'on'].includes(raw)) return true
    if (['false', '0', 'no', 'off'].includes(raw)) return false
    return fallback
  }

  async function loadSystemConfigs() {
    configsLoading.value = true
    try {
      const res = await configApi.list()
      const cfgs = res.data || {}

      configForm.value = {
        max_concurrent_tasks: readConfigNumber(cfgs, 'max_concurrent_tasks', 5),
        command_timeout: readConfigNumber(cfgs, 'command_timeout', 86400),
        log_retention_days: readConfigNumber(cfgs, 'log_retention_days', 7),
        max_log_content_size: readConfigNumber(cfgs, 'max_log_content_size', 102400000),
        random_delay: readConfigString(cfgs, 'random_delay', ''),
        random_delay_extensions: readConfigString(cfgs, 'random_delay_extensions', ''),
        auto_install_deps: readConfigBool(cfgs, 'auto_install_deps', true),
        auto_add_cron: readConfigBool(cfgs, 'auto_add_cron', true),
        auto_del_cron: readConfigBool(cfgs, 'auto_del_cron', true),
        default_cron_rule: readConfigString(cfgs, 'default_cron_rule', ''),
        repo_file_extensions: readConfigString(cfgs, 'repo_file_extensions', ''),
        cpu_warn: readConfigNumber(cfgs, 'cpu_warn', 80),
        memory_warn: readConfigNumber(cfgs, 'memory_warn', 80),
        disk_warn: readConfigNumber(cfgs, 'disk_warn', 90),
        notify_on_resource_warn: readConfigBool(cfgs, 'notify_on_resource_warn', false),
        notify_on_login: readConfigBool(cfgs, 'notify_on_login', false),
        proxy_url: readConfigString(cfgs, 'proxy_url', ''),
        update_image_mirror: readConfigString(cfgs, 'update_image_mirror', ''),
        auto_update_enabled: readConfigBool(cfgs, 'auto_update_enabled', false),
        trusted_proxy_cidrs: readConfigString(cfgs, 'trusted_proxy_cidrs', ''),
        captcha_enabled: readConfigBool(cfgs, 'captcha_enabled', false),
        captcha_id: readConfigString(cfgs, 'captcha_id', ''),
        captcha_key: readConfigString(cfgs, 'captcha_key', ''),
        captcha_fail_mode: readConfigString(cfgs, 'captcha_fail_mode', 'open'),
        panel_title: readConfigString(cfgs, 'panel_title', ''),
        panel_icon: readConfigString(cfgs, 'panel_icon', ''),
        editor_background_color: readConfigString(cfgs, 'editor_background_color', ''),
        log_background_color: readConfigString(cfgs, 'log_background_color', '#0f172a'),
        log_background_image: readConfigString(cfgs, 'log_background_image', ''),
        ai_enabled: readConfigBool(cfgs, 'ai_enabled', false),
        ai_code_custom_prompt: readConfigString(cfgs, 'ai_code_custom_prompt', defaultAICodeCustomPromptTemplate),
        ai_default_provider: readConfigString(cfgs, 'ai_default_provider', 'openai'),
        ai_request_timeout_seconds: readConfigNumber(cfgs, 'ai_request_timeout_seconds', 120),
        ai_temperature: readConfigString(cfgs, 'ai_temperature', '0.2'),
        ai_openai_base_url: readConfigString(cfgs, 'ai_openai_base_url', 'https://api.openai.com/v1'),
        ai_openai_api_key: readConfigString(cfgs, 'ai_openai_api_key', ''),
        ai_openai_model: readConfigString(cfgs, 'ai_openai_model', ''),
        ai_openai_api_format: readConfigString(cfgs, 'ai_openai_api_format', 'openai_chat'),
        ai_openai_is_full_url: readConfigBool(cfgs, 'ai_openai_is_full_url', false),
        ai_anthropic_base_url: readConfigString(cfgs, 'ai_anthropic_base_url', 'https://api.anthropic.com'),
        ai_anthropic_api_key: readConfigString(cfgs, 'ai_anthropic_api_key', ''),
        ai_anthropic_model: readConfigString(cfgs, 'ai_anthropic_model', ''),
        ai_anthropic_api_format: readConfigString(cfgs, 'ai_anthropic_api_format', 'anthropic'),
        ai_anthropic_auth_strategy: readConfigString(cfgs, 'ai_anthropic_auth_strategy', 'anthropic_key'),
        ai_anthropic_is_full_url: readConfigBool(cfgs, 'ai_anthropic_is_full_url', false),
        ai_gemini_base_url: readConfigString(cfgs, 'ai_gemini_base_url', 'https://generativelanguage.googleapis.com'),
        ai_gemini_api_key: readConfigString(cfgs, 'ai_gemini_api_key', ''),
        ai_gemini_model: readConfigString(cfgs, 'ai_gemini_model', ''),
        ai_gemini_is_full_url: readConfigBool(cfgs, 'ai_gemini_is_full_url', false),
        ai_custom_base_url: readConfigString(cfgs, 'ai_custom_base_url', ''),
        ai_custom_api_key: readConfigString(cfgs, 'ai_custom_api_key', ''),
        ai_custom_model: readConfigString(cfgs, 'ai_custom_model', ''),
        ai_custom_api_format: readConfigString(cfgs, 'ai_custom_api_format', 'openai_chat'),
        ai_custom_auth_strategy: readConfigString(cfgs, 'ai_custom_auth_strategy', 'bearer'),
        ai_custom_is_full_url: readConfigBool(cfgs, 'ai_custom_is_full_url', false)
      }
      applyPanelAppearance(configForm.value)
    } catch {
      ElMessage.error('加载配置失败')
    } finally {
      configsLoading.value = false
    }
  }

  async function saveConfigKeys(keys: string[]) {
    configsSaving.value = true
    try {
      const configs: Record<string, string> = {}
      for (const key of keys) {
        const val = (configForm.value as any)[key]
        configs[key] = typeof val === 'boolean' ? (val ? 'true' : 'false') : String(val ?? '')
      }
      await configApi.batchSet(configs)
      applyPanelAppearance(configForm.value)
      ElMessage.success('配置已保存')
    } catch {
      ElMessage.error('保存失败')
    } finally {
      configsSaving.value = false
    }
  }

  function handleSaveSystemConfig() {
    void saveConfigKeys([
      'auto_add_cron', 'auto_del_cron', 'default_cron_rule', 'repo_file_extensions',
      'cpu_warn', 'memory_warn', 'disk_warn', 'notify_on_resource_warn', 'notify_on_login',
      'panel_title', 'panel_icon', 'editor_background_color', 'log_background_color', 'log_background_image'
    ])
  }

  function handleIconUpload(file: File) {
    if (!file.name.endsWith('.svg')) {
      ElMessage.warning('仅支持 SVG 格式图标')
      return false
    }
    if (file.size > 100 * 1024) {
      ElMessage.warning('图标文件不能超过 100KB')
      return false
    }
    const reader = new FileReader()
    reader.onload = (e) => {
      configForm.value.panel_icon = e.target?.result as string
    }
    reader.readAsDataURL(file)
    return false
  }

  function handleLogBackgroundUpload(file: File) {
    if (!file.type.startsWith('image/')) {
      ElMessage.warning('仅支持图片格式背景')
      return false
    }
    if (file.size > 2 * 1024 * 1024) {
      ElMessage.warning('背景图片不能超过 2MB')
      return false
    }

    const reader = new FileReader()
    reader.onload = (e) => {
      configForm.value.log_background_image = e.target?.result as string
      applyPanelAppearance(configForm.value)
    }
    reader.readAsDataURL(file)
    return false
  }

  function previewPanelAppearance() {
    applyPanelAppearance(configForm.value)
  }

  function handleSaveTaskConfig() {
    void saveConfigKeys([
      'max_concurrent_tasks', 'command_timeout', 'log_retention_days',
      'max_log_content_size', 'random_delay', 'random_delay_extensions', 'auto_install_deps'
    ])
  }

  function handleSaveProxy() {
    void saveConfigKeys(['proxy_url', 'update_image_mirror', 'auto_update_enabled', 'trusted_proxy_cidrs'])
  }

  function handleSaveCaptcha() {
    void saveConfigKeys(['captcha_enabled', 'captcha_id', 'captcha_key', 'captcha_fail_mode'])
  }

  function setAIProviderTestState(provider: string, state: AIProviderTestState) {
    aiProviderTestStates.value = {
      ...aiProviderTestStates.value,
      [provider]: state
    }
  }

  function providerTestPayload(provider: AIProviderId | string) {
    const timeout_seconds = Number(configForm.value.ai_request_timeout_seconds || 120)
    switch (provider) {
      case 'anthropic':
        return {
          provider,
          base_url: configForm.value.ai_anthropic_base_url,
          api_key: configForm.value.ai_anthropic_api_key,
          model: configForm.value.ai_anthropic_model,
          api_format: configForm.value.ai_anthropic_api_format,
          auth_strategy: configForm.value.ai_anthropic_auth_strategy,
          is_full_url: configForm.value.ai_anthropic_is_full_url,
          timeout_seconds
        }
      case 'gemini':
        return {
          provider,
          base_url: configForm.value.ai_gemini_base_url,
          api_key: configForm.value.ai_gemini_api_key,
          model: configForm.value.ai_gemini_model,
          api_format: 'gemini',
          auth_strategy: 'google_key',
          is_full_url: configForm.value.ai_gemini_is_full_url,
          timeout_seconds
        }
      case 'custom':
        return {
          provider,
          base_url: configForm.value.ai_custom_base_url,
          api_key: configForm.value.ai_custom_api_key,
          model: configForm.value.ai_custom_model,
          api_format: configForm.value.ai_custom_api_format,
          auth_strategy: configForm.value.ai_custom_auth_strategy,
          is_full_url: configForm.value.ai_custom_is_full_url,
          timeout_seconds
        }
      default:
        return {
          provider: 'openai',
          base_url: configForm.value.ai_openai_base_url,
          api_key: configForm.value.ai_openai_api_key,
          model: configForm.value.ai_openai_model,
          api_format: configForm.value.ai_openai_api_format,
          auth_strategy: 'bearer',
          is_full_url: configForm.value.ai_openai_is_full_url,
          timeout_seconds
        }
    }
  }

  async function handleTestAIProvider(provider: AIProviderId | string) {
    const payload = providerTestPayload(provider)
    if (!String(payload.base_url || '').trim()) {
      ElMessage.warning('请先填写 API 地址')
      setAIProviderTestState(provider, { status: 'error', message: '缺少 API 地址' })
      return
    }
    if (!String(payload.api_key || '').trim()) {
      ElMessage.warning('请先填写 API Key')
      setAIProviderTestState(provider, { status: 'error', message: '缺少 API Key' })
      return
    }
    if (!String(payload.model || '').trim()) {
      ElMessage.warning('请先填写模型名称')
      setAIProviderTestState(provider, { status: 'error', message: '缺少模型名称' })
      return
    }

    aiTestingProvider.value = String(provider)
    setAIProviderTestState(provider, { status: 'idle', message: '' })
    try {
      const res = await aiApi.testConnection(payload)
      const message = res.data?.message || '连接测试成功'
      setAIProviderTestState(provider, { status: 'success', message })
      ElMessage.success(message)
    } catch (err: any) {
      const message = err?.response?.data?.error || err?.message || '连接测试失败'
      setAIProviderTestState(provider, { status: 'error', message })
      ElMessage.error(message)
    } finally {
      aiTestingProvider.value = ''
    }
  }

  function handleSaveAIConfig() {
    void saveConfigKeys([
      'ai_enabled',
      'ai_code_custom_prompt',
      'ai_default_provider',
      'ai_request_timeout_seconds',
      'ai_temperature',
      'ai_openai_base_url',
      'ai_openai_api_key',
      'ai_openai_model',
      'ai_openai_api_format',
      'ai_openai_is_full_url',
      'ai_anthropic_base_url',
      'ai_anthropic_api_key',
      'ai_anthropic_model',
      'ai_anthropic_api_format',
      'ai_anthropic_auth_strategy',
      'ai_anthropic_is_full_url',
      'ai_gemini_base_url',
      'ai_gemini_api_key',
      'ai_gemini_model',
      'ai_gemini_is_full_url',
      'ai_custom_base_url',
      'ai_custom_api_key',
      'ai_custom_model',
      'ai_custom_api_format',
      'ai_custom_auth_strategy',
      'ai_custom_is_full_url'
    ])
  }

  return {
    captchaFeatureImplemented,
    configsLoading,
    configsSaving,
    aiTestingProvider,
    aiProviderTestStates,
    configForm,
    loadSystemConfigs,
    handleSaveSystemConfig,
    handleIconUpload,
    handleLogBackgroundUpload,
    previewPanelAppearance,
    handleSaveTaskConfig,
    handleSaveProxy,
    handleSaveCaptcha,
    handleTestAIProvider,
    handleSaveAIConfig
  }
}
