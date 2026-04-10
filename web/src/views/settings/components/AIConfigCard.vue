<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Document, MagicStick, Select, Warning } from '@element-plus/icons-vue'
import type { AICodeAPIFormat, AICodeAuthStrategy, AIProviderId } from '@/api/ai'
import type { AIProviderTestState, SettingsConfigForm } from '../types'

const props = defineProps<{
  configsLoading: boolean
  configsSaving: boolean
  aiTestingProvider: string
  aiProviderTestStates: Record<string, AIProviderTestState>
  form: SettingsConfigForm
  onSave: () => void
  onTest: (provider: AIProviderId | string) => void
}>()

const openAIFormatOptions: Array<{ label: string; value: AICodeAPIFormat }> = [
  { label: 'OpenAI Chat Completions', value: 'openai_chat' },
  { label: 'OpenAI Responses', value: 'openai_responses' }
]

const anthropicFormatOptions: Array<{ label: string; value: AICodeAPIFormat }> = [
  { label: 'Anthropic Messages', value: 'anthropic' },
  { label: 'OpenAI Chat Completions', value: 'openai_chat' },
  { label: 'OpenAI Responses', value: 'openai_responses' }
]

const customFormatOptions: Array<{ label: string; value: AICodeAPIFormat }> = [
  { label: 'OpenAI Chat Completions', value: 'openai_chat' },
  { label: 'OpenAI Responses', value: 'openai_responses' },
  { label: 'Anthropic Messages', value: 'anthropic' },
  { label: 'Gemini GenerateContent', value: 'gemini' }
]

const anthropicAuthOptions: Array<{ label: string; value: AICodeAuthStrategy }> = [
  { label: 'x-api-key + anthropic-version', value: 'anthropic_key' },
  { label: 'Authorization Bearer', value: 'bearer' }
]

const customAuthOptions: Array<{ label: string; value: AICodeAuthStrategy }> = [
  { label: 'Authorization Bearer', value: 'bearer' },
  { label: 'x-api-key', value: 'x_api_key' },
  { label: 'x-api-key + anthropic-version', value: 'anthropic_key' },
  { label: 'x-goog-api-key', value: 'google_key' }
]

const openAIUrlPlaceholder = computed(() => {
  if (!props.form.ai_openai_is_full_url) {
    return 'https://api.openai.com/v1'
  }
  return props.form.ai_openai_api_format === 'openai_responses'
    ? 'https://api.openai.com/v1/responses'
    : 'https://api.openai.com/v1/chat/completions'
})

const openAIUrlHint = computed(() => {
  if (props.form.ai_openai_is_full_url) {
    return '开启后这里填写完整端点，系统会直接请求这个 URL，不再自动补路径。'
  }
  return props.form.ai_openai_api_format === 'openai_responses'
    ? '关闭完整端点时只填根地址，系统会自动补到 /responses。'
    : '关闭完整端点时只填根地址，系统会自动补到 /chat/completions。'
})

const anthropicUrlPlaceholder = computed(() => {
  if (!props.form.ai_anthropic_is_full_url) {
    return props.form.ai_anthropic_api_format === 'anthropic'
      ? 'https://api.anthropic.com'
      : 'https://relay.example.com/v1'
  }
  switch (props.form.ai_anthropic_api_format) {
    case 'openai_responses':
      return 'https://relay.example.com/v1/responses'
    case 'openai_chat':
      return 'https://relay.example.com/v1/chat/completions'
    default:
      return 'https://api.anthropic.com/v1/messages'
  }
})

const anthropicProtocolHint = computed(() => {
  if (props.form.ai_anthropic_is_full_url) {
    return '开启后将按你填写的完整端点直连，不再自动补 /v1/messages、/chat/completions 或 /responses。'
  }
  switch (props.form.ai_anthropic_api_format) {
    case 'openai_responses':
      return '适合把 Claude 模型挂在 Responses 协议下的中转，关闭完整端点时会自动补到 /responses。'
    case 'openai_chat':
      return '适合把 Claude 模型挂在 OpenAI Chat 协议下的中转，关闭完整端点时会自动补到 /chat/completions。'
    default:
      return '适合 Anthropic 官方或兼容 Messages API 的中转，关闭完整端点时会自动补到 /v1/messages。'
  }
})

const anthropicAuthHint = computed(() => {
  return props.form.ai_anthropic_auth_strategy === 'anthropic_key'
    ? 'Anthropic 官方通常使用 x-api-key + anthropic-version。'
    : 'Bearer 适合只接受 Authorization 头的 Claude relay / 中转接口。'
})

const geminiUrlPlaceholder = computed(() => {
  return props.form.ai_gemini_is_full_url
    ? 'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:generateContent'
    : 'https://generativelanguage.googleapis.com'
})

const geminiUrlHint = computed(() => {
  return props.form.ai_gemini_is_full_url
    ? '开启后这里填写完整 GenerateContent 端点，系统会直接请求这个 URL。'
    : '关闭完整端点时只填基础地址，系统会按模型名自动补到 /v1beta/models/{model}:generateContent。'
})

const customUrlPlaceholder = computed(() => {
  if (!props.form.ai_custom_is_full_url) {
    switch (props.form.ai_custom_api_format) {
      case 'anthropic':
        return 'https://your-api.example.com'
      case 'gemini':
        return 'https://generativelanguage.googleapis.com'
      default:
        return 'https://your-api.example.com/v1'
    }
  }
  switch (props.form.ai_custom_api_format) {
    case 'openai_responses':
      return 'https://your-api.example.com/v1/responses'
    case 'anthropic':
      return 'https://your-api.example.com/v1/messages'
    case 'gemini':
      return 'https://your-api.example.com/v1beta/models/your-model:generateContent'
    default:
      return 'https://your-api.example.com/v1/chat/completions'
  }
})

const customProtocolHint = computed(() => {
  if (props.form.ai_custom_is_full_url) {
    return '开启后系统会按你填写的完整端点直连，适合代理网关、带固定路径的第三方接口或 Azure 风格地址。'
  }
  switch (props.form.ai_custom_api_format) {
    case 'openai_responses':
      return 'Responses 协议适合 GPT-5 / 多数新式 OpenAI 兼容接口，关闭完整端点时自动补到 /responses。'
    case 'anthropic':
      return 'Anthropic 协议适合 Claude 兼容 Messages API 的第三方服务，关闭完整端点时自动补到 /v1/messages。'
    case 'gemini':
      return 'Gemini 协议会按模型名自动补到 GenerateContent 端点。'
    default:
      return 'Chat Completions 协议适合 One API、New API、硅基流动等常见兼容接口，关闭完整端点时自动补到 /chat/completions。'
  }
})

const customAuthHint = computed(() => {
  switch (props.form.ai_custom_auth_strategy) {
    case 'anthropic_key':
      return '适合需要 x-api-key + anthropic-version 的 Claude 兼容接口。'
    case 'google_key':
      return '适合 Gemini / Google 风格的 x-goog-api-key 鉴权。'
    case 'x_api_key':
      return '适合只接受 x-api-key 的第三方接口。'
    default:
      return 'Bearer 是大多数 OpenAI 兼容接口的默认鉴权方式。'
  }
})

function providerUrlLabel(isFullUrl: boolean) {
  return isFullUrl ? '完整接口 URL' : 'API Base URL'
}

function validateAndSave() {
  const f = props.form
  const warnings: string[] = []
  const providers = [
    { label: 'OpenAI', key: f.ai_openai_api_key, model: f.ai_openai_model, url: f.ai_openai_base_url },
    { label: 'Anthropic', key: f.ai_anthropic_api_key, model: f.ai_anthropic_model, url: f.ai_anthropic_base_url },
    { label: 'Gemini', key: f.ai_gemini_api_key, model: f.ai_gemini_model, url: f.ai_gemini_base_url },
    { label: '自定义', key: f.ai_custom_api_key, model: f.ai_custom_model, url: f.ai_custom_base_url }
  ]
  for (const p of providers) {
    if ((p.key || p.model) && !p.url) {
      warnings.push(`${p.label} 已填写密钥或模型但缺少 API 地址`)
    }
  }
  if (warnings.length) {
    ElMessage.warning(warnings.join('；'))
  }
  props.onSave()
}
</script>

<template>
  <el-card shadow="never" v-loading="props.configsLoading">
    <template #header>
      <div class="card-header">
        <span class="card-title"><el-icon><MagicStick /></el-icon> AI 脚本助手</span>
        <el-button type="primary" :loading="props.configsSaving" @click="validateAndSave">
          <el-icon><Document /></el-icon>保存配置
        </el-button>
      </div>
    </template>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="AI 生成、改写和修错时，会把当前脚本内容与调试日志发送到所选模型服务端，请确认所选接口符合你的使用和隐私要求。"
      class="privacy-alert"
    />

    <div class="config-section">
      <h4 class="section-title">基础设置</h4>
      <div class="switch-row">
        <div class="switch-item">
          <span class="switch-label">启用 AI 脚本助手</span>
          <el-switch v-model="props.form.ai_enabled" inline-prompt active-text="开" inactive-text="关" />
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>默认提供商</label>
          <el-select v-model="props.form.ai_default_provider" style="width: 100%">
            <el-option label="OpenAI / GPT" value="openai" />
            <el-option label="Claude / Anthropic" value="anthropic" />
            <el-option label="Gemini / Google" value="gemini" />
            <el-option label="第三方兼容接口" value="custom" />
          </el-select>
          <span class="form-hint">脚本页默认选中的模型通道，可在使用时临时切换</span>
        </div>
        <div class="form-field">
          <label>请求超时（秒）</label>
          <el-input v-model.number="props.form.ai_request_timeout_seconds" />
          <span class="form-hint">模型响应较慢时可适当调大，默认 120 秒</span>
        </div>
        <div class="form-field">
          <label>生成温度 (Temperature)</label>
          <el-input v-model="props.form.ai_temperature" placeholder="0.2" />
          <span class="form-hint">控制生成随机性，0 最确定，2 最随机，代码生成建议 0.1-0.3</span>
        </div>
      </div>
      <div class="fixed-guardrail-box">
        <div class="guardrail-title">
          <el-icon><Warning /></el-icon>
          <span>固定安全规则</span>
        </div>
        <div class="guardrail-list">
          <span>默认按面板托管运行时生成脚本，禁止交互式输入、菜单选择和必填命令行参数。</span>
          <span>不能生成绕过鉴权、窃取 Bearer Token / App Secret / Cookie / 环境变量全集的代码。</span>
          <span>不能生成反弹 shell、后门、内网探测、SSRF、爆破、提权、关闭验证码或关闭鉴权的脚本。</span>
          <span>涉及面板能力时必须优先使用官方接口或内置 helper，例如 `notify.py`、`sendNotify.js`、`/api/v1/open-api/token`。</span>
        </div>
      </div>
      <div class="form-field form-field--wide">
        <label>AI 脚本附加提示词</label>
        <el-input
          v-model="props.form.ai_code_custom_prompt"
          type="textarea"
          :rows="8"
          resize="vertical"
          placeholder="这里写你自己的长期规范，例如：默认使用中文日志、网络请求统一带 timeout、输出结构保持面板现有风格。"
        />
        <span class="form-hint">这里只追加你的偏好和规范，不能覆盖上面的固定安全规则。</span>
      </div>
    </div>

    <div class="config-section">
      <h4 class="section-title">OpenAI / GPT</h4>
      <div class="section-actions">
        <el-button size="small" :loading="props.aiTestingProvider === 'openai'" @click="props.onTest('openai')">
          <el-icon><Select /></el-icon>测试连接
        </el-button>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>{{ providerUrlLabel(props.form.ai_openai_is_full_url) }}</label>
          <el-input v-model="props.form.ai_openai_base_url" :placeholder="openAIUrlPlaceholder" />
          <span class="form-hint">{{ openAIUrlHint }}</span>
        </div>
        <div class="form-field">
          <label>默认模型</label>
          <el-input v-model="props.form.ai_openai_model" placeholder="如 gpt-4.1 / gpt-4o / 你自己的模型名" />
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>接口格式</label>
          <el-select v-model="props.form.ai_openai_api_format" style="width: 100%">
            <el-option v-for="item in openAIFormatOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </div>
        <div class="form-field">
          <label>完整端点直连</label>
          <el-switch v-model="props.form.ai_openai_is_full_url" inline-prompt active-text="开" inactive-text="关" />
          <span class="form-hint">填写到具体 `/chat/completions` 或 `/responses` 端点时请开启。</span>
        </div>
      </div>
      <div class="form-field form-field--wide">
        <label>API Key</label>
        <el-input v-model="props.form.ai_openai_api_key" type="password" show-password placeholder="sk-..." />
      </div>
      <div v-if="props.aiProviderTestStates.openai?.message" class="test-feedback" :class="`is-${props.aiProviderTestStates.openai.status}`">
        <el-icon><component :is="props.aiProviderTestStates.openai.status === 'success' ? Select : Warning" /></el-icon>
        <span>{{ props.aiProviderTestStates.openai.message }}</span>
      </div>
    </div>

    <div class="config-section">
      <h4 class="section-title">Claude / Anthropic</h4>
      <div class="section-actions">
        <el-button size="small" :loading="props.aiTestingProvider === 'anthropic'" @click="props.onTest('anthropic')">
          <el-icon><Select /></el-icon>测试连接
        </el-button>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>{{ providerUrlLabel(props.form.ai_anthropic_is_full_url) }}</label>
          <el-input v-model="props.form.ai_anthropic_base_url" :placeholder="anthropicUrlPlaceholder" />
          <span class="form-hint">{{ anthropicProtocolHint }}</span>
        </div>
        <div class="form-field">
          <label>默认模型</label>
          <el-input v-model="props.form.ai_anthropic_model" placeholder="如 claude-sonnet / claude-opus" />
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>接口格式</label>
          <el-select v-model="props.form.ai_anthropic_api_format" style="width: 100%">
            <el-option v-for="item in anthropicFormatOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </div>
        <div class="form-field">
          <label>鉴权方式</label>
          <el-select v-model="props.form.ai_anthropic_auth_strategy" style="width: 100%">
            <el-option v-for="item in anthropicAuthOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
          <span class="form-hint">{{ anthropicAuthHint }}</span>
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>完整端点直连</label>
          <el-switch v-model="props.form.ai_anthropic_is_full_url" inline-prompt active-text="开" inactive-text="关" />
          <span class="form-hint">对 Claude relay、反代网关或固定路径端点特别有用。</span>
        </div>
      </div>
      <div class="form-field form-field--wide">
        <label>API Key</label>
        <el-input v-model="props.form.ai_anthropic_api_key" type="password" show-password placeholder="sk-ant-..." />
      </div>
      <div v-if="props.aiProviderTestStates.anthropic?.message" class="test-feedback" :class="`is-${props.aiProviderTestStates.anthropic.status}`">
        <el-icon><component :is="props.aiProviderTestStates.anthropic.status === 'success' ? Select : Warning" /></el-icon>
        <span>{{ props.aiProviderTestStates.anthropic.message }}</span>
      </div>
    </div>

    <div class="config-section">
      <h4 class="section-title">Gemini / Google</h4>
      <div class="section-actions">
        <el-button size="small" :loading="props.aiTestingProvider === 'gemini'" @click="props.onTest('gemini')">
          <el-icon><Select /></el-icon>测试连接
        </el-button>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>{{ providerUrlLabel(props.form.ai_gemini_is_full_url) }}</label>
          <el-input v-model="props.form.ai_gemini_base_url" :placeholder="geminiUrlPlaceholder" />
          <span class="form-hint">{{ geminiUrlHint }}</span>
        </div>
        <div class="form-field">
          <label>默认模型</label>
          <el-input v-model="props.form.ai_gemini_model" placeholder="如 gemini-2.5-pro / gemini-2.0-flash" />
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>完整端点直连</label>
          <el-switch v-model="props.form.ai_gemini_is_full_url" inline-prompt active-text="开" inactive-text="关" />
          <span class="form-hint">需要直接填写完整 GenerateContent 端点时再开启。</span>
        </div>
      </div>
      <div class="form-field form-field--wide">
        <label>API Key</label>
        <el-input v-model="props.form.ai_gemini_api_key" type="password" show-password placeholder="AIza..." />
      </div>
      <div v-if="props.aiProviderTestStates.gemini?.message" class="test-feedback" :class="`is-${props.aiProviderTestStates.gemini.status}`">
        <el-icon><component :is="props.aiProviderTestStates.gemini.status === 'success' ? Select : Warning" /></el-icon>
        <span>{{ props.aiProviderTestStates.gemini.message }}</span>
      </div>
    </div>

    <div class="config-section">
      <h4 class="section-title">第三方兼容接口</h4>
      <div class="section-actions">
        <el-button size="small" :loading="props.aiTestingProvider === 'custom'" @click="props.onTest('custom')">
          <el-icon><Select /></el-icon>测试连接
        </el-button>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>{{ providerUrlLabel(props.form.ai_custom_is_full_url) }}</label>
          <el-input v-model="props.form.ai_custom_base_url" :placeholder="customUrlPlaceholder" />
          <span class="form-hint">{{ customProtocolHint }}</span>
        </div>
        <div class="form-field">
          <label>默认模型</label>
          <el-input v-model="props.form.ai_custom_model" placeholder="填写该接口支持的模型名" />
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>接口格式</label>
          <el-select v-model="props.form.ai_custom_api_format" style="width: 100%">
            <el-option v-for="item in customFormatOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </div>
        <div class="form-field">
          <label>鉴权方式</label>
          <el-select v-model="props.form.ai_custom_auth_strategy" style="width: 100%">
            <el-option v-for="item in customAuthOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
          <span class="form-hint">{{ customAuthHint }}</span>
        </div>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label>完整端点直连</label>
          <el-switch v-model="props.form.ai_custom_is_full_url" inline-prompt active-text="开" inactive-text="关" />
          <span class="form-hint">适合固定路径网关、反代地址和带查询参数的完整端点。</span>
        </div>
      </div>
      <div class="form-field form-field--wide">
        <label>API Key</label>
        <el-input v-model="props.form.ai_custom_api_key" type="password" show-password placeholder="第三方接口密钥" />
      </div>
      <div class="compat-note">
        <el-icon><Connection /></el-icon>
        <span>这里已经不再只限 OpenAI Chat。你可以按协议类型、鉴权方式和完整 URL 模式去适配 Claude relay、Gemini 网关或其他第三方 API。</span>
      </div>
      <div v-if="props.aiProviderTestStates.custom?.message" class="test-feedback" :class="`is-${props.aiProviderTestStates.custom.status}`">
        <el-icon><component :is="props.aiProviderTestStates.custom.status === 'success' ? Select : Warning" /></el-icon>
        <span>{{ props.aiProviderTestStates.custom.message }}</span>
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
@use './config-card-shared.scss' as *;

.privacy-alert {
  margin-bottom: 20px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 20px;
}

.form-field--wide {
  max-width: 100%;
}

.fixed-guardrail-box {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 18px;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--el-border-color-light);
  background: var(--el-fill-color-extra-light);
}

.guardrail-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.guardrail-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--el-text-color-secondary);
}

.section-actions {
  display: flex;
  justify-content: flex-end;
  margin: -8px 0 16px;
}

.compat-note {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.test-feedback {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;

  &.is-success {
    color: var(--el-color-success-dark-2);
    background: var(--el-color-success-light-9);
  }

  &.is-error {
    color: var(--el-color-danger-dark-2);
    background: var(--el-color-danger-light-9);
  }
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
