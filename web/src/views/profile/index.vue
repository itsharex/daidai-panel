<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  CircleCheck,
  Key,
  Lock,
  RefreshRight,
  User,
  Clock,
  Avatar,
  Star,
  InfoFilled,
  Camera,
  Delete
} from '@element-plus/icons-vue'
import { authApi } from '@/api/auth'
import { securityApi } from '@/api/security'
import { sponsorApi, type SponsorRecord, type SponsorSummary } from '@/api/sponsor'
import { useAuthStore } from '@/stores/auth'
import { createQrCodeDataUrl } from '@/utils/qrcode'
import { useResponsive } from '@/composables/useResponsive'
import SponsorWall from './components/SponsorWall.vue'

const authStore = useAuthStore()
const { dialogFullscreen } = useResponsive()
const activeTab = ref<'security' | 'sponsors'>('security')

const passwordForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})
const passwordSaving = ref(false)

const twoFAEnabled = ref(false)
const twoFASecret = ref('')
const twoFAUri = ref('')
const twoFAQrUrl = ref('')
const twoFACode = ref('')
const showSetup2FA = ref(false)
const twoFALoading = ref(false)
const twoFADisabling = ref(false)

const sponsors = ref<SponsorRecord[]>([])
const sponsorSummary = ref<SponsorSummary | null>(null)
const sponsorLoading = ref(false)
const sponsorRefreshing = ref(false)
const sponsorFetchedOnce = ref(false)
let sponsorRefreshTimer: ReturnType<typeof setInterval> | null = null

const sponsorRefreshInterval = 24 * 60 * 60 * 1000

const roleLabel = computed(() => {
  const role = authStore.user?.role
  if (role === 'admin') return '管理员'
  if (role === 'operator') return '运维用户'
  return '只读用户'
})

const roleTagType = computed(() => {
  const role = authStore.user?.role
  if (role === 'admin') return 'danger'
  if (role === 'operator') return 'warning'
  return 'info'
})

function formatTime(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function buildEmptySponsorSummary(unavailable = false): SponsorSummary {
  return {
    sponsors: [],
    count: 0,
    total_amount: 0,
    updated_at: null,
    unavailable,
  }
}

function applySponsorSummary(summary: SponsorSummary) {
  const normalizedSponsors = Array.isArray(summary.sponsors) ? summary.sponsors : []
  sponsors.value = normalizedSponsors
  sponsorSummary.value = {
    ...buildEmptySponsorSummary(),
    ...summary,
    sponsors: normalizedSponsors,
    count: summary.count || normalizedSponsors.length,
  }
}

async function loadSponsors(options: { silent?: boolean } = {}) {
  const useSilentRefresh = options.silent === true && sponsorFetchedOnce.value
  if (useSilentRefresh) {
    sponsorRefreshing.value = true
  } else {
    sponsorLoading.value = true
  }

  try {
    const res = await sponsorApi.list() as any
    applySponsorSummary({
      ...buildEmptySponsorSummary(),
      ...(res.data || {}),
      sponsors: res.data?.sponsors || [],
    })
  } catch {
    if (!sponsorFetchedOnce.value) {
      applySponsorSummary(buildEmptySponsorSummary(true))
    } else {
      sponsorSummary.value = {
        ...(sponsorSummary.value || buildEmptySponsorSummary()),
        unavailable: true,
      }
    }
  } finally {
    sponsorFetchedOnce.value = true
    sponsorLoading.value = false
    sponsorRefreshing.value = false
  }
}

function stopSponsorRefreshTimer() {
  if (sponsorRefreshTimer) {
    clearInterval(sponsorRefreshTimer)
    sponsorRefreshTimer = null
  }
}

function startSponsorRefreshTimer() {
  stopSponsorRefreshTimer()
  sponsorRefreshTimer = setInterval(() => {
    void loadSponsors({ silent: true })
  }, sponsorRefreshInterval)
}

async function activateSponsorTab() {
  if (!sponsorFetchedOnce.value) {
    await loadSponsors()
  }
  startSponsorRefreshTimer()
}

async function handleManualSponsorRefresh() {
  await loadSponsors({ silent: sponsorFetchedOnce.value })
}

const sponsorTabHint = computed(() => {
  if (sponsorLoading.value && !sponsorFetchedOnce.value) {
    return '正在同步赞助名单'
  }
  if (sponsorRefreshing.value) {
    return '后台同步中'
  }
  if (sponsorSummary.value?.unavailable && sponsors.value.length > 0) {
    return '当前展示最近一次同步结果'
  }
  if (sponsorSummary.value?.unavailable) {
    return '服务暂不可用，页面会自动重试'
  }
  if (sponsorSummary.value?.updated_at) {
    return `更新于 ${formatTime(sponsorSummary.value.updated_at)}`
  }
  return '本页每 24 小时静默同步一次'
})

const usernameInitial = computed(() => {
  const name = authStore.user?.username || ''
  if (!name) return '用'
  return (name[0] ?? '用').toUpperCase()
})

const avatarUrl = computed(() => authStore.user?.avatar_url || '')
const avatarUploading = ref(false)
const avatarInputRef = ref<HTMLInputElement | null>(null)

function triggerAvatarUpload() {
  avatarInputRef.value?.click()
}

async function handleAvatarFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  input.value = ''

  const maxSize = 2 * 1024 * 1024
  if (file.size > maxSize) {
    ElMessage.warning('头像文件不能超过 2MB')
    return
  }

  const allowed = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']
  if (!allowed.includes(file.type)) {
    ElMessage.warning('仅支持 JPG、PNG、GIF、WebP 格式')
    return
  }

  avatarUploading.value = true
  try {
    const res = await authApi.uploadAvatar(file)
    ElMessage.success('头像上传成功')
    await authStore.fetchUser()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '头像上传失败')
  } finally {
    avatarUploading.value = false
  }
}

async function handleDeleteAvatar() {
  try {
    await ElMessageBox.confirm('确定要删除当前头像吗？', '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await authApi.deleteAvatar()
    ElMessage.success('头像已删除')
    await authStore.fetchUser()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '删除头像失败')
  }
}

async function load2FAStatus() {
  try {
    const res = await securityApi.get2FAStatus()
    twoFAEnabled.value = res.data.enabled
  } catch {
    ElMessage.error('加载 2FA 状态失败')
  }
}

async function handleChangePassword() {
  if (!passwordForm.value.oldPassword || !passwordForm.value.newPassword) {
    ElMessage.warning('请完整填写密码信息')
    return
  }
  if (passwordForm.value.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }

  passwordSaving.value = true
  try {
    await authApi.changePassword(passwordForm.value.oldPassword, passwordForm.value.newPassword)
    ElMessage.success('密码修改成功，即将重新登录')
    passwordForm.value = {
      oldPassword: '',
      newPassword: '',
      confirmPassword: '',
    }
    const LOGOUT_DELAY_MS = 1200
    setTimeout(() => {
      authStore.logout()
    }, LOGOUT_DELAY_MS)
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '密码修改失败')
  } finally {
    passwordSaving.value = false
  }
}

async function handleSetup2FA() {
  twoFALoading.value = true
  try {
    const res = await securityApi.setup2FA()
    twoFASecret.value = res.data.secret
    twoFAUri.value = res.data.uri
    twoFAQrUrl.value = await createQrCodeDataUrl(res.data.uri, 220)
    twoFACode.value = ''
    showSetup2FA.value = true
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '初始化 2FA 失败')
  } finally {
    twoFALoading.value = false
  }
}

async function handleVerify2FA() {
  if (!twoFACode.value.trim()) {
    ElMessage.warning('请输入验证码')
    return
  }
  try {
    await securityApi.verify2FA(twoFACode.value.trim())
    ElMessage.success('2FA 已启用')
    twoFAEnabled.value = true
    showSetup2FA.value = false
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '验证码错误')
  }
}

async function handleDisable2FA() {
  let prompted: { value: string }
  try {
    prompted = await ElMessageBox.prompt(
      '为了确认操作本人持有认证器，请输入当前的 6 位动态验证码后再禁用 2FA。',
      '禁用双因素认证',
      {
        inputPattern: /^\d{6}$/,
        inputErrorMessage: '请输入 6 位数字验证码',
        confirmButtonText: '确认禁用',
        cancelButtonText: '取消',
        type: 'warning',
        inputPlaceholder: '6 位数字验证码',
        closeOnClickModal: false
      }
    ) as { value: string }
  } catch {
    return
  }

  twoFADisabling.value = true
  try {
    await securityApi.disable2FA(prompted.value.trim())
    twoFAEnabled.value = false
    ElMessage.success('2FA 已禁用')
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '禁用 2FA 失败')
  } finally {
    twoFADisabling.value = false
  }
}

onMounted(async () => {
  if (!authStore.user) {
    try {
      await authStore.fetchUser()
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || '获取用户信息失败，请重新登录')
      return
    }
  }
  load2FAStatus()
})

watch(activeTab, (tab) => {
  if (tab === 'sponsors') {
    void activateSponsorTab()
    return
  }
  stopSponsorRefreshTimer()
})

onUnmounted(() => {
  stopSponsorRefreshTimer()
})
</script>

<template>
  <div class="profile-page">
    <!-- ================= Hero ================= -->
    <header class="profile-hero">
      <div class="profile-hero-aura" aria-hidden="true"></div>

      <div class="profile-hero-main">
        <div class="profile-avatar-wrap">
          <div class="profile-avatar" @click="triggerAvatarUpload" :class="{ 'is-uploading': avatarUploading }">
            <img v-if="avatarUrl" :src="avatarUrl" alt="用户头像" class="profile-avatar-img" />
            <span v-else class="profile-avatar-initial">{{ usernameInitial }}</span>
            <span class="profile-avatar-ring"></span>
            <div class="profile-avatar-overlay">
              <el-icon :size="18"><Camera /></el-icon>
            </div>
            <input
              ref="avatarInputRef"
              type="file"
              accept="image/jpeg,image/png,image/gif,image/webp"
              class="profile-avatar-input"
              @change="handleAvatarFileChange"
            />
          </div>
          <el-button
            v-if="avatarUrl"
            class="avatar-delete-btn"
            :icon="Delete"
            circle
            size="small"
            @click.stop="handleDeleteAvatar"
            title="删除头像"
          />
        </div>

        <div class="profile-hero-body">
          <span class="profile-hero-eyebrow">个人设置</span>
          <h1 class="profile-hero-name">{{ authStore.user?.username || '当前用户' }}</h1>
          <div class="profile-hero-meta-row">
            <span class="hero-chip" :class="'hero-chip--' + roleTagType">
              <el-icon :size="13"><Avatar /></el-icon>
              <span>{{ roleLabel }}</span>
            </span>
            <span class="hero-chip hero-chip--2fa" :class="{ 'hero-chip--2fa-on': twoFAEnabled }">
              <span class="hero-chip-dot" :class="{ 'hero-chip-dot--on': twoFAEnabled }"></span>
              <span>2FA {{ twoFAEnabled ? '已启用' : '未启用' }}</span>
            </span>
            <span class="hero-chip hero-chip--muted">
              <el-icon :size="13"><Clock /></el-icon>
              <span>最后登录 {{ formatTime(authStore.user?.last_login_at) }}</span>
            </span>
          </div>
        </div>
      </div>
    </header>

    <!-- ================= Tabs ================= -->
    <el-tabs v-model="activeTab" class="profile-tabs">
      <el-tab-pane name="security">
        <template #label>
          <span class="profile-tab-label">
            <el-icon :size="14"><Lock /></el-icon>
            <span>账号安全</span>
          </span>
        </template>

        <div class="profile-grid">
          <!-- Left column -->
          <div class="profile-column profile-column--left">
            <section class="profile-card profile-card--info">
              <header class="profile-card-header">
                <span class="card-title">
                  <el-icon :size="15"><User /></el-icon>
                  <span>账户信息</span>
                </span>
              </header>
              <div class="info-grid">
                <div class="info-row">
                  <span class="info-label">用户名</span>
                  <span class="info-value">{{ authStore.user?.username || '-' }}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">角色</span>
                  <span class="info-value">
                    <el-tag :type="roleTagType" size="small" effect="light">{{ roleLabel }}</el-tag>
                  </span>
                </div>
                <div class="info-row">
                  <span class="info-label">注册时间</span>
                  <span class="info-value">{{ formatTime(authStore.user?.created_at) }}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">最近登录</span>
                  <span class="info-value">{{ formatTime(authStore.user?.last_login_at) }}</span>
                </div>
              </div>
            </section>

            <section class="profile-card profile-card--tip">
              <header class="profile-card-header">
                <span class="card-title">
                  <el-icon :size="15"><InfoFilled /></el-icon>
                  <span>安全建议</span>
                </span>
              </header>
              <ul class="tip-list">
                <li>
                  <span class="tip-bullet">1</span>
                  <span>密码建议至少 12 位，包含大小写、数字和特殊字符。</span>
                </li>
                <li>
                  <span class="tip-bullet">2</span>
                  <span>启用 2FA 后，即使密码泄露，账户仍有第二层保护。</span>
                </li>
                <li>
                  <span class="tip-bullet">3</span>
                  <span>禁用 2FA 也需要动态验证码，防止会话被劫持后被人关掉 2FA。</span>
                </li>
                <li>
                  <span class="tip-bullet">4</span>
                  <span>修改密码后当前会话之外的其它登录都会被撤销。</span>
                </li>
              </ul>
            </section>
          </div>

          <!-- Right column -->
          <div class="profile-column profile-column--right">
            <section class="profile-card profile-card--password">
              <header class="profile-card-header">
                <span class="card-title">
                  <el-icon :size="15"><Lock /></el-icon>
                  <span>修改密码</span>
                </span>
              </header>
              <el-form label-position="top" class="security-form">
                <el-form-item label="当前密码">
                  <el-input
                    v-model="passwordForm.oldPassword"
                    type="password"
                    show-password
                    placeholder="请输入当前密码"
                  />
                </el-form-item>
                <el-form-item label="新密码">
                  <el-input
                    v-model="passwordForm.newPassword"
                    type="password"
                    show-password
                    placeholder="至少 6 位"
                  />
                </el-form-item>
                <el-form-item label="确认新密码">
                  <el-input
                    v-model="passwordForm.confirmPassword"
                    type="password"
                    show-password
                    placeholder="再次输入新密码"
                    @keyup.enter="handleChangePassword"
                  />
                </el-form-item>
                <el-form-item>
                  <el-button
                    type="primary"
                    :loading="passwordSaving"
                    class="primary-cta"
                    @click="handleChangePassword"
                  >
                    <el-icon><Lock /></el-icon>
                    <span>更新密码</span>
                  </el-button>
                </el-form-item>
              </el-form>
            </section>

            <section class="profile-card profile-card--twofa" :class="{ 'is-on': twoFAEnabled }">
              <div class="twofa-halo" aria-hidden="true"></div>
              <header class="profile-card-header">
                <span class="card-title">
                  <el-icon :size="15"><Key /></el-icon>
                  <span>双因素认证</span>
                </span>
                <span class="twofa-status" :class="{ 'twofa-status--on': twoFAEnabled }">
                  <span class="twofa-status-dot"></span>
                  <span>{{ twoFAEnabled ? '已启用' : '未启用' }}</span>
                </span>
              </header>

              <p class="twofa-desc">
                <template v-if="twoFAEnabled">
                  你已经开启 2FA，登录时会要求输入认证器应用里的 6 位动态码。禁用前需要再次输入当前动态码确认操作。
                </template>
                <template v-else>
                  启用后，登录除了账号密码还需要输入认证器（Google / Microsoft Authenticator 等）生成的 6 位动态码。
                </template>
              </p>

              <div class="twofa-actions">
                <el-button
                  v-if="!twoFAEnabled"
                  type="primary"
                  class="primary-cta"
                  :loading="twoFALoading"
                  @click="handleSetup2FA"
                >
                  <el-icon><Key /></el-icon>
                  <span>启用 2FA</span>
                </el-button>
                <el-button
                  v-else
                  class="danger-outline-btn"
                  :loading="twoFADisabling"
                  @click="handleDisable2FA"
                >
                  <el-icon><Key /></el-icon>
                  <span>禁用 2FA（需动态码）</span>
                </el-button>
              </div>
            </section>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="sponsors" lazy>
        <template #label>
          <span class="profile-tab-label">
            <el-icon :size="14"><Star /></el-icon>
            <span>赞助名单</span>
          </span>
        </template>

        <section class="sponsor-panel">
          <div class="sponsor-toolbar">
            <div class="sponsor-toolbar-copy">
              <div class="sponsor-toolbar-title-row">
                <h3>赞助名单</h3>
                <span class="sponsor-toolbar-hint">{{ sponsorTabHint }}</span>
              </div>
              <p class="sponsor-toolbar-intro">
                本项目长期以公益方式维护，感谢以下赞助人员对持续开发、服务开销与后续迭代的支持。
              </p>
            </div>
            <el-button
              class="sponsor-refresh-btn"
              :loading="sponsorLoading || sponsorRefreshing"
              @click="handleManualSponsorRefresh"
            >
              <el-icon><RefreshRight /></el-icon>
              <span>立即刷新</span>
            </el-button>
          </div>

          <SponsorWall
            :sponsors="sponsors"
            :summary="sponsorSummary"
            :loading="sponsorLoading && sponsors.length === 0"
          />
        </section>
      </el-tab-pane>
    </el-tabs>

    <!-- ================= Setup 2FA dialog ================= -->
    <el-dialog
      v-model="showSetup2FA"
      width="520px"
      :fullscreen="dialogFullscreen"
      :close-on-click-modal="false"
      class="setup-2fa-dialog"
    >
      <template #header>
        <div class="setup-dialog-header">
          <div class="setup-dialog-badge" aria-hidden="true">
            <el-icon :size="16"><Key /></el-icon>
          </div>
          <div>
            <div class="setup-dialog-title">设置双因素认证</div>
            <div class="setup-dialog-sub">扫码 / 抄密钥 / 输入验证码，三步开启</div>
          </div>
        </div>
      </template>

      <div class="setup-2fa">
        <div class="setup-step">
          <div class="step-head">
            <span class="step-num">1</span>
            <span class="step-title">扫描二维码</span>
          </div>
          <div class="qr-wrapper">
            <img v-if="twoFAQrUrl" :src="twoFAQrUrl" alt="2FA QR Code" class="qr-image" />
          </div>
          <div class="step-hint">推荐使用 Google Authenticator、Microsoft Authenticator 或 1Password。</div>
        </div>

        <div class="setup-step">
          <div class="step-head">
            <span class="step-num">2</span>
            <span class="step-title">或手动输入密钥</span>
          </div>
          <div class="secret-box">
            <code>{{ twoFASecret }}</code>
          </div>
        </div>

        <div class="setup-step">
          <div class="step-head">
            <span class="step-num">3</span>
            <span class="step-title">输入 6 位验证码</span>
          </div>
          <el-input
            v-model="twoFACode"
            maxlength="6"
            placeholder="认证器上的 6 位数字"
            size="large"
            class="totp-input"
            @keyup.enter="handleVerify2FA"
          />
        </div>
      </div>

      <template #footer>
        <div class="setup-dialog-footer">
          <el-button @click="showSetup2FA = false">取消</el-button>
          <el-button type="primary" class="primary-cta" @click="handleVerify2FA">验证并启用</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.profile-page {
  --profile-accent: #22c55e;
  --profile-ai-accent: #6366f1;
  --profile-border-soft: color-mix(in srgb, var(--el-border-color-light) 85%, transparent);
  --profile-surface: var(--el-bg-color);
  --profile-surface-muted: color-mix(in srgb, var(--el-fill-color) 70%, transparent);

  display: flex;
  flex-direction: column;
  gap: 18px;
  font-family: var(--dd-font-ui);
}

/* ================= Hero ================= */
.profile-hero {
  position: relative;
  overflow: hidden;
  padding: 26px 28px;
  border-radius: 18px;
  background:
    linear-gradient(135deg,
      color-mix(in srgb, var(--profile-accent) 14%, transparent) 0%,
      color-mix(in srgb, var(--profile-ai-accent) 12%, transparent) 55%,
      transparent 100%),
    var(--profile-surface);
  border: 1px solid var(--profile-border-soft);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

.profile-hero-aura {
  position: absolute;
  inset: auto -80px -120px auto;
  width: 320px;
  height: 320px;
  border-radius: 50%;
  background:
    radial-gradient(circle at center,
      color-mix(in srgb, var(--profile-ai-accent) 30%, transparent) 0%,
      transparent 70%);
  pointer-events: none;
}

.profile-hero-main {
  position: relative;
  display: flex;
  align-items: center;
  gap: 20px;
  min-width: 0;
  flex-wrap: wrap;
}

.profile-avatar-wrap {
  position: relative;
  flex-shrink: 0;
}

.profile-avatar {
  position: relative;
  width: 68px;
  height: 68px;
  border-radius: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-family: var(--dd-font-ui);
  font-size: 26px;
  font-weight: 700;
  background: linear-gradient(135deg, #22c55e 0%, #6366f1 100%);
  box-shadow: 0 12px 24px -12px rgba(34, 197, 94, 0.55);
  flex-shrink: 0;
  cursor: pointer;
  overflow: hidden;
  transition: transform 0.18s;

  &:hover {
    transform: scale(1.04);

    .profile-avatar-overlay {
      opacity: 1;
    }
  }

  &.is-uploading {
    opacity: 0.6;
    pointer-events: none;
  }
}

.profile-avatar-img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: inherit;
  z-index: 1;
}

.profile-avatar-overlay {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  border-radius: inherit;
  opacity: 0;
  transition: opacity 0.2s;
  color: #fff;
}

.profile-avatar-input {
  display: none;
}

.avatar-delete-btn {
  position: absolute;
  bottom: -4px;
  right: -4px;
  z-index: 3;
  color: var(--el-color-danger) !important;
  border-color: var(--profile-border-soft) !important;
  background: var(--profile-surface) !important;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);

  &:hover {
    color: #fff !important;
    background: var(--el-color-danger) !important;
    border-color: var(--el-color-danger) !important;
  }
}

.profile-avatar-initial {
  position: relative;
  z-index: 1;
  letter-spacing: 0.5px;
}

.profile-avatar-ring {
  position: absolute;
  inset: -4px;
  border-radius: inherit;
  border: 2px solid color-mix(in srgb, var(--profile-accent) 35%, transparent);
  z-index: 0;
}

.profile-hero-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  flex: 1;
}

.profile-hero-eyebrow {
  font-size: 11.5px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--el-text-color-placeholder);
  font-weight: 600;
}

.profile-hero-name {
  margin: 0;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: 0.2px;
  color: var(--el-text-color-primary);
  line-height: 1.1;
}

.profile-hero-meta-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 4px;
}

.hero-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.2px;
  background: color-mix(in srgb, var(--el-fill-color) 90%, transparent);
  border: 1px solid var(--profile-border-soft);
  color: var(--el-text-color-regular);
}

.hero-chip--danger { color: var(--el-color-danger); background: color-mix(in srgb, var(--el-color-danger) 10%, transparent); border-color: color-mix(in srgb, var(--el-color-danger) 28%, transparent); }
.hero-chip--warning { color: var(--el-color-warning); background: color-mix(in srgb, var(--el-color-warning) 10%, transparent); border-color: color-mix(in srgb, var(--el-color-warning) 28%, transparent); }
.hero-chip--info { color: var(--el-color-info); background: color-mix(in srgb, var(--el-color-info) 10%, transparent); border-color: color-mix(in srgb, var(--el-color-info) 28%, transparent); }
.hero-chip--muted { color: var(--el-text-color-secondary); }

.hero-chip--2fa {
  background: color-mix(in srgb, var(--el-color-danger) 8%, transparent);
  border-color: color-mix(in srgb, var(--el-color-danger) 22%, transparent);
  color: var(--el-color-danger);
}

.hero-chip--2fa-on {
  background: color-mix(in srgb, var(--profile-accent) 12%, transparent);
  border-color: color-mix(in srgb, var(--profile-accent) 28%, transparent);
  color: color-mix(in srgb, var(--profile-accent) 80%, var(--el-text-color-primary));
}

.hero-chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-color-danger);
}

.hero-chip-dot--on {
  background: var(--profile-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--profile-accent) 22%, transparent);
}

/* ================= Tabs ================= */
.profile-tabs {
  :deep(.el-tabs__header) {
    margin: 0;
  }

  :deep(.el-tabs__nav-wrap::after) {
    height: 1px;
    background: var(--profile-border-soft);
  }

  :deep(.el-tabs__item) {
    height: 42px;
    font-weight: 600;
    font-size: 13.5px;
  }
}

.profile-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.profile-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
  gap: 16px;
  padding-top: 16px;
}

.profile-column {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

/* ================= Cards ================= */
.profile-card {
  position: relative;
  background: var(--profile-surface);
  border: 1px solid var(--profile-border-soft);
  border-radius: 14px;
  padding: 18px 20px;
  overflow: hidden;
  transition: border-color 0.2s, box-shadow 0.2s;

  &:hover {
    border-color: color-mix(in srgb, var(--profile-accent) 20%, var(--profile-border-soft));
  }
}

.profile-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.card-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14.5px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

/* Info grid */
.info-grid {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px dashed var(--profile-border-soft);
  font-size: 13.5px;

  &:last-child {
    border-bottom: none;
    padding-bottom: 2px;
  }
}

.info-label {
  color: var(--el-text-color-secondary);
  font-size: 12.5px;
  letter-spacing: 0.2px;
}

.info-value {
  font-weight: 600;
  color: var(--el-text-color-primary);
  text-align: right;
  word-break: break-all;
}

/* Tips */
.tip-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;

  li {
    display: flex;
    gap: 10px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--profile-surface-muted);
    font-size: 12.5px;
    line-height: 1.6;
    color: var(--el-text-color-regular);
  }
}

.tip-bullet {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
  font-family: var(--dd-font-mono);
  color: var(--profile-accent);
  background: color-mix(in srgb, var(--profile-accent) 14%, transparent);
}

/* Password */
.security-form {
  :deep(.el-form-item) {
    margin-bottom: 14px;
  }

  :deep(.el-form-item__label) {
    font-size: 12.5px;
    font-weight: 600;
    color: var(--el-text-color-secondary);
  }

  :deep(.el-input__wrapper) {
    border-radius: 10px;
  }
}

.primary-cta {
  border-radius: 10px;
  height: 38px;
  padding: 0 18px;
  font-weight: 600;
  background: linear-gradient(135deg, #22c55e 0%, #16a34a 100%);
  border: none;
  box-shadow: 0 8px 20px -12px rgba(34, 197, 94, 0.55);

  &:hover,
  &:focus {
    background: linear-gradient(135deg, #16a34a 0%, #15803d 100%);
    border: none;
  }
}

/* Two-factor card */
.profile-card--twofa {
  border-color: color-mix(in srgb, var(--el-color-danger) 22%, var(--profile-border-soft));
  background:
    linear-gradient(135deg,
      color-mix(in srgb, var(--el-color-danger) 5%, transparent) 0%,
      transparent 55%),
    var(--profile-surface);

  &.is-on {
    border-color: color-mix(in srgb, var(--profile-accent) 30%, var(--profile-border-soft));
    background:
      linear-gradient(135deg,
        color-mix(in srgb, var(--profile-accent) 8%, transparent) 0%,
        transparent 60%),
      var(--profile-surface);
  }
}

.twofa-halo {
  position: absolute;
  inset: auto -60px -80px auto;
  width: 180px;
  height: 180px;
  border-radius: 50%;
  background: radial-gradient(circle,
    color-mix(in srgb, var(--profile-ai-accent) 24%, transparent) 0%,
    transparent 70%);
  pointer-events: none;
}

.twofa-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 700;
  font-family: var(--dd-font-mono);
  letter-spacing: 0.5px;
  background: color-mix(in srgb, var(--el-color-danger) 12%, transparent);
  color: var(--el-color-danger);

  &--on {
    background: color-mix(in srgb, var(--profile-accent) 14%, transparent);
    color: color-mix(in srgb, var(--profile-accent) 80%, var(--el-text-color-primary));
  }
}

.twofa-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.twofa-desc {
  margin: 0 0 14px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}

.twofa-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.danger-outline-btn {
  border-radius: 10px;
  height: 38px;
  padding: 0 16px;
  font-weight: 600;
  color: var(--el-color-danger);
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--el-color-danger) 50%, transparent);

  &:hover {
    color: #fff;
    background: var(--el-color-danger);
    border-color: var(--el-color-danger);
  }
}

/* ================= Sponsor panel ================= */
.sponsor-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 16px;
}

.sponsor-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  padding: 16px 20px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, #f59e0b 16%, transparent);
  background:
    linear-gradient(135deg,
      color-mix(in srgb, #f59e0b 10%, transparent) 0%,
      color-mix(in srgb, #f59e0b 4%, transparent) 100%),
    var(--profile-surface);
}

.sponsor-toolbar-copy {
  min-width: 0;
  flex: 1;
}

.sponsor-toolbar-title-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-wrap: wrap;

  h3 {
    margin: 0;
    font-size: 17px;
    font-weight: 700;
    color: #b45309;
  }
}

.sponsor-toolbar-hint {
  font-size: 11.5px;
  color: #a16207;
}

.sponsor-toolbar-intro {
  margin: 6px 0 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: color-mix(in srgb, #92400e 80%, var(--el-text-color-regular));
  max-width: 680px;
}

.sponsor-refresh-btn {
  border-radius: 10px;
}

/* ================= Setup dialog ================= */
:deep(.setup-2fa-dialog) {
  .el-dialog {
    border-radius: 16px;
    overflow: hidden;
  }

  .el-dialog__header {
    padding: 18px 20px 14px;
    margin: 0;
    border-bottom: 1px solid var(--profile-border-soft);
  }

  .el-dialog__body {
    padding: 20px 24px;
  }

  .el-dialog__footer {
    padding: 12px 20px 18px;
  }
}

.setup-dialog-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.setup-dialog-badge {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
}

.setup-dialog-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.setup-dialog-sub {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.setup-2fa {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.setup-step {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.step-num {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
  font-family: var(--dd-font-mono);
  color: #fff;
  background: linear-gradient(135deg, #22c55e, #16a34a);
}

.step-title {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.step-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
}

.qr-wrapper {
  display: flex;
  justify-content: center;
  padding: 8px 0;
}

.qr-image {
  width: 220px;
  height: 220px;
  padding: 10px;
  border-radius: 18px;
  background: #fff;
  border: 1px solid var(--profile-border-soft);
}

.secret-box {
  padding: 14px 16px;
  border-radius: 12px;
  background: var(--profile-surface-muted);
  border: 1px dashed var(--profile-border-soft);
  text-align: center;

  code {
    font-family: var(--dd-font-mono);
    font-size: 14.5px;
    font-weight: 700;
    letter-spacing: 0.18em;
    user-select: all;
    color: var(--el-text-color-primary);
    word-break: break-all;
  }
}

.totp-input {
  :deep(.el-input__wrapper) {
    border-radius: 12px;
  }

  :deep(.el-input__inner) {
    font-family: var(--dd-font-mono);
    font-size: 18px;
    letter-spacing: 0.5em;
    text-align: center;
    font-weight: 600;
  }
}

.setup-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* ================= Mobile ================= */
@media (max-width: 900px) {
  .profile-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 600px) {
  .profile-hero {
    padding: 20px;
  }

  .profile-hero-name {
    font-size: 22px;
  }

  .profile-hero-meta-row {
    gap: 6px;
  }

  .hero-chip {
    font-size: 11.5px;
  }
}
</style>
