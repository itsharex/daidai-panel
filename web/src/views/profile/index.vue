<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheck, Key, Lock, User } from '@element-plus/icons-vue'
import { authApi } from '@/api/auth'
import { securityApi } from '@/api/security'
import { useAuthStore } from '@/stores/auth'
import { createQrCodeDataUrl } from '@/utils/qrcode'
import { useResponsive } from '@/composables/useResponsive'

const authStore = useAuthStore()
const { dialogFullscreen } = useResponsive()

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
    setTimeout(() => {
      authStore.logout()
    }, 1200)
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
  try {
    await securityApi.disable2FA()
    twoFAEnabled.value = false
    ElMessage.success('2FA 已禁用')
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '禁用 2FA 失败')
  }
}

onMounted(async () => {
  if (!authStore.user) {
    try {
      await authStore.fetchUser()
    } catch {
      return
    }
  }
  load2FAStatus()
})
</script>

<template>
  <div class="profile-page">
    <section class="profile-hero">
      <div class="profile-hero__content">
        <div class="profile-avatar">
          <el-icon :size="24"><User /></el-icon>
        </div>
        <div class="profile-hero__text">
          <p class="profile-eyebrow">个人设置</p>
          <h2>{{ authStore.user?.username || '当前用户' }}</h2>
          <p class="profile-subtitle">这里仅保留账号自身相关的安全设置，系统管理能力已单独收拢到管理员后台。</p>
        </div>
      </div>
      <div class="profile-hero__meta">
        <el-tag :type="roleTagType" effect="dark">{{ roleLabel }}</el-tag>
        <span>最近登录：{{ formatTime(authStore.user?.last_login_at) }}</span>
      </div>
    </section>

    <el-row :gutter="16" class="profile-grid">
      <el-col :xs="24" :lg="10">
        <el-card shadow="never" class="profile-card summary-card">
          <template #header>
            <div class="card-header">
              <span class="card-title"><el-icon><User /></el-icon>账户信息</span>
            </div>
          </template>
          <div class="summary-list">
            <div class="summary-item">
              <span class="summary-label">用户名</span>
              <span class="summary-value">{{ authStore.user?.username || '-' }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">角色</span>
              <span class="summary-value">{{ roleLabel }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">创建时间</span>
              <span class="summary-value">{{ formatTime(authStore.user?.created_at) }}</span>
            </div>
            <div class="summary-item">
              <span class="summary-label">最后登录</span>
              <span class="summary-value">{{ formatTime(authStore.user?.last_login_at) }}</span>
            </div>
          </div>
        </el-card>

        <el-card shadow="never" class="profile-card tip-card">
          <template #header>
            <div class="card-header">
              <span class="card-title"><el-icon><CircleCheck /></el-icon>安全建议</span>
            </div>
          </template>
          <div class="tip-list">
            <div class="tip-item">密码建议使用 12 位以上，包含大小写、数字和特殊字符。</div>
            <div class="tip-item">启用 2FA 后，即使密码泄露，账户仍有第二层保护。</div>
            <div class="tip-item">如果刚修改密码，当前会话外的其他登录会被撤销，需要重新登录。</div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="14">
        <el-card shadow="never" class="profile-card">
          <template #header>
            <div class="card-header">
              <span class="card-title"><el-icon><Lock /></el-icon>修改密码</span>
            </div>
          </template>
          <el-form label-position="top" class="security-form">
            <el-form-item label="当前密码">
              <el-input v-model="passwordForm.oldPassword" type="password" show-password placeholder="请输入当前密码" />
            </el-form-item>
            <el-form-item label="新密码">
              <el-input v-model="passwordForm.newPassword" type="password" show-password placeholder="至少 6 位" />
            </el-form-item>
            <el-form-item label="确认新密码">
              <el-input v-model="passwordForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" @keyup.enter="handleChangePassword" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="passwordSaving" @click="handleChangePassword">
                <el-icon><Lock /></el-icon>更新密码
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" class="profile-card twofa-card">
          <template #header>
            <div class="card-header">
              <span class="card-title"><el-icon><Key /></el-icon>双因素认证</span>
              <el-tag :type="twoFAEnabled ? 'success' : 'info'" size="small" effect="plain">
                {{ twoFAEnabled ? '已启用' : '未启用' }}
              </el-tag>
            </div>
          </template>
          <p class="twofa-desc">
            登录时除了密码，还需要输入认证器应用生成的动态验证码。建议管理员和运维用户至少启用一次。
          </p>
          <div class="twofa-actions">
            <el-button v-if="!twoFAEnabled" type="primary" :loading="twoFALoading" @click="handleSetup2FA">
              <el-icon><Key /></el-icon>启用 2FA
            </el-button>
            <el-button v-else type="danger" plain @click="handleDisable2FA">禁用 2FA</el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-dialog v-model="showSetup2FA" title="设置双因素认证" width="500px" :fullscreen="dialogFullscreen" :close-on-click-modal="false">
      <div class="setup-2fa">
        <div class="setup-step">
          <div class="step-title">步骤 1：扫描二维码</div>
          <div class="qr-wrapper">
            <img v-if="twoFAQrUrl" :src="twoFAQrUrl" alt="2FA QR Code" class="qr-image" />
          </div>
          <div class="step-hint">推荐使用 Google Authenticator、Microsoft Authenticator 或其他 TOTP 应用。</div>
        </div>

        <div class="setup-step">
          <div class="step-title">步骤 2：手动密钥</div>
          <div class="secret-box">
            <code>{{ twoFASecret }}</code>
          </div>
        </div>

        <div class="setup-step">
          <div class="step-title">步骤 3：输入验证码</div>
          <el-input v-model="twoFACode" maxlength="6" placeholder="请输入 6 位验证码" size="large" @keyup.enter="handleVerify2FA" />
        </div>
      </div>
      <template #footer>
        <el-button @click="showSetup2FA = false">取消</el-button>
        <el-button type="primary" @click="handleVerify2FA">验证并启用</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.profile-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.profile-hero {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 24px 28px;
  border-radius: 24px;
  color: #fff;
  background:
    radial-gradient(circle at top right, rgba(255, 255, 255, 0.18), transparent 30%),
    linear-gradient(135deg, #0f766e 0%, #0ea5e9 100%);
}

.profile-hero__content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.profile-avatar {
  width: 56px;
  height: 56px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(8px);
}

.profile-hero__text {
  h2 {
    margin: 0 0 6px;
    font-size: 28px;
    font-weight: 700;
  }
}

.profile-eyebrow {
  margin: 0 0 6px;
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  opacity: 0.88;
}

.profile-subtitle {
  margin: 0;
  max-width: 620px;
  line-height: 1.7;
  color: rgba(255, 255, 255, 0.9);
}

.profile-hero__meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
  gap: 10px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.88);
}

.profile-grid {
  margin: 0 !important;
}

.profile-card {
  border-radius: 20px;

  :deep(.el-card__body) {
    padding: 22px 24px;
  }
}

.summary-card,
.tip-card,
.twofa-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.card-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.summary-list,
.tip-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.summary-item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);

  &:last-child {
    padding-bottom: 0;
    border-bottom: none;
  }
}

.summary-label {
  color: var(--el-text-color-secondary);
}

.summary-value {
  text-align: right;
  font-weight: 600;
  word-break: break-all;
}

.tip-item {
  padding: 12px 14px;
  border-radius: 14px;
  background: var(--el-fill-color-light);
  line-height: 1.7;
  color: var(--el-text-color-regular);
}

.security-form {
  max-width: 520px;
}

.twofa-desc {
  margin: 0 0 18px;
  color: var(--el-text-color-secondary);
  line-height: 1.7;
}

.twofa-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.setup-2fa {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.step-title {
  font-weight: 600;
  margin-bottom: 10px;
}

.step-hint {
  margin-top: 10px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
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
}

.secret-box {
  padding: 14px 16px;
  border-radius: 14px;
  background: var(--el-fill-color-light);
  text-align: center;

  code {
    font-size: 15px;
    font-weight: 700;
    letter-spacing: 0.16em;
    user-select: all;
  }
}

@media (max-width: 900px) {
  .profile-hero {
    flex-direction: column;
    padding: 20px;
  }

  .profile-hero__meta {
    align-items: flex-start;
  }

  .profile-hero__content {
    align-items: flex-start;
  }

  .profile-hero__text h2 {
    font-size: 24px;
  }
}
</style>
