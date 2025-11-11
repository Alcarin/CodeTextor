<!--
  File: components/FileContentViewer.vue
  Purpose: Displays file content with syntax highlighting for selected sections
  Author: CodeTextor project
-->

<script setup lang="ts">
import { ref, watch } from 'vue';
import { backend } from '../api/backend';
import type { OutlineNode } from '../types';

interface Props {
  projectId: string;
  filePath: string;
  selectedNode: OutlineNode | null;
  fileVersion?: number;
}

const props = defineProps<Props>();

const fileContent = ref<string>('');
const isLoading = ref<boolean>(false);
const error = ref<string>('');
const highlightedLines = ref<{ start: number; end: number } | null>(null);

// Load file content
const loadFileContent = async () => {
  if (!props.filePath || !props.projectId) {
    fileContent.value = '';
    return;
  }

  isLoading.value = true;
  error.value = '';

  try {
    fileContent.value = await backend.readFileContent(props.projectId, props.filePath);
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Errore nel caricamento del file';
    fileContent.value = '';
  } finally {
    isLoading.value = false;
  }
};

// Update highlighted lines when selected node changes
watch(() => props.selectedNode, (node) => {
  if (node) {
    highlightedLines.value = {
      start: node.startLine,
      end: node.endLine
    };
  } else {
    highlightedLines.value = null;
  }
});

// Load file when filePath changes
watch(() => props.filePath, () => {
  loadFileContent();
}, { immediate: true });

watch(() => props.fileVersion, (newValue, oldValue) => {
  if (newValue !== undefined && newValue !== oldValue) {
    loadFileContent();
  }
});

// Get lines to display
const getDisplayLines = (): { lineNumber: number; content: string; isHighlighted: boolean }[] => {
  if (!fileContent.value) return [];

  const lines = fileContent.value.split('\n');
  return lines.map((content, index) => {
    const lineNumber = index + 1;
    const isHighlighted = highlightedLines.value
      ? lineNumber >= highlightedLines.value.start && lineNumber <= highlightedLines.value.end
      : false;

    return { lineNumber, content, isHighlighted };
  });
};

// Scroll to highlighted section
const scrollToHighlight = () => {
  if (!highlightedLines.value) return;

  const element = document.querySelector('.line-highlighted');
  if (element) {
    element.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }
};

// Watch for highlight changes to scroll
watch(highlightedLines, () => {
  setTimeout(scrollToHighlight, 100);
});
</script>

<template>
  <div class="file-content-viewer">
    <div v-if="!filePath" class="empty-state">
      <p>Seleziona un nodo dall'albero per visualizzare il contenuto</p>
    </div>

    <div v-else-if="isLoading" class="loading-state">
      <p>Caricamento file...</p>
    </div>

    <div v-else-if="error" class="error-state">
      <p>{{ error }}</p>
    </div>

    <div v-else class="content-display">
      <div class="file-header">
        <h3>{{ filePath }}</h3>
        <div v-if="selectedNode" class="selection-info">
          <span class="node-name">{{ selectedNode.name }}</span>
          <span class="line-range">Lines {{ selectedNode.startLine }}-{{ selectedNode.endLine }}</span>
        </div>
      </div>

      <div class="code-container">
        <pre class="code-content"><code><div
  v-for="line in getDisplayLines()"
  :key="line.lineNumber"
  :class="['code-line', { 'line-highlighted': line.isHighlighted }]"
><span class="line-number">{{ line.lineNumber }}</span><span class="line-content">{{ line.content }}</span></div></code></pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.file-content-viewer {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
  border-radius: 8px;
  overflow: hidden;
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
  overflow: hidden;
}

.file-header {
  padding: 1rem 1.5rem;
  background: #252526;
  border-bottom: 1px solid #3e3e42;
}

.file-header h3 {
  margin: 0 0 0.5rem 0;
  color: #d4d4d4;
  font-size: 1rem;
  font-weight: 500;
}

.selection-info {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  color: #9a9da3;
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
  overflow: auto;
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
  text-align: left;
}

.code-line {
  display: flex;
  align-items: flex-start;
  padding: 0 1rem;
  transition: background-color 0.2s;
  text-align: left;
}

.code-line:hover {
  background: #2a2d2e;
}

.line-highlighted {
  background: #264f78 !important;
  border-left: 3px solid #007acc;
}

.line-number {
  display: inline-block;
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
  text-align: left;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

/* Scrollbar styling */
.code-container::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.code-container::-webkit-scrollbar-track {
  background: #1e1e1e;
}

.code-container::-webkit-scrollbar-thumb {
  background: #424242;
  border-radius: 5px;
}

.code-container::-webkit-scrollbar-thumb:hover {
  background: #4e4e4e;
}
</style>
