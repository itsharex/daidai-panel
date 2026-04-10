import type { AICodeAPIFormat, AICodeAuthStrategy, AIProviderId } from '@/api/ai'

export type CaptchaFailMode = 'open' | 'strict'

export interface AIProviderTestState {
  status: 'idle' | 'success' | 'error'
  message: string
}

export interface SettingsConfigForm {
  max_concurrent_tasks: number
  command_timeout: number
  log_retention_days: number
  max_log_content_size: number
  random_delay: string
  random_delay_extensions: string
  auto_install_deps: boolean
  auto_add_cron: boolean
  auto_del_cron: boolean
  default_cron_rule: string
  repo_file_extensions: string
  cpu_warn: number
  memory_warn: number
  disk_warn: number
  notify_on_resource_warn: boolean
  notify_on_login: boolean
  proxy_url: string
  update_image_mirror: string
  auto_update_enabled: boolean
  trusted_proxy_cidrs: string
  captcha_enabled: boolean
  captcha_id: string
  captcha_key: string
  captcha_fail_mode: CaptchaFailMode | string
  panel_title: string
  panel_icon: string
  editor_background_color: string
  log_background_color: string
  log_background_image: string
  ai_enabled: boolean
  ai_code_custom_prompt: string
  ai_default_provider: AIProviderId | string
  ai_request_timeout_seconds: number
  ai_temperature: string
  ai_openai_base_url: string
  ai_openai_api_key: string
  ai_openai_model: string
  ai_openai_api_format: AICodeAPIFormat | string
  ai_openai_is_full_url: boolean
  ai_anthropic_base_url: string
  ai_anthropic_api_key: string
  ai_anthropic_model: string
  ai_anthropic_api_format: AICodeAPIFormat | string
  ai_anthropic_auth_strategy: AICodeAuthStrategy | string
  ai_anthropic_is_full_url: boolean
  ai_gemini_base_url: string
  ai_gemini_api_key: string
  ai_gemini_model: string
  ai_gemini_is_full_url: boolean
  ai_custom_base_url: string
  ai_custom_api_key: string
  ai_custom_model: string
  ai_custom_api_format: AICodeAPIFormat | string
  ai_custom_auth_strategy: AICodeAuthStrategy | string
  ai_custom_is_full_url: boolean
}
