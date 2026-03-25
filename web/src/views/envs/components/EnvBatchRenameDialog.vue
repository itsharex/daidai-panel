<script setup lang="ts">
import { ref, watch } from 'vue'
import { useResponsive } from '@/composables/useResponsive'

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: [payload: { search: string; replace: string }]
}>()

const searchText = ref('')
const replaceText = ref('')
const { dialogFullscreen } = useResponsive()

function closeDialog() {
  emit('update:modelValue', false)
}

function handleConfirm() {
  emit('confirm', {
    search: searchText.value,
    replace: replaceText.value
  })
}

watch(
  () => props.modelValue,
  (visible) => {
    if (!visible) {
      searchText.value = ''
      replaceText.value = ''
    }
  }
)
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="批量修改变量名"
    width="420px"
    :fullscreen="dialogFullscreen"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-form :label-width="dialogFullscreen ? 'auto' : '80px'" :label-position="dialogFullscreen ? 'top' : 'right'">
      <el-form-item label="查找内容">
        <el-input
          v-model="searchText"
          clearable
          placeholder="例如 COOKIE"
          @keyup.enter="handleConfirm"
        />
      </el-form-item>
      <el-form-item label="替换为">
        <el-input
          v-model="replaceText"
          clearable
          placeholder="例如 TOKEN，留空则删除匹配内容"
          @keyup.enter="handleConfirm"
        />
      </el-form-item>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="仅会修改已勾选环境变量的名称，变量值和备注不会变化。"
      />
    </el-form>
    <template #footer>
      <el-button @click="closeDialog">取消</el-button>
      <el-button type="primary" @click="handleConfirm">确定</el-button>
    </template>
  </el-dialog>
</template>
