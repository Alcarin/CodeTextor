<!--
  File: components/OutlineContentViewer.vue
  Purpose: Displays file content with optional highlighted outline sections.
  Author: CodeTextor project
-->

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue';
import { backend } from '../api/backend';
import type { OutlineNode } from '../types';

interface Props {
  projectId: string;
  filePath: string;
  selectedNode?: OutlineNode | null;
  fileVersion?: number;
}

const props = defineProps<Props>();

const fileContent = ref<string>('');
const isLoading = ref<boolean>(false);
const error = ref<string>('');
const highlightedLines = ref<{ start: number; end: number } | null>(null);
const codeContainerRef = ref<HTMLElement | null>(null);

const loadFileContent = async () => {
  if (!props.projectId || !props.filePath) {
    fileContent.value = '';
    return;
  }

  isLoading.value = true;
  error.value = '';

  try {
    fileContent.value = await backend.readFileContent(props.projectId, props.filePath);
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Error loading file';
    fileContent.value = '';
  } finally {
    isLoading.value = false;
  }
};

watch(() => props.filePath, () => {
  loadFileContent();
}, { immediate: true });

watch(() => props.fileVersion, (newValue, oldValue) => {
  if (newValue !== undefined && newValue !== oldValue) {
    loadFileContent();
  }
});

const scrollToHighlighted = () => {
  const container = codeContainerRef.value;
  if (!container) return;
  const highlighted = container.querySelector('.line-highlighted');
  if (highlighted && highlighted instanceof HTMLElement) {
    highlighted.scrollIntoView({ block: 'center' });
  }
};

watch(() => props.selectedNode, (node) => {
  if (node) {
    highlightedLines.value = { start: node.startLine, end: node.endLine };
  } else {
    highlightedLines.value = null;
  }

  nextTick(() => {
    if (node) {
      scrollToHighlighted();
    }
  });
});

const getDisplayLines = () => {
  if (!fileContent.value) return [];

  return fileContent.value.split('\n').map((content, index) => {
    const lineNumber = index + 1;
    const isHighlighted = highlightedLines.value
      ? lineNumber >= highlightedLines.value.start && lineNumber <= highlightedLines.value.end
      : false;

    return { lineNumber, content, isHighlighted };
  });
};
</script>

<template>
  <div class="outline-content-viewer">
    <div v-if="!filePath" class="empty-state">
      <p>Select a file from the outline</p>
    </div>

    <div v-else-if="isLoading" class="loading-state">
      <p>Loading file...</p>
    </div>

    <div v-else-if="error" class="error-state">
      <p>{{ error }}</p>
    </div>

    <div v-else class="content-display">
      <header class="file-header">
        <div class="file-path">{{ filePath }}</div>
        <div v-if="selectedNode" class="selection-info">
          <span class="node-name">{{ selectedNode.name }}</span>
          <span class="line-range">Lines {{ selectedNode.startLine }}-{{ selectedNode.endLine }}</span>
        </div>
      </header>

      <div class="code-container" ref="codeContainerRef">
        <pre class="code-content">
          <code>
            <div
              v-for="line in getDisplayLines()"
              :key="line.lineNumber"
              :class="['code-line', { 'line-highlighted': line.isHighlighted }]"
            >
              <span class="line-number">{{ line.lineNumber }}</span>
              <span class="line-content">{{ line.content }}</span>
            </div>
          </code>
        </pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.outline-content-viewer {
  height: 100%;
  display: flex;
  flex-direction: column;
  text-align: left;
}

.empty-state,
.loading-state,
.error-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #9a9da3;
  font-size: 0.95rem;
}

.error-state {
  color: #f48771;
}

.content-display {
  display: flex;
  flex-direction: column;
  height: 100%;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  overflow: hidden;
}

.file-header {
  padding: 1rem 1.25rem;
  background: #252526;
  border-bottom: 1px solid #3e3e42;
}

.file-path {
  margin: 0 0 0.35rem 0;
  color: #d4d4d4;
  font-size: 1rem;
  font-weight: 600;
  word-break: break-all;
}

.selection-info {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  color: #9a9da3;
  flex-wrap: wrap;
  align-items: center;
}

.node-name {
  font-weight: 500;
  color: #4ec9b0;
}

.line-range {
  color: #6a9955;
}

.code-container {
  flex: 1;
  overflow-y: auto;
  background: #1e1e1e;
}

.code-content {
  margin: 0;
  padding: 0;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.5;
  color: #d4d4d4;
  background: transparent;
}

.code-line {
  display: flex;
  align-items: flex-start;
  padding: 0 1rem;
  transition: background-color 0.2s;
}

.code-line:hover {
  background: #2a2d2e;
}

.line-highlighted {
  background: #264f78 !important;
  border-left: 3px solid #007acc;
}

.line-number {
  width: 3rem;
  margin-right: 1.5rem;
  text-align: right;
  color: #858585;
  user-select: none;
  flex-shrink: 0;
}

.line-content {
  white-space: pre;
  flex: 1;
  overflow-wrap: break-word;
}
</style>
