<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { userApi } from '@/api/security'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useResponsive } from '@/composables/useResponsive'

const authStore = useAuthStore()
const { isMobile, dialogFullscreen } = useResponsive()
const users = ref<any[]>([])
const loading = ref(false)

const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)

const showCreateDialog = ref(false)
const showResetPwdDialog = ref(false)

const createForm = ref({ username: '', password: '', role: 'operator' })
const resetPwdForm = ref({ id: 0, username: '', password: '' })

const filteredUsers = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return users.value
  return users.value.filter(u =>
    (u.username || '').toLowerCase().includes(k) ||
    (u.role || '').toLowerCase().includes(k)
  )
})

const pagedUsers = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredUsers.value.slice(start, start + pageSize.value)
})

const total = computed(() => filteredUsers.value.length)

function handleSearch() { page.value = 1 }

async function loadUsers() {
  loading.value = true
  try {
    const res = await userApi.list()
    users.value = res.data || []
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '加载用户列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadUsers)

function openCreate() {
  createForm.value = { username: '', password: '', role: 'operator' }
  showCreateDialog.value = true
}

function validatePassword(pwd: string): string | null {
  if (pwd.length < 6) return '密码至少 6 位'
  if (pwd.length > 72) return '密码不能超过 72 位'
  // 至少包含字母和数字
  if (!/[A-Za-z]/.test(pwd) || !/\d/.test(pwd)) return '密码需同时包含字母和数字'
  return null
}

async function handleCreate() {
  const username = createForm.value.username.trim()
  if (!username) {
    ElMessage.warning('用户名不能为空')
    return
  }
  if (!/^[A-Za-z0-9_]{3,32}$/.test(username)) {
    ElMessage.warning('用户名须为 3-32 位字母/数字/下划线')
    return
  }
  const pwdErr = validatePassword(createForm.value.password)
  if (pwdErr) {
    ElMessage.warning(pwdErr)
    return
  }
  try {
    await userApi.create({ ...createForm.value, username })
    ElMessage.success('创建成功')
    showCreateDialog.value = false
    loadUsers()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '创建失败')
  }
}

async function handleToggle(row: any) {
  try {
    const enabling = !row.enabled
    await ElMessageBox.confirm(
      enabling
        ? `确认启用用户 ${row.username} 吗？`
        : `确认禁用用户 ${row.username} 吗？禁用后该账号将无法继续登录。`,
      enabling ? '启用确认' : '禁用确认',
      { type: enabling ? 'info' : 'warning' }
    )
    await userApi.update(row.id, { enabled: !row.enabled })
    ElMessage.success(row.enabled ? '已禁用' : '已启用')
    loadUsers()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error(err?.response?.data?.error || '操作失败')
  }
}

async function handleRoleChange(row: any, role: string) {
  const originalRole = row.role
  if (role === originalRole) return
  const roleName = getRoleName(role)
  try {
    const tips = originalRole === 'admin' && role !== 'admin'
      ? `确认将用户 ${row.username} 从「管理员」降级为「${roleName}」吗？此操作将立即剥夺该用户的管理员权限。`
      : role === 'admin'
        ? `确认将用户 ${row.username} 提升为「管理员」吗？管理员将拥有全部权限。`
        : `确认将用户 ${row.username} 的角色改为「${roleName}」吗？`
    await ElMessageBox.confirm(tips, '角色变更', { type: 'warning' })
    await userApi.update(row.id, { role })
    ElMessage.success('角色更新成功')
    loadUsers()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') {
      // 回滚 UI
      loadUsers()
      return
    }
    ElMessage.error(err?.response?.data?.error || '更新失败')
    loadUsers()
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(`确定要删除用户 ${row.username} 吗？`, '确认删除', { type: 'warning' })
    await userApi.delete(row.id)
    ElMessage.success('删除成功')
    loadUsers()
  } catch (err: any) {
    if (err === 'cancel' || err?.toString?.() === 'cancel') return
    ElMessage.error(err?.response?.data?.error || '删除失败')
  }
}

function openResetPassword(row: any) {
  resetPwdForm.value = { id: row.id, username: row.username, password: '' }
  showResetPwdDialog.value = true
}

async function handleResetPassword() {
  const pwdErr = validatePassword(resetPwdForm.value.password)
  if (pwdErr) {
    ElMessage.warning(pwdErr)
    return
  }
  try {
    await userApi.resetPassword(resetPwdForm.value.id, resetPwdForm.value.password)
    ElMessage.success('密码重置成功')
    showResetPwdDialog.value = false
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.error || '重置失败')
  }
}

function getRoleTag(role: string) {
  switch (role) {
    case 'admin': return 'danger'
    case 'operator': return ''
    case 'viewer': return 'info'
    default: return 'info'
  }
}

function getRoleName(role: string) {
  switch (role) {
    case 'admin': return '管理员'
    case 'operator': return '操作员'
    case 'viewer': return '观察者'
    default: return role
  }
}
</script>

<template>
  <div class="users-page">
    <div class="page-header">
      <div>
        <h2>用户管理</h2>
        <span class="page-subtitle">管理系统用户账户及权限角色</span>
      </div>
      <div class="header-actions">
        <el-input
          v-model="keyword"
          placeholder="搜索用户名/角色"
          clearable
          style="width: 220px"
          @input="handleSearch"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>新建用户
        </el-button>
      </div>
    </div>

    <div v-if="isMobile" class="dd-mobile-list">
      <div
        v-for="row in pagedUsers"
        :key="row.id"
        class="dd-mobile-card"
      >
        <div class="dd-mobile-card__header">
          <div class="dd-mobile-card__title-wrap">
            <span class="dd-mobile-card__title">{{ row.username }}</span>
            <div class="dd-mobile-card__badges">
              <el-tag size="small" :type="getRoleTag(row.role)">{{ getRoleName(row.role) }}</el-tag>
              <el-tag v-if="row.two_factor_enabled" size="small" type="success" effect="plain">2FA</el-tag>
            </div>
          </div>
          <el-switch
            :model-value="row.enabled"
            size="small"
            :disabled="row.username === authStore.user?.username"
            @change="handleToggle(row)"
          />
        </div>

        <div class="dd-mobile-card__body">
          <div class="dd-mobile-card__grid">
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">角色</span>
              <div class="dd-mobile-card__value">
                <el-select
                  :model-value="row.role"
                  size="small"
                  :disabled="row.username === authStore.user?.username"
                  @change="(val: string) => handleRoleChange(row, val)"
                >
                  <el-option value="admin" label="管理员" />
                  <el-option value="operator" label="操作员" />
                  <el-option value="viewer" label="观察者" />
                </el-select>
              </div>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">最后登录</span>
              <span class="dd-mobile-card__value">{{ row.last_login_at ? new Date(row.last_login_at).toLocaleString() : '-' }}</span>
            </div>
            <div class="dd-mobile-card__field">
              <span class="dd-mobile-card__label">创建时间</span>
              <span class="dd-mobile-card__value">{{ new Date(row.created_at).toLocaleString() }}</span>
            </div>
          </div>
          <div class="dd-mobile-card__actions user-card__actions">
            <el-button size="small" type="primary" plain @click="openResetPassword(row)">重置密码</el-button>
            <el-button
              size="small"
              type="danger"
              plain
              :disabled="row.username === authStore.user?.username"
              @click="handleDelete(row)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>

      <el-empty v-if="!loading && pagedUsers.length === 0" description="暂无用户" />
    </div>

    <el-table v-else :data="pagedUsers" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="username" label="用户名" min-width="150" />
      <el-table-column prop="role" label="角色" width="140">
        <template #default="{ row }">
          <el-select
            :model-value="row.role"
            size="small"
            :disabled="row.username === authStore.user?.username"
            @change="(val: string) => handleRoleChange(row, val)"
          >
            <el-option value="admin" label="管理员" />
            <el-option value="operator" label="操作员" />
            <el-option value="viewer" label="观察者" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="2FA" width="80" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.two_factor_enabled" size="small" type="success" effect="plain">已启用</el-tag>
          <span v-else class="text-secondary">-</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled"
            size="small"
            :disabled="row.username === authStore.user?.username"
            @change="handleToggle(row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="last_login_at" label="最后登录" width="170">
        <template #default="{ row }">
          {{ row.last_login_at ? new Date(row.last_login_at).toLocaleString() : '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="170">
        <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text type="primary" @click="openResetPassword(row)">重置密码</el-button>
          <el-button
            size="small" text type="danger"
            :disabled="row.username === authStore.user?.username"
            @click="handleDelete(row)"
          >删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="total > pageSize" class="pagination-container" style="margin-top: 12px">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        :layout="isMobile ? 'prev, pager, next' : 'total, sizes, prev, pager, next, jumper'"
      />
    </div>

    <el-dialog v-model="showCreateDialog" title="新建用户" width="400px" :fullscreen="dialogFullscreen">
      <el-form :model="createForm" :label-width="dialogFullscreen ? 'auto' : '80px'" :label-position="dialogFullscreen ? 'top' : 'right'">
        <el-form-item label="用户名">
          <el-input v-model="createForm.username" placeholder="3-32 位字母/数字/下划线" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="createForm.password" type="password" show-password placeholder="至少 6 位，含字母和数字" />
        </el-form-item>
        <el-form-item label="角色">
          <el-radio-group v-model="createForm.role">
            <el-radio value="admin">管理员</el-radio>
            <el-radio value="operator">操作员</el-radio>
            <el-radio value="viewer">观察者</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showResetPwdDialog" title="重置密码" width="400px" :fullscreen="dialogFullscreen">
      <el-form :model="resetPwdForm" :label-width="dialogFullscreen ? 'auto' : '80px'" :label-position="dialogFullscreen ? 'top' : 'right'">
        <el-form-item label="用户">
          <el-input :model-value="resetPwdForm.username" disabled />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="resetPwdForm.password" type="password" show-password placeholder="至少 6 位，含字母和数字" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showResetPwdDialog = false">取消</el-button>
        <el-button type="primary" @click="handleResetPassword">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.users-page {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;

  h2 { margin: 0; font-size: 20px; font-weight: 700; color: var(--el-text-color-primary); }

  .page-subtitle {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    display: block;
    margin-top: 2px;
  }
}

.header-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.user-card__actions > * {
  flex: 1 1 calc(50% - 4px);
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
}

.text-secondary {
  color: var(--el-text-color-secondary);
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
  .header-actions {
    flex-direction: column;
    align-items: stretch;
  }
  .header-actions :deep(.el-input) {
    width: 100% !important;
  }
}
</style>
