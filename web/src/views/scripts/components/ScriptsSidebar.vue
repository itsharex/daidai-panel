<script setup lang="ts">
import { DocumentAdd, FolderAdd, Refresh, Upload, VideoPlay } from '@element-plus/icons-vue'
import ScriptTreeNode from './ScriptTreeNode.vue'
import type { TreeNode } from '../types'

defineProps<{
  isMobile: boolean
  mobileShowEditor: boolean
  treeLoading: boolean
  fileTree: TreeNode[]
  allowDrag: (draggingNode: any) => boolean
  allowDrop: (draggingNode: any, dropNode: any, type: string) => boolean
  onOpenCreateFile: () => void
  onOpenCreateDir: () => void
  onOpenUpload: () => void
  onOpenCodeRunner: () => void
  onRefresh: () => void | Promise<void>
  onNodeClick: (data: TreeNode) => void | Promise<void>
  onNodeDrop: (draggingNode: any, dropNode: any) => void | Promise<void>
  onOpenRename: (path: string) => void
  onDelete: (path: string, isDir: boolean) => void | Promise<void>
}>()
</script>

<template>
  <div class="scripts-sidebar" :class="{ mobile: isMobile }" v-show="!isMobile || !mobileShowEditor">
    <div class="sidebar-header">
      <span class="sidebar-title">脚本文件</span>
      <div class="sidebar-actions">
        <el-tooltip content="新建文件" placement="bottom">
          <el-button class="action-btn" circle @click="onOpenCreateFile">
            <el-icon :size="14"><DocumentAdd /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="新建目录" placement="bottom">
          <el-button class="action-btn" circle @click="onOpenCreateDir">
            <el-icon :size="14"><FolderAdd /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="上传文件" placement="bottom">
          <el-button class="action-btn" circle @click="onOpenUpload">
            <el-icon :size="14"><Upload /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="代码运行器" placement="bottom">
          <el-button class="action-btn" circle @click="onOpenCodeRunner">
            <el-icon :size="14"><VideoPlay /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="刷新" placement="bottom">
          <el-button class="action-btn" circle @click="onRefresh">
            <el-icon :size="14"><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </div>
    <div class="sidebar-tree" v-loading="treeLoading">
      <el-tree
        :data="fileTree"
        node-key="key"
        :props="{ children: 'children', label: 'title' }"
        :highlight-current="true"
        :expand-on-click-node="true"
        draggable
        :allow-drag="allowDrag"
        :allow-drop="allowDrop"
        @node-drop="onNodeDrop"
        @node-click="onNodeClick"
      >
        <template #default="{ data }">
          <ScriptTreeNode :data="data" :on-open-rename="onOpenRename" :on-delete="onDelete" />
        </template>
      </el-tree>
    </div>
  </div>
</template>

<style scoped lang="scss">
.scripts-sidebar {
  width: 300px;
  min-width: 300px;
  border-right: 1px solid var(--el-border-color-light);
  display: flex;
  flex-direction: column;
  background: var(--el-bg-color);
}

.sidebar-header {
  padding: 10px 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  flex-shrink: 0;

  .sidebar-title {
    font-weight: 600;
    font-size: 15px;
    white-space: nowrap;
  }

  .sidebar-actions {
    display: flex;
    align-items: center;
    gap: 2px;

    .action-btn {
      width: 28px;
      height: 28px;
      padding: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      border: 1px solid var(--el-border-color);
      background: var(--el-bg-color);
      transition: all 0.3s;

      &:hover {
        border-color: var(--el-color-primary);
        color: var(--el-color-primary);
        background: var(--el-color-primary-light-9);
        transform: translateY(-2px);
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
      }

      &:active {
        transform: translateY(0);
      }
    }
  }
}

.sidebar-tree {
  flex: 1;
  overflow-y: auto;
  padding: 10px;

  :deep(.el-tree-node__content) {
    height: 36px;
    font-size: 14px;
  }

  :deep(.el-tree-node__content:hover) {
    background: var(--el-fill-color-light);
  }

  :deep(.el-tree-node.is-current > .el-tree-node__content) {
    background: var(--el-color-primary-light-9);
    color: var(--el-color-primary);
  }

  :deep(.el-tree-node.is-drop-inner > .el-tree-node__content) {
    background: var(--el-color-primary-light-9);
    border-radius: 6px;
    outline: 2px dashed var(--el-color-primary);
    outline-offset: -2px;
  }

  :deep(.el-tree__drop-indicator) {
    display: none;
  }

  :deep(.el-tree-node.is-dragging > .el-tree-node__content) {
    opacity: 0.5;
  }
}

.scripts-sidebar.mobile {
  .sidebar-header {
    padding: 8px 10px;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 8px;

    .sidebar-actions {
      gap: 1px;
      width: 100%;
      justify-content: space-between;
    }
  }

  :deep(.tree-node) {
    .tree-node-actions {
      opacity: 1;
    }

    .file-ext-badge {
      opacity: 1;
    }
  }
}
</style>
