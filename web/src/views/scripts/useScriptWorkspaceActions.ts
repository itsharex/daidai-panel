import { ref, type ComputedRef, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { scriptApi } from '@/api/script'
import type { ScriptVersionRecord } from './types'

interface ScriptWorkspaceActionsOptions {
  selectedFile: Ref<string>
  fileContent: Ref<string>
  originalContent: Ref<string>
  isBinary: Ref<boolean>
  isEditing: Ref<boolean>
  hasChanges: ComputedRef<boolean>
  loadTree: () => Promise<void>
  loadFileContent: (path: string) => Promise<boolean>
}

export function useScriptWorkspaceActions({
  selectedFile,
  fileContent,
  originalContent,
  isBinary,
  isEditing,
  hasChanges,
  loadTree,
  loadFileContent
}: ScriptWorkspaceActionsOptions) {
  const router = useRouter()

  const saving = ref(false)
  const formatting = ref(false)

  const showCreateFileDialog = ref(false)
  const showCreateDirDialog = ref(false)
  const showRenameDialog = ref(false)
  const showVersionDialog = ref(false)
  const showUploadDialog = ref(false)

  const uploadDir = ref('')
  const uploadFileList = ref<File[]>([])

  const newFileName = ref('')
  const newFileParent = ref('')
  const newDirName = ref('')
  const newDirParent = ref('')
  const renameTarget = ref('')
  const renamePath = ref('')

  const versions = ref<ScriptVersionRecord[]>([])
  const versionsLoading = ref(false)

  function isActionCancelled(err: unknown) {
    return err === 'cancel' || err === 'close' || String(err) === 'cancel' || String(err) === 'close'
  }

  async function handleSave() {
    if (!selectedFile.value || isBinary.value) return
    saving.value = true
    try {
      let versionMessage = 'V1 初始版本'
      if (originalContent.value !== '') {
        try {
          const res = await scriptApi.listVersions(selectedFile.value)
          const versionCount = res.data?.length || 0
          versionMessage = `V${versionCount + 1} 更新`
        } catch {
          versionMessage = 'V2 更新'
        }
      }
      await scriptApi.saveContent(selectedFile.value, fileContent.value, versionMessage)
      originalContent.value = fileContent.value
      ElMessage.success('保存成功')
    } catch {
      ElMessage.error('保存失败')
    } finally {
      saving.value = false
    }
  }

  async function handleCreateFile() {
    if (!newFileName.value.trim()) return
    try {
      const fullPath = newFileParent.value
        ? `${newFileParent.value}/${newFileName.value.trim()}`
        : newFileName.value.trim()
      await scriptApi.saveContent(fullPath, '', 'V1 初始版本')
      ElMessage.success('创建成功')
      showCreateFileDialog.value = false
      newFileName.value = ''
      newFileParent.value = ''
      await loadTree()
      selectedFile.value = fullPath
      isEditing.value = true
      await loadFileContent(fullPath)
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '创建失败')
    }
  }

  async function handleCreateDir() {
    if (!newDirName.value.trim()) return
    try {
      const fullPath = newDirParent.value
        ? `${newDirParent.value}/${newDirName.value.trim()}`
        : newDirName.value.trim()
      await scriptApi.createDirectory(fullPath)
      ElMessage.success('创建成功')
      showCreateDirDialog.value = false
      newDirName.value = ''
      newDirParent.value = ''
      await loadTree()
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '创建失败')
    }
  }

  async function handleDelete(path: string, isDir = false) {
    try {
      await ElMessageBox.confirm(`确定要删除 ${path} 吗？${isDir ? '\n注意：将同时删除文件夹内所有文件！' : ''}`, '确认删除', { type: 'warning' })
      await scriptApi.delete(path, isDir ? 'directory' : 'file')
      ElMessage.success('删除成功')
      if (selectedFile.value === path || (isDir && selectedFile.value.startsWith(path + '/'))) {
        selectedFile.value = ''
        fileContent.value = ''
        originalContent.value = ''
      }
      await loadTree()
    } catch (err: any) {
      if (isActionCancelled(err)) return
      ElMessage.error(err?.response?.data?.error || err?.message || '删除失败')
    }
  }

  async function handleRename() {
    if (!renameTarget.value.trim()) return
    try {
      const res = await scriptApi.rename(renamePath.value, renameTarget.value.trim())
      ElMessage.success('重命名成功')
      showRenameDialog.value = false
      if (selectedFile.value === renamePath.value) {
        selectedFile.value = res.new_path || renameTarget.value.trim()
      }
      await loadTree()
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '重命名失败')
    }
  }

  function openRename(path: string) {
    renamePath.value = path
    renameTarget.value = path.split('/').pop() || path
    showRenameDialog.value = true
  }

  function openUploadDialog() {
    showUploadDialog.value = true
    uploadDir.value = ''
    uploadFileList.value = []
  }

  async function handleUpload(files: File[]) {
    const formData = new FormData()
    for (const file of files) {
      formData.append('file', file)
    }
    if (uploadDir.value) {
      formData.append('dir', uploadDir.value)
    }
    try {
      const res = await scriptApi.upload(formData)
      const uploadedPaths = Array.isArray(res.paths) && res.paths.length > 0
        ? res.paths
        : files.map((file) => (uploadDir.value ? `${uploadDir.value}/${file.name}` : file.name))

      ElMessage.success(uploadedPaths.length > 1 ? `成功上传 ${uploadedPaths.length} 个文件` : '上传成功')
      showUploadDialog.value = false
      uploadDir.value = ''
      uploadFileList.value = []
      await loadTree()

      if (uploadedPaths.length === 1) {
        const targetPath = uploadedPaths[0]
        if (!targetPath) return false
        try {
          await ElMessageBox.confirm('是否将此脚本添加到定时任务？', '提示', {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'info'
          })
          navigateToTaskWithScript(targetPath)
        } catch {
          // cancelled
        }
      }
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '上传失败')
    }
    return false
  }

  function handleUploadFileChange(_file: { raw?: File } | undefined, files: Array<{ raw?: File }>) {
    uploadFileList.value = files
      .map((item) => item.raw)
      .filter((file): file is File => Boolean(file))
  }

  async function handleUploadSubmit() {
    if (uploadFileList.value.length === 0) {
      ElMessage.warning('请至少选择一个文件')
      return
    }
    await handleUpload(uploadFileList.value)
  }

  function navigateToTaskWithScript(filePath: string) {
    const fileName = filePath.split('/').pop() || filePath
    const taskName = fileName.replace(/\.[^/.]+$/, '')
    const command = `task ${filePath}`
    void router.push({
      path: '/tasks',
      query: { autoCreate: '1', name: taskName, command }
    })
  }

  function handleAddToTask() {
    if (!selectedFile.value) return
    navigateToTaskWithScript(selectedFile.value)
  }

  async function loadVersions() {
    if (!selectedFile.value) return
    versionsLoading.value = true
    showVersionDialog.value = true
    try {
      const res = await scriptApi.listVersions(selectedFile.value)
      versions.value = res.data || []
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '加载版本历史失败')
    } finally {
      versionsLoading.value = false
    }
  }

  async function handleRollback(versionId: number) {
    try {
      await ElMessageBox.confirm('确定要回滚到此版本吗？', '确认回滚', { type: 'warning' })
      await scriptApi.rollback(versionId)
      ElMessage.success('回滚成功')
      showVersionDialog.value = false
      await loadFileContent(selectedFile.value)
    } catch {
      // cancelled
    }
  }

  async function handleFormat() {
    if (!selectedFile.value || isBinary.value) return
    const langMap: Record<string, string> = {
      py: 'python',
      sh: 'shell',
      json: 'json'
    }
    const ext = selectedFile.value.split('.').pop()?.toLowerCase() || ''
    const lang = langMap[ext]
    if (!lang) {
      ElMessage.warning('该文件类型不支持格式化')
      return
    }
    formatting.value = true
    try {
      const res = await scriptApi.format({ content: fileContent.value, language: lang })
      if (res.data?.content) {
        fileContent.value = res.data.content
        ElMessage.success('格式化完成')
      }
    } catch {
      ElMessage.error('格式化失败')
    } finally {
      formatting.value = false
    }
  }

  function getFileName(path: string) {
    return path.split('/').pop() || path
  }

  function handleDownload() {
    if (!selectedFile.value) return
    const blob = new Blob([fileContent.value], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = getFileName(selectedFile.value)
    a.click()
    URL.revokeObjectURL(url)
  }

  function handleKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault()
      if (selectedFile.value && !isBinary.value && hasChanges.value) {
        void handleSave()
      }
    }
  }

  return {
    saving,
    formatting,
    showCreateFileDialog,
    showCreateDirDialog,
    showRenameDialog,
    showVersionDialog,
    showUploadDialog,
    uploadDir,
    newFileName,
    newFileParent,
    newDirName,
    newDirParent,
    renameTarget,
    versions,
    versionsLoading,
    handleSave,
    handleCreateFile,
    handleCreateDir,
    handleDelete,
    handleRename,
    openRename,
    openUploadDialog,
    handleUploadFileChange,
    handleUploadSubmit,
    handleAddToTask,
    loadVersions,
    handleRollback,
    handleFormat,
    handleDownload,
    handleKeyDown
  }
}
