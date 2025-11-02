/*
  File: composables/useCurrentProject.ts
  Purpose: Manages the current active project state.
  Author: CodeTextor project
  Notes: Provides reactive current project state for the application.
*/

import { ref, computed } from 'vue';
import type { Project } from '../types';
import { mockBackend } from '../services/mockBackend';

// Current active project
const currentProject = ref<Project | null>(null);

// Loading state
const loading = ref<boolean>(false);

/**
 * Composable for managing current project state.
 * Provides reactive current project and methods to change it.
 * @returns Current project state and methods
 */
export function useCurrentProject() {
  /**
   * Loads the current project from backend.
   * Called on app initialization.
   */
  const loadCurrentProject = async () => {
    loading.value = true;
    try {
      currentProject.value = await mockBackend.getCurrentProject();
    } catch (error) {
      console.error('Failed to load current project:', error);
      currentProject.value = null;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Sets a project as the current active project.
   * @param project - Project to set as current
   */
  const setCurrentProject = async (project: Project) => {
    loading.value = true;
    try {
      await mockBackend.setCurrentProject(project.id);
      currentProject.value = project;
    } catch (error) {
      console.error('Failed to set current project:', error);
      throw error;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Clears the current project.
   */
  const clearCurrentProject = () => {
    currentProject.value = null;
  };

  /**
   * Refreshes the current project data.
   */
  const refreshCurrentProject = async () => {
    if (!currentProject.value) return;

    loading.value = true;
    try {
      const updatedProject = await mockBackend.getCurrentProject();
      if (updatedProject && updatedProject.id === currentProject.value.id) {
        currentProject.value = updatedProject;
      }
    } catch (error) {
      console.error('Failed to refresh current project:', error);
    } finally {
      loading.value = false;
    }
  };

  // Computed properties
  const hasCurrentProject = computed(() => currentProject.value !== null);
  const currentProjectId = computed(() => currentProject.value?.id ?? null);

  return {
    currentProject,
    loading,
    hasCurrentProject,
    currentProjectId,
    loadCurrentProject,
    setCurrentProject,
    clearCurrentProject,
    refreshCurrentProject
  };
}
