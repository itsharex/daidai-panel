<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { systemApi } from '@/api/system'
import { loadPanelSettings as loadCachedPanelSettings } from '@/utils/panelSettings'
import { useResponsive } from '@/composables/useResponsive'
import {
  Bell,
  Box,
  Connection,
  Document,
  Download,
  Expand,
  Fold,
  Key,
  MagicStick,
  Moon,
  Odometer,
  Operation,
  SetUp,
  Setting,
  Sunny,
  Tickets,
  Timer,
  User,
  UserFilled,
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const themeStore = useThemeStore()
const { isMobile } = useResponsive()
const isCollapsed = ref(false)
const drawerVisible = ref(false)
const panelTitle = ref('呆呆面板')
const panelIcon = ref('')
const panelVersion = ref('')

const roleLevel: Record<string, number> = {
  viewer: 1,
  operator: 2,
  admin: 3,
}

function hasRole(minRole: string) {
  const currentRole = authStore.user?.role
  if (!currentRole) return false
  return (roleLevel[currentRole] || 0) >= (roleLevel[minRole] || 0)
}

const currentSection = computed(() => {
  const matched = [...route.matched].reverse().find(item => item.meta.section)
  return matched?.meta.section === 'admin' ? 'admin' : 'workspace'
})

const canAccessAdmin = computed(() => hasRole('admin'))

const workspaceItems = [
  { index: '/dashboard', title: '仪表板', icon: Odometer, minRole: 'viewer' },
  { index: '/tasks', title: '定时任务', icon: Timer, minRole: 'viewer' },
  { index: '/subscriptions', title: '订阅管理', icon: Download, minRole: 'operator' },
  { index: '/envs', title: '环境变量', icon: Setting, minRole: 'operator' },
  { index: '/logs', title: '执行日志', icon: Tickets, minRole: 'viewer' },
  { index: '/scripts', title: '脚本管理', icon: Document, minRole: 'operator' },
  { index: '/ai-code', title: 'Ai Code', icon: MagicStick, minRole: 'operator' },
  { index: '/deps', title: '依赖管理', icon: Box, minRole: 'admin' },
  { index: '/api-docs', title: '接口文档', icon: Connection, minRole: 'viewer' },
  { index: '/profile', title: '个人设置', icon: User, minRole: 'viewer' },
]

const adminItems = [
  { index: '/admin/settings', title: '系统设置', icon: SetUp, minRole: 'admin' },
  { index: '/admin/notifications', title: '通知渠道', icon: Bell, minRole: 'admin' },
  { index: '/admin/users', title: '用户管理', icon: UserFilled, minRole: 'admin' },
  { index: '/admin/open-api', title: 'Open API', icon: Key, minRole: 'admin' },
]

onMounted(() => {
  loadPanelSettings()
  loadVersion()
  if (authStore.isLoggedIn && !authStore.user) {
    authStore.fetchUser()
  }
})

watch(isMobile, (mobile) => {
  if (mobile) {
    isCollapsed.value = true
    return
  }
  drawerVisible.value = false
}, { immediate: true })

const menuItems = computed(() => {
  const allItems = currentSection.value === 'admin' ? adminItems : workspaceItems
  return allItems.filter(item => hasRole(item.minRole))
})

const activeMenu = computed(() => route.path)
const sectionLabel = computed(() => currentSection.value === 'admin' ? '管理后台' : '工作台')
const sectionActionLabel = computed(() => currentSection.value === 'admin' ? '返回工作台' : '进入后台')
const sidebarToggleIcon = computed(() => (isMobile.value ? Operation : (isCollapsed.value ? Expand : Fold)))
const themeIcon = computed(() => (themeStore.isDark ? Sunny : Moon))

function handleMenuSelect(index: string) {
  router.push(index)
  if (isMobile.value) drawerVisible.value = false
}

function toggleSidebar() {
  if (isMobile.value) {
    drawerVisible.value = !drawerVisible.value
  } else {
    isCollapsed.value = !isCollapsed.value
  }
}

async function handleLogout() {
  await authStore.logout()
}

function toggleSection() {
  if (!canAccessAdmin.value) return
  router.push(currentSection.value === 'admin' ? '/dashboard' : '/admin/settings')
  if (isMobile.value) drawerVisible.value = false
}

async function loadPanelSettings() {
  try {
    const settings = await loadCachedPanelSettings()
    if (settings?.panel_title) panelTitle.value = settings.panel_title
    if (settings?.panel_icon) panelIcon.value = settings.panel_icon
  } catch {}
}

async function loadVersion() {
  try {
    const res = await systemApi.version() as any
    if (res.data?.version) panelVersion.value = res.data.version
  } catch {}
}
</script>

<template>
  <el-container class="layout-container">
    <el-aside v-if="!isMobile" :width="isCollapsed ? '64px' : '220px'" class="layout-aside">
      <div class="logo-area" :class="{ 'is-collapsed': isCollapsed }">
        <div class="brand-shell">
          <div class="brand-mark">
            <img :src="panelIcon || '/favicon.svg'" alt="logo" class="logo-img" />
          </div>
          <div v-show="!isCollapsed" class="brand-copy">
            <span class="logo-text">{{ panelTitle }}</span>
            <span v-if="panelVersion" class="version-badge">v{{ panelVersion }}</span>
          </div>
        </div>
      </div>
      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapsed"
        :collapse-transition="false"
        background-color="transparent"
        @select="handleMenuSelect"
      >
        <el-menu-item v-for="item in menuItems" :key="item.index" :index="item.index">
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ item.title }}</template>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-drawer
      v-if="isMobile"
      v-model="drawerVisible"
      direction="ltr"
      :size="240"
      :with-header="false"
      :show-close="false"
    >
      <div class="logo-area mobile-logo">
        <div class="brand-shell">
          <div class="brand-mark">
            <img :src="panelIcon || '/favicon.svg'" alt="logo" class="logo-img" />
          </div>
          <div class="brand-copy">
            <span class="logo-text">{{ panelTitle }}</span>
            <span v-if="panelVersion" class="version-badge">v{{ panelVersion }}</span>
          </div>
        </div>
      </div>
      <el-menu
        :default-active="activeMenu"
        background-color="transparent"
        @select="handleMenuSelect"
      >
        <el-menu-item v-for="item in menuItems" :key="item.index" :index="item.index">
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ item.title }}</template>
        </el-menu-item>
      </el-menu>
      <div class="mobile-drawer-footer">
        <el-button
          v-if="canAccessAdmin"
          class="mobile-footer-btn"
          @click="toggleSection"
        >
          {{ sectionActionLabel }}
        </el-button>
        <div class="mobile-footer-actions">
          <el-button text class="mobile-footer-link" @click="router.push('/profile'); drawerVisible = false">
            个人设置
          </el-button>
          <el-button text class="mobile-footer-link" @click="handleLogout">
            退出登录
          </el-button>
        </div>
      </div>
    </el-drawer>

    <el-container>
      <el-header class="layout-header">
        <div class="header-left">
          <el-button :icon="sidebarToggleIcon" text @click="toggleSidebar" />
          <div v-if="isMobile" class="mobile-brand-inline">
            <span class="mobile-title">{{ panelTitle }}</span>
            <span v-if="panelVersion" class="mobile-version">v{{ panelVersion }}</span>
          </div>
          <el-tag v-if="!isMobile" size="small" effect="plain" class="section-tag">{{ sectionLabel }}</el-tag>
        </div>
        <div class="header-right">
          <el-button v-if="canAccessAdmin && !isMobile" text class="section-switch-btn" @click="toggleSection">
            {{ sectionActionLabel }}
          </el-button>
          <el-button :icon="themeIcon" text circle class="theme-btn" @click="themeStore.toggleTheme" />
          <el-dropdown trigger="click">
            <span class="user-dropdown">
              <el-icon><User /></el-icon>
              <span v-if="!isMobile">{{ authStore.user?.username || 'User' }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="router.push('/profile')">个人设置</el-dropdown-item>
                <el-dropdown-item v-if="canAccessAdmin" @click="router.push('/admin/settings')">系统设置</el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="layout-main">
        <router-view v-slot="{ Component, route: viewRoute }">
          <keep-alive :max="3">
            <component :is="Component" :key="viewRoute.path" />
          </keep-alive>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped lang="scss">
.layout-container {
  height: 100vh;
}

.layout-aside {
  border-right: 1px solid var(--el-border-color-light);
  transition: width 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--el-bg-color);
  will-change: width;
}

.logo-area {
  height: 60px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--el-border-color-light);
  flex-shrink: 0;
  padding: 8px 10px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--el-color-primary-light-9) 48%, white) 0%, var(--el-bg-color) 100%);

  &.is-collapsed {
    justify-content: center;
    padding-inline: 8px;
  }
}

.brand-shell {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  min-height: 42px;
  border-radius: 14px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--el-bg-color) 88%, white) 0%, var(--el-bg-color) 100%);
  border: 1px solid color-mix(in srgb, var(--el-color-primary) 10%, var(--el-border-color-lighter));
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.04);
  transition: box-shadow var(--dd-transition), border-color var(--dd-transition);

  .logo-area:hover & {
    box-shadow: 0 6px 16px rgba(15, 23, 42, 0.06);
    border-color: color-mix(in srgb, var(--el-color-primary) 18%, var(--el-border-color-lighter));
  }
}

.logo-area.is-collapsed .brand-shell {
  width: 40px;
  min-height: 40px;
  padding: 6px;
  justify-content: center;
}

.brand-mark {
  width: 28px;
  height: 28px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background:
    linear-gradient(145deg, var(--el-color-primary-light-8), var(--el-color-primary-light-9));
  border: 1px solid color-mix(in srgb, var(--el-color-primary) 18%, transparent);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.55),
    0 4px 10px rgba(64, 158, 255, 0.12);
  transition: transform var(--dd-transition), box-shadow var(--dd-transition);

  .logo-area:hover & {
    transform: translateY(-1px);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.55),
      0 6px 12px rgba(64, 158, 255, 0.15);
  }
}

.logo-img {
  width: 16px;
  height: 16px;
}

.brand-copy {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.logo-text {
  min-width: 0;
  flex: 1;
  font-size: 15px;
  line-height: 1;
  font-weight: 700;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.version-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 999px;
  font-family: var(--dd-font-mono);
  font-size: 10px;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--el-color-primary-dark-2);
  background: color-mix(in srgb, var(--el-color-primary) 10%, white);
  border: 1px solid color-mix(in srgb, var(--el-color-primary) 18%, transparent);
}

.mobile-logo {
  border-bottom: 1px solid var(--el-border-color-light);
  justify-content: flex-start;
  padding: 10px 12px;
}

.mobile-brand-inline {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.mobile-title {
  min-width: 0;
  font-size: 15px;
  line-height: 1;
  font-weight: 700;
  color: var(--el-text-color-primary);
  letter-spacing: 0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-version {
  flex-shrink: 0;
  padding: 2px 6px;
  border-radius: 999px;
  font-family: var(--dd-font-mono);
  font-size: 10px;
  font-weight: 700;
  line-height: 1.1;
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 10%, var(--el-bg-color));
  border: 1px solid color-mix(in srgb, var(--el-color-primary) 18%, transparent);
}

.layout-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  padding: 0 20px;
  background: var(--el-bg-color);
  backdrop-filter: blur(8px);
  position: sticky;
  top: 0;
  z-index: 20;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-tag {
  border-radius: 999px;
  padding: 0 10px;
}

.section-switch-btn {
  font-weight: 600;
  border-radius: 999px;
}

.user-dropdown {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 6px 12px;
  border-radius: 8px;
  transition: all 0.2s cubic-bezier(0.25, 0.46, 0.45, 0.94);
  font-weight: 500;

  &:hover {
    background: var(--el-fill-color-light);
    color: var(--el-color-primary);
    transform: translateY(-1px);
  }
}

.layout-main {
  background: var(--el-bg-color-page);
  overflow-y: auto;
  padding: 20px;
}

.mobile-drawer-footer {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 12px 18px;
  border-top: 1px solid var(--el-border-color-light);
  margin-top: 8px;
}

.mobile-footer-btn {
  width: 100%;
}

.mobile-footer-actions {
  display: flex;
  gap: 8px;
}

.mobile-footer-link {
  flex: 1;
  justify-content: center;
}

.theme-btn {
  transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1) !important;

  &:hover {
    transform: rotate(20deg) scale(1.15) !important;
  }

  &:active {
    transform: rotate(0deg) scale(0.95) !important;
  }
}

.page-fade-enter-active {
  animation: pageEnter 0.22s ease-out;
}

.page-fade-leave-active {
  animation: pageFadeOut 0.12s ease-in;
}

@keyframes pageEnter {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes pageFadeOut {
  from { opacity: 1; }
  to { opacity: 0; }
}

@media screen and (max-width: 768px) {
  .layout-header {
    padding: 0 10px;
    height: 54px;
  }

  .layout-main {
    padding: max(12px, env(safe-area-inset-top)) 12px calc(16px + env(safe-area-inset-bottom));
  }

  .logo-area {
    height: 56px;
    padding: 8px 10px;
  }

  .brand-shell {
    min-height: 38px;
    padding: 6px 9px;
  }

  .brand-mark {
    width: 26px;
    height: 26px;
  }

  .logo-img {
    width: 15px;
    height: 15px;
  }

  .header-left {
    gap: 6px;
  }

  .header-right {
    gap: 2px;
  }

  .user-dropdown {
    padding: 6px 8px;
  }

  .mobile-brand-inline {
    min-width: 0;
    max-width: 52vw;
  }
}
</style>
