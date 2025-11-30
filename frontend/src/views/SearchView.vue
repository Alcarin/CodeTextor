<!--
  File: views/SearchView.vue
  Purpose: Semantic search interface with result display.
  Author: CodeTextor project
  Notes: Provides query input, filters, and displays search results with similarity scores.
-->

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { backend } from '../api/backend';
import type { SearchResponse, Chunk } from '../types';

// Get current project
const { currentProject } = useCurrentProject();

// State
const query = ref<string>('');
const topK = ref<number>(10);
const isSearching = ref<boolean>(false);
const searchResults = ref<SearchResponse | null>(null);
const selectedChunk = ref<Chunk | null>(null);
const textareaRef = ref<HTMLTextAreaElement | null>(null);

/**
 * Adjusts the height of the textarea to fit content.
 */
const adjustHeight = () => {
  const textarea = textareaRef.value;
  if (textarea) {
    textarea.style.height = 'auto';
    if (textarea.value) {
      // scrollHeight includes padding. Add 2px for borders (box-sizing: border-box)
      textarea.style.height = `${textarea.scrollHeight + 2}px`;
    } else {
      // Reset to default height if empty (handled by CSS min-height)
      textarea.style.height = '';
    }
  }
};

onMounted(() => {
  adjustHeight();
});

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
    const results = await backend.search(currentProject.value.id, query.value, topK.value);

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
    <!-- Search form -->
    <div class="search-form section">
      <div class="form-inline">
        <div class="form-group inline">
          <label for="query">Search Query</label>
          <textarea
            id="query"
            ref="textareaRef"
            v-model="query"
            placeholder="e.g., 'function that handles user authentication'"
            class="input-text input-area"
            @keydown.enter.prevent="performSearch"
            @input="adjustHeight"
            :disabled="isSearching"
            rows="1"
          ></textarea>
        </div>
        <div class="form-group inline compact">
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
        <div class="form-actions inline">
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
  text-align: left;
}

.search-form.section {
  max-width: 1100px;
  margin-left: auto;
  margin-right: auto;
  padding: 1rem 1.25rem;
}

.form-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 2rem;
}

.form-group.inline label {
  margin-bottom: 0.35rem;
}

.form-group.inline {
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  flex: 1 1 300px; /* Grow, shrink, basis */
  margin-bottom: 0;
}

.form-group.inline .input-text {
  width: 100%;
}

.form-group.inline.compact {
  flex: 0 0 auto;
  width: auto;
  min-width: 100px;
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
  padding: 0.45rem 0.65rem;
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

.input-area {
  box-sizing: border-box;
  resize: none; /* Disable manual resize to rely on auto-resize */
  min-height: 36px; /* Exact height: Content(~23px) + Padding(~11px) + Border(2px) */
  height: auto;
  overflow-y: hidden; /* Hide scrollbar */
  font-family: inherit;
  line-height: 1.5;
  padding: 0.35rem 0.65rem;
}

.form-row {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.input-number {
  width: 100%;
  min-width: 70px;
  padding: 0.4rem 0.55rem;
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

.form-actions.inline {
  margin-top: 0;
  display: flex;
  align-items: center;
  padding-bottom: 0;
  gap: 0.5rem;
  align-self: flex-start;
  margin-top: 1.7rem; /* Align with input fields (label height + gap) */
}
.form-actions.inline .btn {
  min-width: 90px;
  align-self: flex-end;
}

.btn {
  padding: 0.45rem 1.1rem;
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
  grid-template-columns: 360px 1fr;
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

@media (max-width: 1100px) {
  .results-container {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .form-group.inline {
    flex: 1 1 100%;
  }

  .form-group.inline.compact {
    flex: 0 0 100px;
  }
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
  display: block;
  white-space: pre-wrap;
  text-align: left;
}

.no-results {
  padding: 3rem;
  text-align: center;
  color: #858585;
}

</style>
