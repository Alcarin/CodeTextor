<!--
  File: FileTreeNode.vue
  Purpose: Render directory and file nodes that expand into outlines.
  Author: CodeTextor project
  Notes: Recursively renders the directory tree and loads outlines on demand.
-->

<script setup lang="ts">
import { computed } from 'vue'
import OutlineTreeNode from './OutlineTreeNode.vue'
import ChunkTreeNode from './ChunkTreeNode.vue'
import type { FileTreeNode, OutlineNode, Chunk } from '../types'

defineOptions({ name: 'FileTreeNode' })

interface Props {
  node: FileTreeNode
  level: number
  fetchOutline?: (node: FileTreeNode) => Promise<void>
  isOutlineNodeExpanded?: (id: string) => boolean
  toggleOutlineNode?: (id: string) => void
  onNodeClick?: (filePath: string, node: OutlineNode) => void
  onChunkClick?: (filePath: string, chunk: Chunk) => void
  onFileClick?: (node: FileTreeNode) => void
  selectedNodeId?: string
  simpleMode?: boolean
  chunksMode?: boolean
}

const props = defineProps<Props>()

const toggleNode = async () => {
  // In simple mode, just call onFileClick for files
  if (props.simpleMode && !props.node.isDirectory && props.onFileClick) {
    props.onFileClick(props.node)
    return
  }

  props.node.expanded = !props.node.expanded
  if (!props.node.isDirectory && props.node.expanded && props.node.outlineStatus !== 'ready' && props.fetchOutline) {
    await props.fetchOutline(props.node)
  }
}

const statusLabel = computed(() => {
  switch (props.node.outlineStatus) {
    case 'loading':
      return 'Loading...'
    case 'error':
      return props.node.outlineError || 'Error during loading'
    default:
      return ''
  }
})
</script>

<template>
  <div
    class="file-tree-row"
    :style="{ paddingLeft: (level * 1.25) + 'rem' }"
  >
    <button
      class="file-tree-toggle"
      :class="{ directory: node.isDirectory, file: !node.isDirectory }"
      @click="toggleNode"
      :aria-expanded="node.expanded"
      :aria-label="node.isDirectory ? 'Espandi cartella' : 'Mostra outline'"
      data-testid="file-tree-toggle"
    >
      <span v-if="node.isDirectory">{{ node.expanded ? '📂' : '📁' }}</span>
      <span v-else>{{ node.outlineStatus === 'ready' ? '📄' : '📄' }}</span>
      <span class="file-tree-label">{{ node.name }}</span>
    </button>
    <span v-if="statusLabel" class="file-tree-status">{{ statusLabel }}</span>
  </div>

  <div v-if="node.expanded">
    <FileTreeNode
      v-for="child in node.children"
      :key="child.path"
      :node="child"
      :level="level + 1"
      :fetchOutline="fetchOutline"
      :is-outline-node-expanded="isOutlineNodeExpanded"
      :toggle-outline-node="toggleOutlineNode"
      :on-node-click="onNodeClick"
      :on-chunk-click="onChunkClick"
      :on-file-click="onFileClick"
      :selected-node-id="selectedNodeId"
      :simple-mode="simpleMode"
      :chunks-mode="chunksMode"
    />

    <!-- Render chunks in chunks mode -->
    <div v-if="!node.isDirectory && node.outlineStatus === 'ready' && chunksMode" class="outline-section">
      <div
        v-if="!node.chunks || node.chunks.length === 0"
        class="muted-helper"
        data-testid="chunks-empty"
      >
        No chunks available for this file.
      </div>
      <ChunkTreeNode
        v-for="chunk in node.chunks || []"
        :key="chunk.id"
        :chunk="chunk"
        :file-path="node.path"
        :level="level + 1"
        :selected-node-id="selectedNodeId"
        @click="onChunkClick"
      />
    </div>

    <!-- Render outline nodes in normal mode -->
    <div v-if="!node.isDirectory && node.outlineStatus === 'ready' && !simpleMode && !chunksMode" class="outline-section">
      <div
        v-if="!node.outlineNodes || node.outlineNodes.length === 0"
        class="muted-helper"
        data-testid="outline-empty"
      >
        No symbols available in this file.
      </div>
      <OutlineTreeNode
        v-for="outlineNode in node.outlineNodes || []"
        :key="outlineNode.id"
        :node="outlineNode"
        :file-path="node.path"
        :level="level + 1"
        :expanded="isOutlineNodeExpanded?.(outlineNode.id) ?? false"
        :isExpanded="isOutlineNodeExpanded"
        :selected-node-id="selectedNodeId"
        @toggle="toggleOutlineNode"
        @click="onNodeClick"
      />
    </div>
  </div>
</template>

<style scoped>
.file-tree-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.file-tree-toggle {
  display: flex;
  align-items: center;
  justify-content: flex-start; /* keep file names left-aligned */
  gap: 0.5rem;
  background: none;
  border: none;
  color: #d4d4d4;
  padding: 0.25rem;
  font: inherit;
  cursor: pointer;
  text-align: left;
}

.file-tree-toggle:hover {
  color: #ffffff;
}

.file-tree-label {
  flex: 1;
  text-align: left;
  font-weight: 500;
}

.file-tree-status {
  font-size: 0.8rem;
  color: #9a9da3;
}

.outline-section {
  margin-top: 0.25rem;
}
</style>
