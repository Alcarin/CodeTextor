<!--
  File: views/ProjectsView.vue
  Purpose: Project management interface for creating, selecting, and deleting projects.
  Author: CodeTextor project
  Notes: Dedicated view for multi-project management.
-->

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { useNavigation } from '../composables/useNavigation';
import { backend } from '../api/backend';
import type { Project } from '../types';
import ProjectCard from '../components/ProjectCard.vue';
import ProjectTable from '../components/ProjectTable.vue';
import DeleteConfirmModal from '../components/DeleteConfirmModal.vue';
import ProjectFormModal from '../components/ProjectFormModal.vue';

// Get composables
const { currentProject, setCurrentProject, clearCurrentProject } = useCurrentProject();
const { navigateTo } = useNavigation();

// State
const projects = ref<Project[]>([]);
const isLoading = ref<boolean>(false);
const showProjectForm = ref<boolean>(false);
const showDeleteConfirm = ref<boolean>(false);
const projectToDelete = ref<Project | null>(null);
const projectToEdit = ref<Project | null>(null);
const viewMode = ref<'grid' | 'table'>('grid'); // Default to grid view

/**
 * Loads all projects from backend.
 */
const loadProjects = async () => {
  isLoading.value = true;
  try {
    projects.value = await backend.listProjects();
  } catch (error) {
    console.error('Failed to load projects:', error);
    alert('Failed to load projects: ' + (error instanceof Error ? error.message : 'Unknown error'));
  } finally {
    isLoading.value = false;
  }
};

/**
 * Opens the create project form.
 */
const openCreateForm = () => {
  projectToEdit.value = null;
  showProjectForm.value = true;
};

/**
 * Opens the edit project form.
 * @param project - The project to edit.
 */
const openEditForm = (project: Project) => {
  projectToEdit.value = project;
  showProjectForm.value = true;
};

/**
 * Cancels project form.
 */
const cancelProjectForm = () => {
  showProjectForm.value = false;
  projectToEdit.value = null;
};

/**
 * Handles project save from modal.
 * @param project - The saved project
 */
const handleProjectSave = async (project: Project) => {
  // Update or add to list
  const index = projects.value.findIndex((p: Project) => p.id === project.id);
  if (index !== -1) {
    // Update existing
    projects.value[index] = project;
  } else {
    // Add new
    projects.value.push(project);
    await setCurrentProject(project);
    navigateTo('indexing');
  }

  showProjectForm.value = false;
  projectToEdit.value = null;
};


/**
 * Opens delete confirmation dialog.
 * @param project - Project to delete
 */
const confirmDelete = (project: Project) => {
  projectToDelete.value = project;
  showDeleteConfirm.value = true;
};

/**
 * Cancels project deletion.
 */
const cancelDelete = () => {
  projectToDelete.value = null;
  showDeleteConfirm.value = false;
};

/**
 * Deletes the selected project.
 */
const deleteProject = async () => {
  if (!projectToDelete.value) return;

  try {
    await backend.deleteProject(projectToDelete.value.id);

    // Remove from list
    projects.value = projects.value.filter((p: Project) => p.id !== projectToDelete.value!.id);

    // Clear current project if it was deleted
    if (currentProject.value?.id === projectToDelete.value.id) {
      clearCurrentProject();
    }

    // Close dialog
    showDeleteConfirm.value = false;
    projectToDelete.value = null;
  } catch (error) {
    console.error('Failed to delete project:', error);
    alert('Failed to delete project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

/**
 * Selects a project and navigates to indexing view.
 * @param project - Project to select
 */
const goToIndexing = async (project: Project) => {
  try {
    await setCurrentProject(project);
    navigateTo('indexing');
  } catch (error) {
    console.error('Failed to select project:', error);
    alert('Failed to select project: ' + (error instanceof Error ? error.message : 'Unknown error'));
  }
};

// Load projects on mount
onMounted(() => {
  loadProjects();
});
</script>

<template>
  <div class="projects-view">
    <!-- Actions -->
    <div class="actions-bar">
      <div class="actions-left">
        <button @click="openCreateForm" class="btn btn-primary">
          + Create New Project
        </button>
        <button @click="loadProjects" :disabled="isLoading" class="btn btn-secondary">
          {{ isLoading ? 'Refreshing...' : '↻ Refresh' }}
        </button>
      </div>

      <!-- View mode toggle -->
      <div class="view-toggle">
        <button
          :class="['toggle-btn', { active: viewMode === 'grid' }]"
          @click="viewMode = 'grid'"
          title="Grid view"
        >
          <span class="toggle-icon">▦</span>
        </button>
        <button
          :class="['toggle-btn', { active: viewMode === 'table' }]"
          @click="viewMode = 'table'"
          title="Table view"
        >
          <span class="toggle-icon">☰</span>
        </button>
      </div>
    </div>

    <!-- Projects list -->
    <div v-if="isLoading" class="loading-state section">
      <div class="spinner"></div>
      <p>Loading projects...</p>
    </div>

    <div v-else-if="projects.length === 0" class="empty-state section">
      <div class="empty-icon">📁</div>
      <h3>No Projects Yet</h3>
      <p>Create your first project to get started with code indexing and semantic search.</p>
      <button @click="openCreateForm" class="btn btn-primary" style="margin-top: 1rem">
        Create Your First Project
      </button>
    </div>

    <!-- Grid View -->
    <div v-else-if="viewMode === 'grid'" class="projects-grid">
      <ProjectCard
        v-for="project in projects"
        :key="project.id"
        :project="project"
        :is-active="currentProject?.id === project.id"
        @go-to-indexing="goToIndexing"
        @edit="openEditForm"
        @delete="confirmDelete"
      />
    </div>

    <!-- Table View -->
    <ProjectTable
      v-else
      :projects="projects"
      :current-project-id="currentProject?.id"
      @go-to-indexing="goToIndexing"
      @edit="openEditForm"
      @delete="confirmDelete"
    />

    <!-- Create/Edit Project Modal -->
    <ProjectFormModal
      v-if="showProjectForm"
      :project="projectToEdit"
      @save="handleProjectSave"
      @cancel="cancelProjectForm"
    />

    <!-- Delete Confirmation Modal -->
    <DeleteConfirmModal
      v-if="showDeleteConfirm && projectToDelete"
      :project="projectToDelete"
      @confirm="deleteProject"
      @cancel="cancelDelete"
    />
  </div>
</template>

<style scoped>
.projects-view {
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

/* Actions bar */
.actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.actions-left {
  display: flex;
  gap: 1rem;
}

/* View toggle */
.view-toggle {
  display: flex;
  gap: 0.25rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 0.25rem;
}

.toggle-btn {
  padding: 0.5rem 0.75rem;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: #858585;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.toggle-btn:hover {
  background: #3e3e42;
  color: #d4d4d4;
}

.toggle-btn.active {
  background: #007acc;
  color: white;
}

.toggle-icon {
  font-size: 1.2rem;
  line-height: 1;
}

/* Projects grid */
.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 1.5rem;
}

/* Buttons */
.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
  font-weight: 500;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.btn-primary:hover:not(:disabled) {
  background: #005a9e;
}

.btn-success {
  background: #28a745;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #218838;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background: #5a6268;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background: #c82333;
}

/* States */
.loading-state,
.empty-state {
  text-align: center;
  padding: 3rem;
  color: #858585;
}

.spinner {
  border: 4px solid #3e3e42;
  border-top-color: #007acc;
  border-radius: 50%;
  width: 40px;
  height: 40px;
  animation: spin 1s linear infinite;
  margin: 0 auto 1rem;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
}

.empty-state h3 {
  margin: 0 0 0.5rem 0;
  color: #d4d4d4;
}
</style>
