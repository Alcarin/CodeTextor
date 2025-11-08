<!--
  File: components/ProjectCard.vue
  Purpose: Single project card component for grid view.
  Author: CodeTextor project
-->

<script setup lang="ts">
import type { Project } from '../types';

interface Props {
  project: Project;
  isActive: boolean;
}

interface Emits {
  (e: 'go-to-indexing', project: Project): void;
  (e: 'edit', project: Project): void;
  (e: 'delete', project: Project): void;
}

defineProps<Props>();
const emit = defineEmits<Emits>();

/**
 * Formats date to relative time or system locale format.
 */
const formatDate = (date?: Date | number | string): string => {
  if (!date) return 'Never';

  const timestamp = typeof date === 'number' ? date * 1000 : date;
  const now = new Date();
  const target = new Date(timestamp);

  if (isNaN(target.getTime())) return 'Invalid date';

  const diffMs = now.getTime() - target.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return target.toLocaleString();
};
</script>

<template>
  <div
    :class="['project-card', {
      active: isActive,
      indexing: project.isIndexing
    }]"
  >
    <!-- Indexing badge -->
    <div v-if="project.isIndexing" class="indexing-badge">
      ● Indexing
    </div>

    <!-- Project header -->
    <div class="project-header">
      <span class="project-icon">📁</span>
      <div class="project-info">
        <h3>{{ project.name }}</h3>
        <p v-if="project.description" class="project-description">
          {{ project.description }}
        </p>
      </div>
    </div>

    <!-- Project details -->
    <div class="project-details">
      <div class="detail-row">
        <span class="detail-label">ID:</span>
        <code class="detail-value">{{ project.id }}</code>
      </div>
      <div class="detail-row">
        <span class="detail-label">Database:</span>
        <code class="detail-value db-path">indexes/project-{{ project.id }}.db</code>
      </div>
      <div class="detail-row">
        <span class="detail-label">Root:</span>
        <span class="detail-value">{{ project.config?.rootPath || '—' }}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Created:</span>
        <span class="detail-value">{{ formatDate(project.createdAt) }}</span>
      </div>
      <div class="detail-row" v-if="project.stats">
        <span class="detail-label">Last Indexed:</span>
        <span class="detail-value">{{ formatDate(project.stats.lastIndexedAt) }}</span>
      </div>
    </div>

    <!-- Actions -->
    <div class="project-actions">
      <button
        @click="emit('go-to-indexing', project)"
        class="btn btn-primary btn-sm"
      >
        Go to Indexing
      </button>
      <button
        @click="emit('edit', project)"
        class="btn btn-secondary btn-sm"
        data-testid="edit-project-button"
      >
        Edit
      </button>
      <button
        @click="emit('delete', project)"
        class="btn btn-danger btn-sm"
      >
        Delete
      </button>
    </div>
  </div>
</template>

<style scoped>
.project-card {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  transition: all 0.2s ease;
  position: relative;
}

.project-card:hover {
  border-color: #007acc;
  box-shadow: 0 4px 12px rgba(0, 122, 204, 0.2);
}

.project-card.active {
  border-color: #007acc;
  background: #1a2533;
  box-shadow: 0 4px 12px rgba(0, 122, 204, 0.2);
}

.project-card.indexing {
  border-color: #28a745;
  background: #1a2e1a;
  box-shadow: 0 4px 12px rgba(40, 167, 69, 0.2);
}

.indexing-badge {
  position: absolute;
  top: 1rem;
  right: 1rem;
  background: #28a745;
  color: white;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
}

.project-header {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
  margin-bottom: 1rem;
}

.project-icon {
  font-size: 2rem;
}

.project-info {
  flex: 1;
}

.project-info h3 {
  margin: 0 0 0.5rem 0;
  color: #d4d4d4;
  font-size: 1.2rem;
}

.project-description {
  margin: 0;
  color: #858585;
  font-size: 0.9rem;
  line-height: 1.4;
}

.project-details {
  padding: 1rem 0;
  margin-bottom: 1rem;
}

.detail-row {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
  font-size: 0.85rem;
}

.detail-row:last-child {
  margin-bottom: 0;
}

.detail-label {
  color: #858585;
  font-weight: 600;
  min-width: 100px;
}

.detail-value {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  word-break: break-all;
}

.detail-value.db-path {
  color: #4ec9b0;
  background: #1a1a1a;
  padding: 0.2rem 0.5rem;
  border-radius: 3px;
}

.project-actions {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
  font-weight: 500;
}

.btn-sm {
  padding: 0.5rem 1rem;
  font-size: 0.85rem;
  flex: 1;
}

.btn-primary {
  background: #007acc;
  color: white;
}

.btn-primary:hover {
  background: #005a9e;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background: #5a6268;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover {
  background: #c82333;
}
</style>
