<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useResponsive } from '@/composables/useResponsive'

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  create: [items: { name: string; value: string }[]]
}>()

const batchText = ref('')
const { dialogFullscreen } = useResponsive()

function closeDialog() {
  emit('update:modelValue', false)
}

function handleCreate() {
  const text = batchText.value.trim()
  if (!text) {
    ElMessage.warning('请输入环境变量')
    return
  }

  const lines = text.split('\n').filter(line => line.trim())
  const items: { name: string; value: string }[] = []
  for (const line of lines) {
    const eqIndex = line.indexOf('=')
    if (eqIndex <= 0) {
      ElMessage.warning(`格式错误: ${line}，应为 NAME=VALUE`)
      return
    }
    items.push({
      name: line.substring(0, eqIndex).trim(),
      value: line.substring(eqIndex + 1).trim()
    })
  }

  emit('create', items)
}

watch(
  () => props.modelValue,
  (visible) => {
    if (!visible) {
      batchText.value = ''
    }
  }
)
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="批量添加环境变量"
    width="550px"
    :fullscreen="dialogFullscreen"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-alert type="info" :closable="false" style="margin-bottom: 12px">
      每行一个变量，格式: NAME=VALUE
    </el-alert>
    <el-input
      v-model="batchText"
      type="textarea"
      :rows="10"
      placeholder="API_KEY=your_key&#10;SECRET=your_secret&#10;TOKEN=your_token"
    />
    <template #footer>
      <el-button @click="closeDialog">取消</el-button>
      <el-button type="primary" @click="handleCreate">批量创建</el-button>
    </template>
  </el-dialog>
</template>
