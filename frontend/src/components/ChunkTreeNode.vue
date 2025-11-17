<!--
  File: ChunkTreeNode.vue
  Purpose: Render individual chunk nodes with metadata in the tree.
  Author: CodeTextor project
  Notes: Similar to OutlineTreeNode but for semantic chunks.
-->

<script setup lang="ts">
import { computed } from 'vue'
import type { Chunk } from '../types'

defineOptions({ name: 'ChunkTreeNode' })

interface Props {
  chunk: Chunk
  filePath: string
  level: number
  selectedNodeId?: string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  click: [filePath: string, chunk: Chunk]
}>()

const isSelected = computed(() => props.selectedNodeId === props.chunk.id)

const handleClick = () => {
  emit('click', props.filePath, props.chunk)
}

const getChunkIcon = (kind: string): string => {
  const iconMap: Record<string, string> = {
    'function': '𝑓',
    'method': 'ⓜ',
    'class': '©',
    'struct': '◫',
    'interface': 'ⓘ',
    'type': '𝕋',
    'const': '𝕂',
    'variable': '𝕧',
    'import': '⇐',
    'package': '📦',
    'module': '📦',
  };
  return iconMap[kind?.toLowerCase()] || '•';
};

const formatTokenCount = (count?: number): string => {
  if (!count) return '';
  if (count < 1000) return `${count}t`;
  return `${(count / 1000).toFixed(1)}kt`;
};
</script>

<template>
  <div
    class="chunk-node"
    :class="{ selected: isSelected }"
    :style="{ paddingLeft: (level * 1.25) + 'rem' }"
    @click="handleClick"
  >
    <span class="chunk-icon">{{ getChunkIcon(chunk.symbolKind || '') }}</span>
    <span class="chunk-name">{{ chunk.symbolName || 'unnamed' }}</span>
    <span v-if="chunk.symbolKind" class="chunk-kind">{{ chunk.symbolKind }}</span>
    <span v-if="chunk.tokenCount" class="chunk-tokens">{{ formatTokenCount(chunk.tokenCount) }}</span>
    <span class="chunk-location">L{{ chunk.lineStart }}-{{ chunk.lineEnd }}</span>
  </div>
</template>

<style scoped>
.chunk-node {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.5rem;
  cursor: pointer;
  border-radius: 4px;
  transition: background-color 0.15s ease;
  font-size: 0.9rem;
  line-height: 1.4;
}

.chunk-node:hover {
  background-color: #2a2a2d;
}

.chunk-node.selected {
  background-color: #094771;
}

.chunk-icon {
  font-size: 1.1rem;
  color: #4a9eff;
  width: 18px;
  text-align: center;
  flex-shrink: 0;
}

.chunk-name {
  font-weight: 500;
  color: #dcdcdc;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chunk-kind {
  font-size: 0.7rem;
  color: #9a9da3;
  background: #3e3e42;
  padding: 0.125rem 0.4rem;
  border-radius: 3px;
  flex-shrink: 0;
}

.chunk-tokens {
  font-size: 0.7rem;
  color: #a0c566;
  font-weight: 500;
  flex-shrink: 0;
}

.chunk-location {
  font-size: 0.7rem;
  color: #6a6d73;
  flex-shrink: 0;
}
</style>
