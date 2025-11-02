<!--
  File: components/ProjectSelector.vue
  Purpose: Dropdown component for selecting and managing projects.
  Author: CodeTextor project
  Notes: Shows current project and allows switching between projects.
-->

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { mockBackend } from '../services/mockBackend';
import type { Project } from '../types';

// Get current project composable
const { currentProject, setCurrentProject } = useCurrentProject();

// Local state
const projects = ref<Project[]>([]);
const showDropdown = ref(false);
const loading = ref(false);

/**
 * Loads list of all projects.
 */
const loadProjects = async () => {
  loading.value = true;
  try {
    const response = await mockBackend.listProjects();
    projects.value = response.projects;
  } catch (error) {
    console.error('Failed to load projects:', error);
  } finally {
    loading.value = false;
  }
};

/**
 * Handles project selection.
 * @param project - Project to select
 */
const handleSelectProject = async (project: Project) => {
  try {
    await setCurrentProject(project);
    showDropdown.value = false;
  } catch (error) {
    console.error('Failed to select project:', error);
  }
};

/**
 * Toggles dropdown visibility.
 */
const toggleDropdown = () => {
  showDropdown.value = !showDropdown.value;
  if (showDropdown.value) {
    loadProjects();
  }
};

/**
 * Closes dropdown when clicking outside.
 */
const closeDropdown = () => {
  showDropdown.value = false;
};

// Load projects on mount
onMounted(() => {
  loadProjects();
});
</script>

<template>
  <div class="project-selector" @blur="closeDropdown">
    <button class="selector-button" @click="toggleDropdown">
      <span class="project-icon">📁</span>
      <div class="project-info">
        <span v-if="currentProject" class="project-name">{{ currentProject.name }}</span>
        <span v-else class="no-project">No Project</span>
      </div>
      <span class="dropdown-icon">{{ showDropdown ? '▲' : '▼' }}</span>
    </button>

    <div v-if="showDropdown" class="dropdown-menu">
      <div v-if="loading" class="dropdown-loading">
        Loading projects...
      </div>
      <div v-else-if="projects.length === 0" class="dropdown-empty">
        No projects found. Go to Projects to create one.
      </div>
      <div v-else class="dropdown-list">
        <button
          v-for="project in projects"
          :key="project.id"
          :class="['dropdown-item', { active: currentProject?.id === project.id }]"
          @click="handleSelectProject(project)"
        >
          <div class="item-header">
            <span class="item-icon">📁</span>
            <span class="item-name">{{ project.name }}</span>
            <span v-if="currentProject?.id === project.id" class="item-badge">Active</span>
          </div>
          <div class="item-path">{{ project.path }}</div>
          <div v-if="project.lastIndexed" class="item-meta">
            Last indexed: {{ formatDate(project.lastIndexed) }}
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
/**
 * Formats a date to relative time.
 * @param date - Date to format
 * @returns Formatted string
 */
function formatDate(date: Date): string {
  const now = new Date();
  const diffMs = now.getTime() - new Date(date).getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return new Date(date).toLocaleDateString();
}
</script>

<style scoped>
.project-selector {
  position: relative;
}

.selector-button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 6px;
  color: white;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 200px;
}

.selector-button:hover {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
}

.project-icon {
  font-size: 1.2rem;
}

.project-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.project-name {
  font-weight: 500;
  font-size: 0.9rem;
}

.no-project {
  font-size: 0.9rem;
  color: rgba(255, 255, 255, 0.7);
}

.dropdown-icon {
  font-size: 0.7rem;
  color: rgba(255, 255, 255, 0.7);
}

.dropdown-menu {
  position: absolute;
  top: calc(100% + 0.5rem);
  left: 0;
  min-width: 300px;
  max-width: 500px;
  max-height: 400px;
  overflow-y: auto;
  background: #2d2d30;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  z-index: 1000;
}

.dropdown-loading,
.dropdown-empty {
  padding: 1rem;
  text-align: center;
  color: #858585;
  font-size: 0.9rem;
}

.dropdown-list {
  padding: 0.5rem;
}

.dropdown-item {
  width: 100%;
  padding: 0.75rem;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: #d4d4d4;
  cursor: pointer;
  text-align: left;
  transition: background 0.2s ease;
  margin-bottom: 0.25rem;
}

.dropdown-item:hover {
  background: #3e3e42;
}

.dropdown-item.active {
  background: #007acc;
}

.item-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
}

.item-icon {
  font-size: 1rem;
}

.item-name {
  flex: 1;
  font-weight: 500;
  font-size: 0.95rem;
}

.item-badge {
  padding: 0.125rem 0.5rem;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 600;
}

.item-path {
  font-size: 0.8rem;
  color: #858585;
  margin-bottom: 0.25rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-meta {
  font-size: 0.75rem;
  color: #6c757d;
}

/* Scrollbar styling */
.dropdown-menu::-webkit-scrollbar {
  width: 8px;
}

.dropdown-menu::-webkit-scrollbar-track {
  background: #1e1e1e;
}

.dropdown-menu::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 4px;
}

.dropdown-menu::-webkit-scrollbar-thumb:hover {
  background: #666;
}
</style>
