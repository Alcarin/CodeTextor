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
  level: number;
  expanded: boolean;
}

const props = defineProps<Props>();

// Emits
const emit = defineEmits<{
  toggle: [nodeId: string];
}>();

// Computed
const hasChildren = computed(() => props.node.children && props.node.children.length > 0);

/**
 * Handles node toggle click.
 */
const handleToggle = () => {
  emit('toggle', props.node.id);
};

/**
 * Gets icon for node kind.
 * @param kind - Node kind string
 * @returns Icon character
 */
const getKindIcon = (kind: string): string => {
  const icons: Record<string, string> = {
    'class': '🔷',
    'function': '🔹',
    'method': '🔸',
    'interface': '📐',
    'variable': '📌',
    'const': '🔒',
    'type': '🏷️'
  };
  return icons[kind] || '📄';
};
</script>

<template>
  <div class="tree-node">
    <div
      class="node-header"
      :style="{ paddingLeft: (level * 1.5) + 'rem' }"
      @click="handleToggle"
    >
      <span v-if="hasChildren" class="node-toggle">
        {{ expanded ? '▼' : '▶' }}
      </span>
      <span v-else class="node-toggle-placeholder"></span>
      <span class="node-icon">{{ getKindIcon(node.kind) }}</span>
      <span class="node-name">{{ node.name }}</span>
      <span class="node-kind">{{ node.kind }}</span>
      <span class="node-lines">L{{ node.startLine }}-{{ node.endLine }}</span>
    </div>
    <div v-if="expanded && hasChildren" class="node-children">
      <OutlineTreeNode
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :level="level + 1"
        :expanded="expanded"
        @toggle="$emit('toggle', $event)"
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
}

.node-header:hover {
  background: #2d2d30;
}

.node-toggle {
  width: 16px;
  color: #858585;
  font-size: 0.75rem;
}

.node-toggle-placeholder {
  width: 16px;
}

.node-icon {
  font-size: 1rem;
}

.node-name {
  flex: 1;
  color: #d4d4d4;
  font-weight: 500;
}

.node-kind {
  padding: 0.125rem 0.5rem;
  background: #007acc;
  border-radius: 3px;
  color: white;
  font-size: 0.75rem;
}

.node-lines {
  color: #858585;
  font-size: 0.85rem;
  font-family: 'Courier New', monospace;
}
</style>
