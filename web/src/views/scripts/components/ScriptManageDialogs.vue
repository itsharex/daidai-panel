<script setup lang="ts">
import { Upload } from '@element-plus/icons-vue'
import { computed } from 'vue'
import type { ScriptVersionRecord } from '../types'

const showCreateFileDialog = defineModel<boolean>('showCreateFileDialog', { required: true })
const showCreateDirDialog = defineModel<boolean>('showCreateDirDialog', { required: true })
const showRenameDialog = defineModel<boolean>('showRenameDialog', { required: true })
const showVersionDialog = defineModel<boolean>('showVersionDialog', { required: true })
const showUploadDialog = defineModel<boolean>('showUploadDialog', { required: true })

const newFileName = defineModel<string>('newFileName', { required: true })
const newFileParent = defineModel<string>('newFileParent', { required: true })
const newDirName = defineModel<string>('newDirName', { required: true })
const newDirParent = defineModel<string>('newDirParent', { required: true })
const renameTarget = defineModel<string>('renameTarget', { required: true })
const uploadDir = defineModel<string>('uploadDir', { required: true })

const props = defineProps<{
  isMobile: boolean
  allFolders: string[]
  versions: ScriptVersionRecord[]
  versionsLoading: boolean
  onCreateFile: () => void | Promise<void>
  onCreateDir: () => void | Promise<void>
  onRename: () => void | Promise<void>
  onRollback: (versionId: number) => void | Promise<void>
  onUploadFileChange: (file: any, files: any[]) => void
  onUploadSubmit: () => void | Promise<void>
}>()

const nestedFolders = computed(() => props.allFolders.filter(folder => folder))
</script>

<template>
  <el-dialog v-model="showCreateFileDialog" title="新建文件" :width="isMobile ? '90%' : '480px'" :fullscreen="isMobile">
    <el-form :label-width="isMobile ? 'auto' : '80px'" :label-position="isMobile ? 'top' : 'right'">
      <el-form-item label="上级目录">
        <el-select v-model="newFileParent" placeholder="根目录" clearable style="width: 100%">
          <el-option label="根目录" value="" />
          <el-option v-for="folder in nestedFolders" :key="folder" :label="folder" :value="folder" />
        </el-select>
      </el-form-item>
      <el-form-item label="文件名">
        <el-input v-model="newFileName" placeholder="如: script.py" @keyup.enter="onCreateFile" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showCreateFileDialog = false">取消</el-button>
      <el-button type="primary" @click="onCreateFile">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="showCreateDirDialog" title="新建目录" :width="isMobile ? '90%' : '480px'" :fullscreen="isMobile">
    <el-form :label-width="isMobile ? 'auto' : '80px'" :label-position="isMobile ? 'top' : 'right'">
      <el-form-item label="上级目录">
        <el-select v-model="newDirParent" placeholder="根目录" clearable style="width: 100%">
          <el-option label="根目录" value="" />
          <el-option v-for="folder in nestedFolders" :key="folder" :label="folder" :value="folder" />
        </el-select>
      </el-form-item>
      <el-form-item label="目录名">
        <el-input v-model="newDirName" placeholder="如: utils" @keyup.enter="onCreateDir" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showCreateDirDialog = false">取消</el-button>
      <el-button type="primary" @click="onCreateDir">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="showRenameDialog" title="重命名" :width="isMobile ? '90%' : '400px'" :fullscreen="isMobile">
    <el-input v-model="renameTarget" placeholder="新名称" @keyup.enter="onRename" />
    <template #footer>
      <el-button @click="showRenameDialog = false">取消</el-button>
      <el-button type="primary" @click="onRename">确定</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="showVersionDialog" title="版本历史" :width="isMobile ? '95%' : '600px'" :fullscreen="isMobile">
    <el-table :data="versions" v-loading="versionsLoading" max-height="400px">
      <el-table-column prop="version" label="版本" width="80" />
      <el-table-column prop="message" label="备注" />
      <el-table-column prop="content_length" label="大小" width="100">
        <template #default="{ row }">{{ (row.content_length / 1024).toFixed(1) }} KB</template>
      </el-table-column>
      <el-table-column prop="created_at" label="时间" width="180">
        <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text type="primary" @click="onRollback(row.id)">回滚</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>

  <el-dialog v-model="showUploadDialog" title="上传文件" :width="isMobile ? '90%' : '480px'" :fullscreen="isMobile" destroy-on-close>
    <el-form :label-width="isMobile ? 'auto' : '80px'" :label-position="isMobile ? 'top' : 'right'">
      <el-form-item label="目标目录">
        <el-select v-model="uploadDir" placeholder="根目录" clearable style="width: 100%">
          <el-option label="根目录" value="" />
          <el-option v-for="folder in nestedFolders" :key="folder" :label="folder" :value="folder" />
        </el-select>
      </el-form-item>
      <el-form-item label="选择文件">
        <el-upload
          :auto-upload="false"
          :show-file-list="true"
          multiple
          :on-change="onUploadFileChange"
          :on-remove="onUploadFileChange"
          drag
        >
          <el-icon :size="40"><Upload /></el-icon>
          <div>拖拽文件到此处或点击选择</div>
          <div class="el-upload__tip">支持一次选择多个脚本文件，单个文件大小不超过 10MB。</div>
        </el-upload>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showUploadDialog = false">取消</el-button>
      <el-button type="primary" @click="onUploadSubmit">上传</el-button>
    </template>
  </el-dialog>
</template>
