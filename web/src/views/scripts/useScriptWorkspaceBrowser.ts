import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { scriptApi } from '@/api/script'
import { useResponsive } from '@/composables/useResponsive'
import type { TreeNode } from './types'

type ScriptBrowserState = {
  selectedFile: string
  fileContent: string
  originalContent: string
  isBinary: boolean
  isEditing: boolean
  mobileShowEditor: boolean
}

export function useScriptWorkspaceBrowser() {
  const { isMobile } = useResponsive()
  const mobileShowEditor = ref(false)

  const fileTree = ref<TreeNode[]>([])
  const selectedFile = ref('')
  const fileContent = ref('')
  const originalContent = ref('')
  const isBinary = ref(false)
  const loading = ref(false)
  const treeLoading = ref(false)
  const isEditing = ref(false)

  const editorLanguage = computed(() => {
    if (!selectedFile.value) return 'javascript'
    const ext = selectedFile.value.split('.').pop()?.toLowerCase()
    const langMap: Record<string, string> = {
      js: 'javascript',
      ts: 'typescript',
      py: 'python',
      sh: 'shell',
      go: 'go',
      json: 'json',
      yaml: 'yaml',
      yml: 'yaml',
      md: 'markdown',
      html: 'html',
      css: 'css',
      xml: 'xml'
    }
    return langMap[ext || ''] || 'plaintext'
  })

  const hasChanges = computed(() => fileContent.value !== originalContent.value)

  const allFolders = computed(() => {
    const folders: string[] = ['']
    const collectFolders = (nodes: TreeNode[], prefix = '') => {
      for (const node of nodes) {
        if (!node.isLeaf) {
          const path = prefix ? `${prefix}/${node.title}` : node.title
          folders.push(path)
          if (node.children) {
            collectFolders(node.children, path)
          }
        }
      }
    }
    collectFolders(fileTree.value)
    return folders
  })

  watch(isMobile, (mobile) => {
    if (!mobile) {
      mobileShowEditor.value = false
    }
  })

  function snapshotState(): ScriptBrowserState {
    return {
      selectedFile: selectedFile.value,
      fileContent: fileContent.value,
      originalContent: originalContent.value,
      isBinary: isBinary.value,
      isEditing: isEditing.value,
      mobileShowEditor: mobileShowEditor.value
    }
  }

  function restoreState(state: ScriptBrowserState) {
    selectedFile.value = state.selectedFile
    fileContent.value = state.fileContent
    originalContent.value = state.originalContent
    isBinary.value = state.isBinary
    isEditing.value = state.isEditing
    mobileShowEditor.value = state.mobileShowEditor
  }

  async function loadTree() {
    treeLoading.value = true
    try {
      const res = await scriptApi.tree()
      fileTree.value = res.data || []
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '加载文件树失败')
    } finally {
      treeLoading.value = false
    }
  }

  async function loadFileContent(path: string) {
    loading.value = true
    try {
      const res = await scriptApi.getContent(path)
      isBinary.value = res.data.is_binary ?? res.data.binary ?? false
      fileContent.value = res.data.content
      originalContent.value = res.data.content
      return true
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.error || err?.message || '加载文件内容失败')
      return false
    } finally {
      loading.value = false
    }
  }

  async function confirmOpenFile(path: string, skipUnsavedCheck = false) {
    if (skipUnsavedCheck || !hasChanges.value || path === selectedFile.value) {
      return true
    }

    try {
      await ElMessageBox.confirm('当前文件有未保存的修改，是否放弃？', '提示', {
        confirmButtonText: '放弃',
        cancelButtonText: '取消',
        type: 'warning'
      })
      return true
    } catch {
      return false
    }
  }

  async function openFile(path: string, options: { skipUnsavedCheck?: boolean } = {}) {
    const normalizedPath = path.trim()
    if (!normalizedPath) {
      return false
    }

    if (normalizedPath === selectedFile.value) {
      mobileShowEditor.value = true
      return true
    }

    const canProceed = await confirmOpenFile(normalizedPath, options.skipUnsavedCheck ?? false)
    if (!canProceed) {
      return false
    }

    const previousState = snapshotState()
    selectedFile.value = normalizedPath
    isEditing.value = false
    const loaded = await loadFileContent(normalizedPath)
    if (!loaded) {
      restoreState(previousState)
      return false
    }

    mobileShowEditor.value = true
    return true
  }

  async function handleNodeClick(data: TreeNode) {
    if (!data.isLeaf) return
    await openFile(data.key)
  }

  function allowDrag(draggingNode: any) {
    return draggingNode.data.isLeaf
  }

  function allowDrop(draggingNode: any, dropNode: any, type: string) {
    if (type === 'inner') {
      return !dropNode.data.isLeaf
    }
    if (type === 'before' || type === 'after') {
      return dropNode.level === 1
    }
    return false
  }

  async function handleNodeDrop(draggingNode: any, dropNode: any, dropType: string) {
    const sourcePath = draggingNode.data.key
    const targetDir = dropType === 'inner' ? dropNode.data.key : ''
    try {
      await scriptApi.move(sourcePath, targetDir)
      ElMessage.success('移动成功')
      if (selectedFile.value === sourcePath) {
        const fileName = sourcePath.split('/').pop() || sourcePath
        selectedFile.value = targetDir ? `${targetDir}/${fileName}` : fileName
      }
      await loadTree()
    } catch {
      ElMessage.error('移动失败')
      await loadTree()
    }
  }

  function handleMobileBack() {
    mobileShowEditor.value = false
  }

  return {
    isMobile,
    mobileShowEditor,
    fileTree,
    selectedFile,
    fileContent,
    originalContent,
    isBinary,
    loading,
    treeLoading,
    isEditing,
    editorLanguage,
    hasChanges,
    allFolders,
    loadTree,
    loadFileContent,
    openFile,
    handleNodeClick,
    allowDrag,
    allowDrop,
    handleNodeDrop,
    handleMobileBack
  }
}
