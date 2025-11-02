<!--
  File: views/StatsView.vue
  Purpose: Displays project statistics and indexing metadata.
  Author: CodeTextor project
  Notes: Shows aggregated information about indexed codebase.
-->

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { mockBackend } from '../services/mockBackend';
import type { ProjectStats } from '../types';

// Get current project
const { currentProject } = useCurrentProject();

// State
const stats = ref<ProjectStats | null>(null);
const isLoading = ref<boolean>(false);

/**
 * Loads project statistics from backend.
 */
const loadStats = async () => {
  if (!currentProject.value) {
    stats.value = null;
    return;
  }

  isLoading.value = true;

  try {
    const result = await mockBackend.getProjectStats();
    stats.value = result;
  } catch (error) {
    console.error('Failed to load stats:', error);
    alert('Failed to load stats: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isLoading.value = false;
  }
};

/**
 * Formats byte size to human-readable string.
 * @param bytes - Size in bytes
 * @returns Formatted string (e.g., "2.4 MB")
 */
const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes';

  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
};

/**
 * Formats date to locale string.
 * @param date - Date object or undefined
 * @returns Formatted date string
 */
const formatDate = (date?: Date): string => {
  if (!date) return 'Never';
  return new Date(date).toLocaleString();
};

// Watch for current project changes
watch(currentProject, () => {
  loadStats();
});

// Load stats on component mount
onMounted(() => {
  loadStats();
});
</script>

<template>
  <div class="stats-view">
    <!-- No project selected -->
    <div v-if="!currentProject" class="empty-state section">
      <div class="empty-icon">📁</div>
      <h3>No Project Selected</h3>
      <p>Please select a project from the dropdown in the navbar to view statistics.</p>
    </div>

    <!-- Loading state -->
    <div v-else-if="isLoading" class="loading-state section">
      <div class="spinner"></div>
      <p>Loading statistics...</p>
    </div>

    <!-- Stats display -->
    <div v-else-if="stats" class="stats-container">
      <!-- Database info banner -->
      <div class="database-info section">
        <div class="info-banner">
          <span class="info-icon">💾</span>
          <div class="info-content">
            <strong>Database Location:</strong>
            <code>indexes/{{ currentProject.id }}.db</code>
            <br>
            <span class="info-detail">
              This project's data is stored in an isolated SQLite-vec database, ensuring complete separation from other projects.
            </span>
          </div>
        </div>
      </div>

      <!-- Summary cards -->
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon">📁</div>
          <div class="stat-content">
            <div class="stat-label">Total Files</div>
            <div class="stat-value">{{ stats.totalFiles.toLocaleString() }}</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-icon">🧩</div>
          <div class="stat-content">
            <div class="stat-label">Total Chunks</div>
            <div class="stat-value">{{ stats.totalChunks.toLocaleString() }}</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-icon">🔤</div>
          <div class="stat-content">
            <div class="stat-label">Total Symbols</div>
            <div class="stat-value">{{ stats.totalSymbols.toLocaleString() }}</div>
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-icon">💾</div>
          <div class="stat-content">
            <div class="stat-label">Index Size</div>
            <div class="stat-value">{{ formatBytes(stats.indexSize) }}</div>
          </div>
        </div>
      </div>

      <!-- Detailed info -->
      <div class="info-section section">
        <h3>Indexing Information</h3>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">Last Indexed:</span>
            <span class="info-value">{{ formatDate(stats.lastIndexed) }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Average Chunks per File:</span>
            <span class="info-value">
              {{ stats.totalFiles > 0 ? (stats.totalChunks / stats.totalFiles).toFixed(1) : 'N/A' }}
            </span>
          </div>
          <div class="info-item">
            <span class="info-label">Average Symbols per File:</span>
            <span class="info-value">
              {{ stats.totalFiles > 0 ? (stats.totalSymbols / stats.totalFiles).toFixed(1) : 'N/A' }}
            </span>
          </div>
        </div>
      </div>

      <!-- Refresh button -->
      <div class="actions">
        <button @click="loadStats" class="btn btn-primary">
          Refresh Statistics
        </button>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="empty-state section">
      <p>No statistics available. Please index a project first.</p>
    </div>
  </div>
</template>

<style scoped>
.stats-view {
  max-width: 1200px;
  margin: 0 auto;
}

.section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem;
  gap: 1rem;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #3e3e42;
  border-top-color: #007acc;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1.5rem;
  margin-bottom: 1.5rem;
}

.stat-card {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 122, 204, 0.2);
}

.stat-icon {
  font-size: 2.5rem;
}

.stat-content {
  flex: 1;
}

.stat-label {
  color: #858585;
  font-size: 0.85rem;
  margin-bottom: 0.25rem;
}

.stat-value {
  color: #d4d4d4;
  font-size: 1.8rem;
  font-weight: 600;
}

.info-section h3 {
  margin: 0 0 1rem 0;
  color: #d4d4d4;
}

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.info-item {
  display: flex;
  justify-content: space-between;
  padding: 0.75rem;
  background: #1e1e1e;
  border-radius: 4px;
}

.info-label {
  color: #858585;
  font-weight: 500;
}

.info-value {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
}

.actions {
  display: flex;
  justify-content: center;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-primary {
  background: #007acc;
  color: white;
}

.btn-primary:hover {
  background: #005a9e;
}

.empty-state {
  text-align: center;
  padding: 3rem;
  color: #858585;
}

/* Database info banner */
.database-info {
  margin-bottom: 1.5rem;
}

.info-banner {
  display: flex;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  background: #1a3a5a;
  border: 1px solid #007acc;
  border-radius: 4px;
  align-items: flex-start;
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

.info-content code {
  background: #0d2438;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
  color: #4ec9b0;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
  border: 1px solid #1a4a6e;
}

.info-detail {
  display: inline-block;
  margin-top: 0.5rem;
  color: #9ec7e0;
  font-size: 0.85rem;
}
</style>
