import request from './request'

export type AIProviderId = 'openai' | 'anthropic' | 'gemini' | 'custom'
export type AICodeMode = 'generate' | 'modify' | 'fix'
export type AICodeResponseMode = 'full' | 'patch' | 'explain'
export type AICodeAPIFormat = 'anthropic' | 'openai_chat' | 'openai_responses' | 'gemini'
export type AICodeAuthStrategy = 'bearer' | 'anthropic_key' | 'google_key' | 'x_api_key'

export interface AIProviderOption {
  id: AIProviderId | string
  label: string
  configured: boolean
  model: string
}

export interface AICodeConfigStatus {
  enabled: boolean
  default_provider: AIProviderId | string
  providers: AIProviderOption[]
}

export interface AICodeGenerateRequest {
  provider?: string
  model?: string
  mode: AICodeMode | string
  response_mode?: AICodeResponseMode | string
  prompt: string
  language?: string
  target_path?: string
  current_content?: string
  debug_logs?: string[]
  debug_exit_code?: number | null
  debug_error?: string
  conversation_history?: AICodeConversationTurn[]
}

export interface AICodeGenerateResponse {
  provider: string
  provider_label: string
  model: string
  response_mode: AICodeResponseMode | string
  can_apply: boolean
  summary: string
  content: string
  preview_content?: string
  warnings?: string[]
}

export interface AICodeConversationTurn {
  mode?: AICodeMode | string
  response_mode?: AICodeResponseMode | string
  prompt: string
  summary?: string
  content?: string
}

export interface AIProviderTestRequest {
  provider: AIProviderId | string
  base_url?: string
  api_key?: string
  model?: string
  api_format?: AICodeAPIFormat | string
  auth_strategy?: AICodeAuthStrategy | string
  is_full_url?: boolean
  timeout_seconds?: number
}

export interface AIProviderTestResponse {
  provider: string
  provider_label: string
  model: string
  message: string
}

export const aiApi = {
  config: () => request.get('/ai-code/config') as Promise<{ data: AICodeConfigStatus }>,
  testConnection: (data: AIProviderTestRequest) =>
    request.post('/ai-code/test', data, { timeout: 0 }) as Promise<{ data: AIProviderTestResponse }>
}
