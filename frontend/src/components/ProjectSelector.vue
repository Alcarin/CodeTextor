<!--
  File: components/ProjectSelector.vue
  Purpose: Dropdown component for selecting and managing projects.
  Author: CodeTextor project
  Notes: Shows current project and allows switching between projects.
-->

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { backend } from '../api/backend';
import type { Project } from '../types';

// Get current project composable
const { currentProject, setCurrentProject } = useCurrentProject();

// Get navigation composable
const { navigateTo } = useNavigation();

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
    projects.value = await backend.listProjects();
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
 * Handles "View All" selection to navigate to projects page.
 */
const handleViewAll = () => {
  showDropdown.value = false;
  navigateTo('projects');
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
    <h1 class="selector-title" @click="toggleDropdown">
      <span v-if="currentProject" class="project-name">{{ currentProject.name }}</span>
      <span v-else class="no-project">No Project Selected</span>
      <span class="dropdown-icon">{{ showDropdown ? '▲' : '▼' }}</span>
    </h1>

    <div v-if="showDropdown" class="dropdown-menu">
      <div v-if="loading" class="dropdown-loading">
        Loading projects...
      </div>
      <div v-else class="dropdown-list">
        <!-- View All option as first item -->
        <button class="dropdown-item view-all" @click="handleViewAll">
          <div class="item-header">
            <span class="item-icon">📂</span>
            <span class="item-name">View All Projects</span>
          </div>
        </button>

        <!-- Divider -->
        <div v-if="projects.length > 0" class="dropdown-divider"></div>

        <!-- Project list -->
        <div v-if="projects.length === 0" class="dropdown-empty">
          No projects found. Click "View All Projects" to create one.
        </div>
        <button
          v-for="project in projects"
          :key="project.id"
          :class="['dropdown-item', { active: currentProject?.id === project.id }]"
          @click="handleSelectProject(project)"
        >
          <div class="item-header">
            <span class="item-icon">📁</span>
            <span class="item-name">{{ project.name }}</span>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
// Utility functions for project selector can be added here if needed
</script>

<style scoped>
.project-selector {
  position: relative;
}

.selector-title {
  margin: 0;
  padding: 0.5rem 1.5rem;
  font-size: 1.8rem;
  font-weight: 700;
  color: white;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 1rem;
  transition: all 0.2s ease;
  user-select: none;
}

.selector-title:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}

.project-name {
  font-weight: 700;
}

.no-project {
  color: rgba(255, 255, 255, 0.7);
  font-weight: 600;
}

.dropdown-icon {
  font-size: 0.9rem;
  color: rgba(255, 255, 255, 0.8);
  margin-left: 0.5rem;
}

.dropdown-menu {
  position: absolute;
  top: calc(100% + 0.5rem);
  left: 0;
  min-width: 350px;
  max-width: 500px;
  max-height: 500px;
  overflow-y: auto;
  background: #2d2d30;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  z-index: 1000;
}

.dropdown-loading,
.dropdown-empty {
  padding: 1.5rem;
  text-align: center;
  color: #858585;
  font-size: 0.9rem;
}

.dropdown-list {
  padding: 0.5rem;
}

.dropdown-divider {
  height: 1px;
  background: #3e3e42;
  margin: 0.5rem 0.75rem;
}

.dropdown-item {
  width: 100%;
  padding: 0.875rem 1rem;
  background: transparent;
  border: none;
  border-radius: 6px;
  color: #d4d4d4;
  cursor: pointer;
  text-align: left;
  transition: all 0.2s ease;
  margin-bottom: 0.25rem;
}

.dropdown-item:hover {
  background: #3e3e42;
  transform: translateX(2px);
}

.dropdown-item.active {
  background: #007acc;
}

.dropdown-item.view-all {
  background: rgba(102, 126, 234, 0.15);
  border: 1px solid rgba(102, 126, 234, 0.3);
  font-weight: 600;
}

.dropdown-item.view-all:hover {
  background: rgba(102, 126, 234, 0.25);
  border-color: rgba(102, 126, 234, 0.5);
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
