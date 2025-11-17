<!--
  File: views/SearchView.vue
  Purpose: Semantic search interface with result display.
  Author: CodeTextor project
  Notes: Provides query input, filters, and displays search results with similarity scores.
-->

<script setup lang="ts">
import { ref, computed } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { mockBackend } from '../services/mockBackend';
import type { SearchResponse, Chunk } from '../types';

// Get current project
const { currentProject } = useCurrentProject();

// State
const query = ref<string>('');
const topK = ref<number>(10);
const isSearching = ref<boolean>(false);
const searchResults = ref<SearchResponse | null>(null);
const selectedChunk = ref<Chunk | null>(null);

// Computed
const hasResults = computed(() => searchResults.value && searchResults.value.chunks.length > 0);

/**
 * Executes semantic search with current query parameters.
 */
const performSearch = async () => {
  if (!currentProject.value) {
    alert('Please select a project first');
    return;
  }

  if (!query.value.trim()) {
    alert('Please enter a search query');
    return;
  }

  isSearching.value = true;

  try {
    const results = await mockBackend.semanticSearch({
      projectId: currentProject.value.id,
      query: query.value,
      k: topK.value
    });

    searchResults.value = results;
    selectedChunk.value = null; // Clear selection
  } catch (error) {
    console.error('Search failed:', error);
    alert('Search failed: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isSearching.value = false;
  }
};

/**
 * Selects a chunk to display its full content.
 * @param chunk - The chunk to select
 */
const selectChunk = (chunk: Chunk) => {
  selectedChunk.value = chunk;
};

/**
 * Clears current search results and resets form.
 */
const clearSearch = () => {
  searchResults.value = null;
  selectedChunk.value = null;
};

/**
 * Formats similarity score as percentage.
 * @param similarity - Similarity score (0-1)
 * @returns Formatted percentage string
 */
const formatSimilarity = (similarity?: number): string => {
  if (!similarity) return 'N/A';
  return `${(similarity * 100).toFixed(1)}%`;
};

/**
 * Gets color class based on similarity score.
 * @param similarity - Similarity score (0-1)
 * @returns CSS class name
 */
const getSimilarityColor = (similarity?: number): string => {
  if (!similarity) return 'similarity-low';
  if (similarity > 0.8) return 'similarity-high';
  if (similarity > 0.6) return 'similarity-medium';
  return 'similarity-low';
};
</script>

<template>
  <div class="search-view">
    <!-- Project context info -->
    <div v-if="currentProject" class="project-context section">
      <div class="info-banner">
        <span class="info-icon">🔍</span>
        <div class="info-content">
          <strong>Searching in:</strong> {{ currentProject.name }}
          <span class="db-path">(Database: <code>indexes/{{ currentProject.id }}.db</code>)</span>
        </div>
      </div>
    </div>

    <!-- Search form -->
    <div class="search-form section">
      <div class="form-group">
        <label for="query">Search Query</label>
        <input
          id="query"
          v-model="query"
          type="text"
          placeholder="e.g., 'function that handles user authentication'"
          class="input-text"
          @keyup.enter="performSearch"
          :disabled="isSearching"
        />
      </div>

      <div class="form-row">
        <div class="form-group">
          <label for="topK">Max Results</label>
          <input
            id="topK"
            v-model.number="topK"
            type="number"
            min="1"
            max="50"
            class="input-number"
            :disabled="isSearching"
          />
        </div>
      </div>

      <div class="form-actions">
        <button
          @click="performSearch"
          :disabled="isSearching || !query.trim()"
          class="btn btn-primary"
        >
          {{ isSearching ? 'Searching...' : 'Search' }}
        </button>
        <button
          v-if="hasResults"
          @click="clearSearch"
          class="btn btn-secondary"
        >
          Clear
        </button>
      </div>
    </div>

    <!-- Search results -->
    <div v-if="searchResults" class="results-section">
      <div class="results-header">
        <h3>Results</h3>
        <div class="results-meta">
          Found {{ searchResults.totalResults }} results in {{ searchResults.queryTime }}ms
        </div>
      </div>

      <div v-if="hasResults" class="results-container">
        <!-- Results list -->
        <div class="results-list">
          <div
            v-for="chunk in searchResults.chunks"
            :key="chunk.id"
            :class="['result-item', { selected: selectedChunk?.id === chunk.id }]"
            @click="selectChunk(chunk)"
          >
            <div class="result-header">
              <span class="result-name">{{ chunk.symbolName || 'unnamed' }}</span>
              <span :class="['result-similarity', getSimilarityColor(chunk.similarity)]">
                {{ formatSimilarity(chunk.similarity) }}
              </span>
            </div>
            <div class="result-meta">
              <span class="result-kind">{{ chunk.symbolKind }}</span>
              <span class="result-location">{{ chunk.filePath }}:{{ chunk.lineStart }}</span>
            </div>
          </div>
        </div>

        <!-- Selected chunk detail -->
        <div v-if="selectedChunk" class="chunk-detail">
          <div class="detail-header">
            <h4>{{ selectedChunk.symbolName || 'unnamed' }}</h4>
            <button @click="selectedChunk = null" class="btn-close">×</button>
          </div>
          <div class="detail-meta">
            <span>{{ selectedChunk.symbolKind }}</span>
            <span>Lines {{ selectedChunk.lineStart }}-{{ selectedChunk.lineEnd }}</span>
          </div>
          <div class="detail-path">{{ selectedChunk.filePath }}</div>
          <pre class="detail-code"><code>{{ selectedChunk.content }}</code></pre>
        </div>
      </div>

      <div v-else class="no-results">
        No results found for your query. Try different search terms.
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-view {
  max-width: 1400px;
  margin: 0 auto;
}

.section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  color: #d4d4d4;
  font-weight: 500;
}

.input-text {
  width: 100%;
  padding: 0.75rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #d4d4d4;
  font-size: 0.95rem;
}

.input-text:focus {
  outline: none;
  border-color: #007acc;
}

.form-row {
  display: flex;
  gap: 1rem;
}

.input-number {
  width: 120px;
  padding: 0.75rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #d4d4d4;
}

.form-actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: #007acc;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #005a9e;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background: #5a6268;
}

.results-section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
}

.results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #3e3e42;
}

.results-header h3 {
  margin: 0;
  color: #d4d4d4;
}

.results-meta {
  color: #858585;
  font-size: 0.9rem;
}

.results-container {
  display: grid;
  grid-template-columns: 400px 1fr;
  gap: 1.5rem;
  min-height: 400px;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  overflow-y: auto;
  max-height: 600px;
}

.result-item {
  padding: 1rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.result-item:hover {
  background: #2d2d30;
  border-color: #007acc;
}

.result-item.selected {
  background: #2d2d30;
  border-color: #007acc;
  border-width: 2px;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.result-name {
  font-weight: 600;
  color: #d4d4d4;
}

.result-similarity {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8rem;
  font-weight: 600;
}

.similarity-high {
  background: #28a745;
  color: white;
}

.similarity-medium {
  background: #ffc107;
  color: #000;
}

.similarity-low {
  background: #6c757d;
  color: white;
}

.result-meta {
  display: flex;
  gap: 0.75rem;
  font-size: 0.85rem;
  color: #858585;
}

.result-kind {
  padding: 0.125rem 0.5rem;
  background: #007acc;
  border-radius: 3px;
  color: white;
}

.chunk-detail {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 1.5rem;
  overflow: auto;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
}

.detail-header h4 {
  margin: 0;
  color: #d4d4d4;
}

.btn-close {
  background: transparent;
  border: none;
  color: #858585;
  font-size: 1.5rem;
  cursor: pointer;
  padding: 0;
  width: 30px;
  height: 30px;
}

.btn-close:hover {
  color: #d4d4d4;
}

.detail-meta {
  display: flex;
  gap: 1rem;
  margin-bottom: 0.5rem;
  font-size: 0.85rem;
  color: #858585;
}

.detail-path {
  color: #007acc;
  font-family: 'Courier New', monospace;
  font-size: 0.85rem;
  margin-bottom: 1rem;
}

.detail-code {
  background: #0d1117;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  padding: 1rem;
  overflow-x: auto;
  margin: 0;
}

.detail-code code {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.5;
}

.no-results {
  padding: 3rem;
  text-align: center;
  color: #858585;
}

/* Project context banner */
.project-context {
  margin-bottom: 1.5rem;
}

.info-banner {
  display: flex;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  background: #1a3a5a;
  border: 1px solid #007acc;
  border-radius: 4px;
  align-items: center;
}

.info-icon {
  font-size: 1.2rem;
  flex-shrink: 0;
}

.info-content {
  flex: 1;
  color: #7fc7ff;
  font-size: 0.9rem;
  line-height: 1.5;
}

.info-content strong {
  color: #a8d8ff;
  margin-right: 0.5rem;
}

.db-path {
  margin-left: 0.5rem;
  font-size: 0.85rem;
  color: #9ec7e0;
}

.db-path code {
  background: #0d2438;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
  color: #4ec9b0;
  font-family: 'Courier New', monospace;
  font-size: 0.85em;
  border: 1px solid #1a4a6e;
}
</style>
