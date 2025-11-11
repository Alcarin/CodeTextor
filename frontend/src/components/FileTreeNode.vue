<!--
  File: FileTreeNode.vue
  Purpose: Render directory and file nodes that expand into outlines.
  Author: CodeTextor project
  Notes: Recursively renders the directory tree and loads outlines on demand.
-->

<script setup lang="ts">
import { computed } from 'vue'
import OutlineTreeNode from './OutlineTreeNode.vue'
import type { FileTreeNode, OutlineNode } from '../types'

defineOptions({ name: 'FileTreeNode' })

interface Props {
  node: FileTreeNode
  level: number
  fetchOutline: (node: FileTreeNode) => Promise<void>
  isOutlineNodeExpanded: (id: string) => boolean
  toggleOutlineNode: (id: string) => void
  onNodeClick?: (filePath: string, node: OutlineNode) => void
  selectedNodeId?: string
}

const props = defineProps<Props>()

const toggleNode = async () => {
  props.node.expanded = !props.node.expanded
  if (!props.node.isDirectory && props.node.expanded && props.node.outlineStatus !== 'ready') {
    await props.fetchOutline(props.node)
  }
}

const statusLabel = computed(() => {
  switch (props.node.outlineStatus) {
    case 'loading':
      return 'Caricamento...'
    case 'error':
      return props.node.outlineError || 'Errore durante il caricamento'
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
      :selected-node-id="selectedNodeId"
    />

    <div v-if="!node.isDirectory && node.outlineStatus === 'ready'" class="outline-section">
      <div
        v-if="!node.outlineNodes || node.outlineNodes.length === 0"
        class="muted-helper"
        data-testid="outline-empty"
      >
        Nessun simbolo disponibile in questo file.
      </div>
      <OutlineTreeNode
        v-for="outlineNode in node.outlineNodes || []"
        :key="outlineNode.id"
        :node="outlineNode"
        :file-path="node.path"
        :level="level + 1"
        :expanded="isOutlineNodeExpanded(outlineNode.id)"
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
