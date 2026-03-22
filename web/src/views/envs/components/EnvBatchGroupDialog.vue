<script setup lang="ts">
import { ref, watch } from 'vue'
import { useResponsive } from '@/composables/useResponsive'

const props = defineProps<{
  modelValue: boolean
  groups: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: [group: string]
}>()

const batchGroupName = ref('')
const { dialogFullscreen } = useResponsive()

function closeDialog() {
  emit('update:modelValue', false)
}

function applyBatchGroupName(group: string) {
  batchGroupName.value = group
}

function handleConfirm() {
  emit('confirm', batchGroupName.value.trim())
}

watch(
  () => props.modelValue,
  (visible) => {
    if (!visible) {
      batchGroupName.value = ''
    }
  }
)
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="批量设置分组"
    width="400px"
    :fullscreen="dialogFullscreen"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-form :label-width="dialogFullscreen ? 'auto' : '80px'" :label-position="dialogFullscreen ? 'top' : 'right'">
      <el-form-item label="分组名称">
        <el-input
          v-model="batchGroupName"
          clearable
          placeholder="输入新分组名，或点击下方已有分组"
          @keyup.enter="handleConfirm"
        />
      </el-form-item>
      <el-form-item v-if="groups.length > 0" label="已有分组">
        <div class="batch-group-options">
          <el-tag
            v-for="group in groups"
            :key="group"
            class="batch-group-tag"
            effect="plain"
            @click="applyBatchGroupName(group)"
          >
            {{ group }}
          </el-tag>
        </div>
      </el-form-item>
      <el-alert type="info" :closable="false" show-icon>
        留空将清除选中变量的分组
      </el-alert>
    </el-form>
    <template #footer>
      <el-button @click="closeDialog">取消</el-button>
      <el-button type="primary" @click="handleConfirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<style scoped lang="scss">
.batch-group-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.batch-group-tag {
  cursor: pointer;
}
</style>
