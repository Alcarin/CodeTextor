<!--
  File: components/OutlineTreeNode.vue
  Purpose: Recursive tree node component for outline display.
  Author: CodeTextor project
  Notes: Displays a single node with children in hierarchical tree structure.
-->

<script setup lang="ts">
import { computed } from 'vue';
import type { OutlineNode } from '../types';

// Props
interface Props {
  node: OutlineNode;
  filePath?: string;
  level: number;
  expanded: boolean;
  isExpanded?: (nodeId: string) => boolean;
  selectedNodeId?: string;
}

const props = defineProps<Props>();

// Computed
const isSelected = computed(() => props.selectedNodeId === props.node.id);

// Emits
const emit = defineEmits<{
  toggle: [nodeId: string];
  click: [filePath: string, node: OutlineNode];
}>();

// Computed
const hasChildren = computed(() => props.node.children && props.node.children.length > 0);

/**
 * Handles node toggle click.
 */
const handleToggle = (event: Event) => {
  event.stopPropagation();
  emit('toggle', props.node.id);
};

/**
 * Handles node click to show content.
 */
const handleClick = () => {
  if (props.filePath) {
    emit('click', props.filePath, props.node);
  }
};

/**
 * Gets icon for node kind.
 * @param kind - Node kind string
 * @returns Icon character
 */
const getKindIcon = (kind: string): string => {
  const icons: Record<string, string> = {
    // Programming constructs
    'class': '🔷',
    'function': '🔹',
    'method': '🔸',
    'interface': '📐',
    'variable': '📌',
    'const': '🔒',
    'type': '🏷️',
    // HTML/Vue elements
    'element': '🏷️',
    'script': '📜',
    'style': '🎨',
    // Markdown
    'heading': '📑',
    'code_block': '💻',
    'link': '🔗',
    // CSS
    'rule': '🎯',
    'media': '📱',
    'keyframes': '🎬'
  };
  return icons[kind] || '📄';
};
</script>

<template>
  <div class="tree-node">
    <div
      :class="['node-header', { selected: isSelected }]"
      :style="{ paddingLeft: (level * 1.5) + 'rem' }"
      @click="handleClick"
    >
      <span v-if="hasChildren" class="node-toggle" @click="handleToggle">
        {{ expanded ? '▼' : '▶' }}
      </span>
      <span v-else class="node-toggle-placeholder"></span>
      <span class="node-icon">{{ getKindIcon(node.kind) }}</span>
      <span class="node-name">{{ node.name }}</span>
      <div class="node-meta">
        <span class="node-kind">{{ node.kind }}</span>
        <span class="node-lines">L{{ node.startLine }}-{{ node.endLine }}</span>
      </div>
    </div>
    <div v-if="expanded && hasChildren" class="node-children">
      <OutlineTreeNode
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :file-path="filePath"
        :level="level + 1"
        :expanded="isExpanded ? isExpanded(child.id) : false"
        :isExpanded="isExpanded"
        :selected-node-id="selectedNodeId"
        @toggle="$emit('toggle', $event)"
        @click="(fp: string, n: OutlineNode) => $emit('click', fp, n)"
      />
    </div>
  </div>
</template>

<style scoped>
.tree-node {
  margin-bottom: 0.25rem;
}

.node-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.2s ease;
  min-width: fit-content;
}

.node-header:hover {
  background: #2d2d30;
}

.node-header.selected {
  background: #094771 !important;
  border-left: 3px solid #007acc;
}

.node-toggle {
  width: 16px;
  color: #858585;
  font-size: 0.75rem;
  flex-shrink: 0;
  cursor: pointer;
}

.node-toggle-placeholder {
  width: 16px;
  flex-shrink: 0;
}

.node-icon {
  font-size: 1rem;
  flex-shrink: 0;
}

.node-name {
  color: #d4d4d4;
  font-weight: 500;
  text-align: left;
  white-space: nowrap;
  flex-shrink: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.node-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.node-kind {
  padding: 0.125rem 0.5rem;
  background: #007acc;
  border-radius: 3px;
  color: white;
  font-size: 0.75rem;
  white-space: nowrap;
}

.node-lines {
  color: #858585;
  font-size: 0.85rem;
  font-family: 'Courier New', monospace;
  white-space: nowrap;
}
</style>
