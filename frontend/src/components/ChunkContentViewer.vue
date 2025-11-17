<!--
  File: components/ChunkContentViewer.vue
  Purpose: Displays semantic chunk metadata and associated source lines.
  Author: CodeTextor project
-->

<script setup lang="ts">
import { computed } from 'vue';
import type { Chunk } from '../types';

interface Props {
  projectId: string;
  filePath: string;
  selectedChunk?: Chunk | null;
}

const props = defineProps<Props>();

const chunkLineCount = computed(() => {
  if (!props.selectedChunk) return null;
  return props.selectedChunk.lineEnd - props.selectedChunk.lineStart + 1;
});

const formatTokenCount = (count?: number): string => {
  if (!count) return '';
  if (count < 1000) return `${count}t`;
  return `${(count / 1000).toFixed(1)}kt`;
};

const chunkContentLines = computed(() => {
  if (!props.selectedChunk || !props.selectedChunk.content) {
    return [];
  }
  const lines = props.selectedChunk.content.split('\n');
  return lines.map((content, index) => ({
    lineNumber: index + 1,
    content,
  }));
});
</script>

<template>
  <div class="chunk-content-viewer">
    <div v-if="!selectedChunk" class="empty-state">
      <p>Select a chunk to view details</p>
    </div>
    <div v-else class="content-display">
      <div class="chunk-panel">
        <header class="panel-header">
          <div class="file-info">
            <div class="file-path">{{ filePath }}</div>
            <div class="selection-info">
              <span class="node-name">{{ selectedChunk.symbolName || 'Chunk selezionato' }}</span>
            </div>
          </div>
          <div class="panel-stat-chips">
            <span v-if="selectedChunk.language" class="stat-chip">{{ selectedChunk.language }}</span>
            <span class="stat-chip">L{{ selectedChunk.lineStart }}-{{ selectedChunk.lineEnd }}</span>
            <span v-if="chunkLineCount" class="stat-chip">{{ chunkLineCount }} linee</span>
            <span v-if="selectedChunk.tokenCount" class="stat-chip emphasis">{{ formatTokenCount(selectedChunk.tokenCount) }}</span>
          </div>
        </header>

        <section class="panel-body file-section">
          <div class="body-title">Contenuto del chunk (testo inviato all'embedder)</div>
          <pre class="chunk-code"><code><div
  v-for="line in chunkContentLines"
  :key="line.lineNumber"
  class="code-line"
><span class="line-number">{{ line.lineNumber }}</span><span class="line-content">{{ line.content }}</span></div></code></pre>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chunk-content-viewer {
  height: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  text-align: left;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #9a9da3;
  font-size: 0.95rem;
}

.content-display {
  flex: 1;
  display: flex;
  min-height: 0;
}

.chunk-panel {
  background: #1b1b1b;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  overflow: hidden;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.5rem;
  background: #252526;
  border-bottom: 1px solid #3e3e42;
}

.file-info {
  flex: 1;
  min-width: 0;
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

.panel-stat-chips {
  display: flex;
  gap: 0.35rem;
  align-items: center;
  flex-wrap: wrap;
}

.stat-chip {
  padding: 0.15rem 0.7rem;
  border-radius: 999px;
  font-size: 0.78rem;
  color: #d4d4d4;
  background: #2d2f34;
  border: 1px solid #3a3d43;
}

.stat-chip.emphasis {
  color: #0e141b;
  background: #a0c566;
  border-color: #a0c566;
  font-weight: 600;
}

.panel-body {
  flex: 1;
  background: #101112;
  padding: 1rem 1.5rem 2rem;
  overflow: auto;
  min-height: 0;
  scrollbar-width: thin;
  scrollbar-color: #555555 #1e1e1e;
}

.panel-body.file-section {
  border-top: 1px solid #2b2d31;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.body-title {
  margin-bottom: 0.6rem;
  font-size: 0.85rem;
  color: #9a9da3;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.chunk-code {
  margin: 0;
  padding: 0;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.5;
  color: #d4d4d4;
  background: transparent;
  display: block;
  flex: 1;
  overflow: auto;
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
}

/* Scrollbar styling */
.panel-body::-webkit-scrollbar {
  width: 12px;
}

.panel-body::-webkit-scrollbar-track {
  background: #1e1e1e;
  border-left: 1px solid #3e3e42;
}

.panel-body::-webkit-scrollbar-thumb {
  background: #555555;
  border-radius: 6px;
  border: 2px solid #1e1e1e;
}

.panel-body::-webkit-scrollbar-thumb:hover {
  background: #666666;
}
</style>
