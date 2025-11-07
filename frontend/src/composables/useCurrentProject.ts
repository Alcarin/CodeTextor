/*
  File: composables/useCurrentProject.ts
  Purpose: Manages the current active project state.
  Author: CodeTextor project
  Notes: Provides reactive current project state for the application.
         Uses database persistence (is_selected flag) instead of localStorage.
*/

import { ref, computed } from 'vue';
import type { Project } from '../types';
import { backend } from '../api/backend';

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
   * Loads the currently selected project from the database.
   * Called on app initialization.
   * Automatically selects the oldest project if none is selected.
   */
  const loadCurrentProject = async () => {
    loading.value = true;
    try {
      const project = await backend.getSelectedProject();
      currentProject.value = project;
    } catch (error) {
      console.error('Failed to load current project:', error);
      currentProject.value = null;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Sets a project as the current active project.
   * Persists selection to database.
   * @param project - Project to set as current
   */
  const setCurrentProject = async (project: Project) => {
    loading.value = true;
    try {
      await backend.setSelectedProject(project.id);
      currentProject.value = project;
    } catch (error) {
      console.error('Failed to set current project:', error);
      throw error;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Clears the current project selection.
   */
  const clearCurrentProject = async () => {
    loading.value = true;
    try {
      await backend.clearSelectedProject();
      currentProject.value = null;
    } catch (error) {
      console.error('Failed to clear current project:', error);
    } finally {
      loading.value = false;
    }
  };

  /**
   * Refreshes the current project data from backend.
   */
  const refreshCurrentProject = async () => {
    if (!currentProject.value) return;

    loading.value = true;
    try {
      const updatedProject = await backend.getProject(currentProject.value.id);
      currentProject.value = updatedProject;
    } catch (error) {
      console.error('Failed to refresh current project:', error);
      // If project doesn't exist anymore, clear it
      await clearCurrentProject();
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
