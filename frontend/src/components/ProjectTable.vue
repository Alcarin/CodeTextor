<!--
  File: components/ProjectTable.vue
  Purpose: Table view for projects list.
  Author: CodeTextor project
-->

<script setup lang="ts">
import type { Project } from '../types';

interface Props {
  projects: Project[];
  currentProjectId?: string;
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
  <div class="projects-table-container">
    <table class="projects-table">
      <thead>
        <tr>
          <th>Status</th>
          <th>Name</th>
          <th>ID</th>
          <th>Created</th>
          <th>Last Indexed</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="project in projects"
          :key="project.id"
          :class="{
            'row-active': currentProjectId === project.id,
            'row-indexing': project.isIndexing
          }"
        >
          <td class="status-cell">
            <span v-if="project.isIndexing" class="status-badge indexing">● Indexing</span>
            <span v-else-if="currentProjectId === project.id" class="status-badge active">● Active</span>
            <span v-else class="status-badge">○</span>
          </td>
          <td class="name-cell">
            <div class="name-wrapper">
              <span class="project-icon">📁</span>
              <strong>{{ project.name }}</strong>
            </div>
          </td>
          <td class="id-cell"><code>{{ project.id }}</code></td>
          <td class="date-cell">{{ formatDate(project.createdAt) }}</td>
          <td class="date-cell">{{ project.stats ? formatDate(project.stats.lastIndexedAt) : 'Never' }}</td>
          <td class="actions-cell">
            <div class="table-actions">
              <button
                @click="emit('go-to-indexing', project)"
                class="btn-icon btn-primary"
                title="Go to Indexing"
              >
                →
              </button>
              <button
                @click="emit('edit', project)"
                class="btn-icon btn-secondary"
                title="Edit"
              >
                ✎
              </button>
              <button
                @click="emit('delete', project)"
                class="btn-icon btn-danger"
                title="Delete"
              >
                ✕
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.projects-table-container {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  overflow: hidden;
}

.projects-table {
  width: 100%;
  border-collapse: collapse;
}

.projects-table thead {
  background: #1e1e1e;
  border-bottom: 2px solid #3e3e42;
}

.projects-table th {
  padding: 1rem;
  text-align: left;
  color: #d4d4d4;
  font-weight: 600;
  font-size: 0.9rem;
  white-space: nowrap;
}

.projects-table tbody tr {
  border-bottom: 1px solid #3e3e42;
  transition: background 0.2s ease;
}

.projects-table tbody tr:last-child {
  border-bottom: none;
}

.projects-table tbody tr:hover {
  background: #2a2a2b;
}

.projects-table tbody tr.row-active {
  background: #1a2533;
}

.projects-table tbody tr.row-indexing {
  background: #1a2e1a;
}

.projects-table td {
  padding: 1rem;
  color: #d4d4d4;
  font-size: 0.9rem;
}

.status-cell {
  width: 100px;
}

.status-badge {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  color: #858585;
}

.status-badge.indexing {
  background: #28a745;
  color: white;
}

.status-badge.active {
  background: #007acc;
  color: white;
}

.name-cell {
  min-width: 200px;
}

.name-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.id-cell code {
  background: #1e1e1e;
  padding: 0.25rem 0.5rem;
  border-radius: 3px;
  color: #4ec9b0;
  font-family: 'Courier New', monospace;
  font-size: 0.85rem;
}

.date-cell {
  white-space: nowrap;
  font-size: 0.85rem;
  color: #858585;
}

.actions-cell {
  width: 120px;
}

.table-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
}

.btn-icon {
  width: 32px;
  height: 32px;
  padding: 0;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  font-weight: bold;
}

.btn-icon.btn-primary {
  background: #007acc;
  color: white;
}

.btn-icon.btn-primary:hover {
  background: #005a9e;
}

.btn-icon.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-icon.btn-secondary:hover {
  background: #5a6268;
}

.btn-icon.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-icon.btn-danger:hover {
  background: #c82333;
}
</style>
