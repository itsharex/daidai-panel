<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useResponsive } from '@/composables/useResponsive'

type EnvFormModel = {
  id: number
  name: string
  value: string
  remarks: string
  group: string
}

const props = withDefaults(defineProps<{
  modelValue: boolean
  mode: 'create' | 'edit'
  initialData?: EnvFormModel | null
}>(), {
  initialData: null
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [value: EnvFormModel]
}>()

function createEmptyForm(): EnvFormModel {
  return { id: 0, name: '', value: '', remarks: '', group: '' }
}

const form = ref<EnvFormModel>(createEmptyForm())
const { dialogFullscreen } = useResponsive()

const dialogTitle = computed(() => props.mode === 'create' ? '新建环境变量' : '编辑环境变量')
const submitText = computed(() => props.mode === 'create' ? '创建' : '保存')

function syncForm() {
  form.value = {
    ...createEmptyForm(),
    ...(props.initialData ?? {})
  }
}

function closeDialog() {
  emit('update:modelValue', false)
}

function handleSave() {
  const payload: EnvFormModel = {
    id: form.value.id,
    name: form.value.name.trim(),
    value: form.value.value,
    remarks: form.value.remarks.trim(),
    group: form.value.group.trim()
  }
  if (!payload.name) {
    ElMessage.warning('变量名不能为空')
    return
  }
  emit('save', payload)
}

watch(
  () => [props.modelValue, props.initialData, props.mode],
  ([visible]) => {
    if (visible) {
      syncForm()
    }
  },
  { immediate: true }
)
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    :title="dialogTitle"
    width="500px"
    :fullscreen="dialogFullscreen"
    :close-on-click-modal="false"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-form :model="form" :label-width="dialogFullscreen ? 'auto' : '80px'" :label-position="dialogFullscreen ? 'top' : 'right'">
      <el-form-item label="变量名">
        <el-input v-model="form.name" placeholder="变量名 (如: API_KEY)" />
      </el-form-item>
      <el-form-item label="值">
        <el-input v-model="form.value" type="textarea" :rows="3" placeholder="变量值" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.remarks" placeholder="备注说明" />
      </el-form-item>
      <el-form-item label="分组">
        <el-input v-model="form.group" placeholder="分组 (可选)" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="closeDialog">取消</el-button>
      <el-button type="primary" @click="handleSave">{{ submitText }}</el-button>
    </template>
  </el-dialog>
</template>
